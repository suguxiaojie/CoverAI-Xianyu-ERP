package adapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"xianyu-go/internal/browser"
	"xianyu-go/internal/db"
	"xianyu-go/internal/engine"
	"xianyu-go/internal/logsafe"
	"xianyu-go/internal/xianyu/cookierefresh"
	"xianyu-go/internal/xianyu/mtop"
)

// tokenCaptchaExecution 保存单次 token 风控引擎选择与结果，不持有 Cookie 之外的账号敏感字段。
type tokenCaptchaExecution struct {
	// cookies 是验证完成后待持久化的明文 Cookie，只允许存在于当前调用栈。
	cookies string
	// engine 是风险日志使用的引擎标识：manual、remote、playwright 或 drissionpage。
	engine string
	// manual 表示本轮完全由用户操作，必须跳过远程、主、备用自动引擎和旧 Profile 快照读取。
	manual bool
	// remote 表示远程服务已经处理本轮验证，不能再执行本机浏览器路径。
	remote bool
	// headless 是自动本机路径和 Profile 快照读取使用的浏览器可见性结果。
	headless bool
	// err 是引擎执行错误，调用方负责脱敏日志、风险记录和用户通知。
	err error
}

// OnTokenCaptchaVerification 处理 token 刷新触发的闲鱼风控，按账号设置选择人工或自动路径并持久化结果。
func (a *Adapter) OnTokenCaptchaVerification(ctx context.Context, cookieID, cookieStr, verificationURL, deviceID string) (*mtop.RefreshResult, bool) {
	// start 是本轮验证开始时间，用于风险日志记录总耗时。
	start := time.Now()
	// logID 是 processing 风险日志主键；写入失败时保留零值并继续处理验证。
	logID := a.startTokenCaptchaRiskLog(ctx, cookieID, verificationURL)
	if a.store == nil || a.store.Cookies == nil {
		a.OnAccountEvent(ctx, cookieID, engine.EventSecurityVerification, engine.AlertLevelWarn,
			"token 风控验证无法保存", "账号存储未初始化，无法保存验证后的 Cookie。")
		return nil, false
	}
	// showBrowser 和 metadataJSON 是平台运行所需的非密码设置；读取失败时沿用自动模式和空 metadata。
	showBrowser, metadataJSON := a.tokenCaptchaRuntimeSettings(ctx, cookieID)
	// provider 只在浏览器已经关闭后重取验证地址，返回值中的 Cookie 不得写入日志。
	provider := a.tokenCaptchaURLProvider(deviceID)
	// execution 是人工、远程或本机自动路径的统一结果；available=false 表示缺少浏览器能力且已告警。
	execution, available := a.executeTokenCaptcha(ctx, cookieID, cookieStr, verificationURL, deviceID, showBrowser, provider)
	if !available {
		return nil, false
	}
	if execution.err != nil {
		a.recordTokenCaptchaFailure(ctx, cookieID, verificationURL, logID, start, execution)
		return nil, false
	}
	if strings.TrimSpace(execution.cookies) == "" {
		return nil, false
	}
	// savedCookies、cookieSnapshot、snapshotComplete 是最终持久化的扁平 Cookie、精确快照和完整性标记。
	savedCookies, cookieSnapshot, snapshotComplete := a.tokenCaptchaSnapshot(ctx, cookieID, metadataJSON, execution)
	if !a.persistTokenCaptchaSuccess(ctx, cookieID, metadataJSON, savedCookies, cookieSnapshot, snapshotComplete, logID, start, execution.engine) {
		return nil, false
	}
	a.notifyTokenCaptchaSuccess(ctx, cookieID, execution.manual)
	return &mtop.RefreshResult{
		UpdatedCookies:         savedCookies,
		CookieSnapshot:         cookieSnapshot,
		CookieSnapshotComplete: snapshotComplete,
		CookieStateChanged:     savedCookies != cookieStr || snapshotComplete,
	}, true
}

// startTokenCaptchaRiskLog 创建 processing 风险记录；失败只写脱敏日志，不阻断风控恢复。
func (a *Adapter) startTokenCaptchaRiskLog(ctx context.Context, cookieID, verificationURL string) int64 {
	if a.store == nil || a.store.RiskLogs == nil {
		return 0
	}
	// logID、logErr 是新风险记录主键和持久化错误。
	logID, logErr := a.store.RiskLogs.Add(ctx, db.RiskControlLog{
		CookieID:         cookieID,
		EventType:        "slider_captcha",
		EventDescription: "触发场景: Token刷新, URL: " + verificationURL,
		ProcessingStatus: "processing",
	})
	if logErr != nil {
		a.logger.Warn("记录风控日志失败", "account", cookieID, "err", logErr)
		return 0
	}
	return logID
}

// tokenCaptchaRuntimeSettings 读取人工模式和 Cookie 快照 metadata，不读取或解密账号登录密码。
func (a *Adapter) tokenCaptchaRuntimeSettings(ctx context.Context, cookieID string) (bool, string) {
	// runtimeData、runtimeErr 是平台运行所需的最小账号数据和读取错误。
	runtimeData, runtimeErr := a.store.Cookies.GetCookiePlatformRuntimeData(ctx, cookieID)
	if runtimeErr != nil {
		return false, ""
	}
	return runtimeData.ShowBrowser, runtimeData.MetadataJSON
}

// tokenCaptchaURLProvider 返回地址刷新闭包；deviceID 只传给平台请求，不进入日志或持久化。
func (a *Adapter) tokenCaptchaURLProvider(deviceID string) browser.TokenCaptchaURLProvider {
	return func(runCtx context.Context, currentCookies string) (string, bool, string, error) {
		if a.captchaReq == nil {
			return "", false, "", nil
		}
		// result、requestErr 是平台新验证地址响应和请求错误；Cookie 只在返回值中传递。
		result, requestErr := a.captchaReq.RequestFreshCaptchaURLContext(runCtx, currentCookies, deviceID)
		if requestErr != nil || result == nil {
			return "", false, "", requestErr
		}
		return result.VerificationURL, result.TokenOK, result.UpdatedCookies, nil
	}
}

// executeTokenCaptcha 按 showBrowser 选择人工接管或既有自动引擎，并返回统一执行结果。
func (a *Adapter) executeTokenCaptcha(ctx context.Context, cookieID, cookieStr, verificationURL, deviceID string, showBrowser bool, provider browser.TokenCaptchaURLProvider) (tokenCaptchaExecution, bool) {
	// execution 初始化为本机自动 Playwright 路径；人工或远程命中后会覆盖对应字段。
	execution := tokenCaptchaExecution{engine: "playwright", manual: showBrowser, headless: browser.ResolveHeadless(showBrowser)}
	if showBrowser {
		execution.engine = "manual"
		// manualBrowser、ok 是人工接管能力及其可用状态；缺少能力时禁止静默回退自动拖动。
		manualBrowser, ok := a.browser.(browserTokenCaptchaManualRecoverer)
		if !ok {
			execution.err = fmt.Errorf("当前浏览器运行时不支持 token 风控人工验证")
			return execution, true
		}
		execution.cookies, execution.err = manualBrowser.TokenCaptchaManualRecover(ctx, cookieID, cookieStr, verificationURL, provider)
		return execution, true
	}

	// remoteConfig 是当前账号可用的远程滑块设置；为空时直接进入本机自动路径。
	remoteConfig := a.loadRemoteCaptchaConfig(ctx, cookieID)
	if remoteConfig != nil {
		execution.cookies, execution.remote, execution.err = solveRemoteCaptcha(
			ctx, newRemoteCaptchaHTTPClient(), *remoteConfig,
			cookieID, verificationURL, cookieStr, deviceID, provider,
		)
		if execution.remote {
			execution.engine = "remote"
			return execution, true
		}
		if execution.err != nil {
			a.logger.Warn("远程过滑块不可用，回退本机逻辑", "account", cookieID, "err", execution.err)
			execution.err = nil
		}
	}
	// automaticBrowser、ok 是既有本机自动滑块能力及其可用状态。
	automaticBrowser, ok := a.browser.(browserTokenCaptchaRecoverer)
	if a.browser == nil || !ok {
		a.OnAccountEvent(ctx, cookieID, engine.EventSecurityVerification, engine.AlertLevelWarn,
			"token 风控验证无法自动处理", "远程服务不可用且浏览器自动化未启用，无法自动完成 token 滑块验证。")
		return execution, false
	}
	// engineBrowser、hasEngineResult 表示浏览器是否返回精确的主／备用引擎标识。
	if engineBrowser, hasEngineResult := a.browser.(browserTokenCaptchaEngineRecoverer); hasEngineResult {
		execution.cookies, execution.engine, execution.err = engineBrowser.TokenCaptchaRecoverWithEngine(
			ctx, cookieID, cookieStr, verificationURL, execution.headless, provider,
		)
		return execution, true
	}
	execution.cookies, execution.err = automaticBrowser.TokenCaptchaRecover(
		ctx, cookieID, cookieStr, verificationURL, execution.headless, provider,
	)
	return execution, true
}

// recordTokenCaptchaFailure 保存失败状态和人工诊断地址，并发送一次安全验证告警。
func (a *Adapter) recordTokenCaptchaFailure(ctx context.Context, cookieID, verificationURL string, logID int64, start time.Time, execution tokenCaptchaExecution) {
	// manualURL 是错误携带的最后实际验证地址；缺失时回退最初地址，日志层负责敏感查询参数处理。
	manualURL := browser.TokenCaptchaManualVerificationURL(execution.err)
	if strings.TrimSpace(manualURL) == "" {
		manualURL = verificationURL
	}
	a.logger.Warn("token 风控滑块处理失败", "account", cookieID, "err", logsafe.Error(execution.err), "verification_url", logsafe.URL(manualURL))
	if a.store != nil && a.store.RiskLogs != nil {
		_ = a.store.RiskLogs.Update(ctx, logID, db.RiskControlLog{
			ProcessingStatus: "failed",
			ProcessingResult: fmt.Sprintf("token 风控滑块处理失败，耗时: %.2f秒", time.Since(start).Seconds()),
			CaptchaEngine:    execution.engine,
			ErrorMessage:     execution.err.Error(),
			DurationMS:       time.Since(start).Milliseconds(),
		})
	}
	a.OnAccountEvent(ctx, cookieID, engine.EventSecurityVerification, engine.AlertLevelWarn,
		"token 风控验证失败", execution.err.Error())
}

// tokenCaptchaSnapshot 以引擎返回 Cookie 为持久化基线，并只从自动 Profile 补充 x5 系列精确属性。
func (a *Adapter) tokenCaptchaSnapshot(ctx context.Context, cookieID, metadataJSON string, execution tokenCaptchaExecution) (string, []cookierefresh.BrowserCookie, bool) {
	// savedCookies 是最终待持久化的扁平 Cookie；始终保留引擎已合并的新 x5 与原账号身份字段。
	savedCookies := execution.cookies
	// cookieSnapshot 是待写入 metadata 的协调 Cookie Jar；已有快照优先保留非 x5 字段的精确作用域。
	var cookieSnapshot []cookierefresh.BrowserCookie
	// snapshotComplete 表示 cookieSnapshot 可作为完整凭证快照写回 metadata。
	snapshotComplete := false
	// existing、complete 是 metadata 中已有快照和完整性标记；扁平结果负责同步当前值与删除项。
	if existing, complete := cookierefresh.SnapshotFromMetadataOK(metadataJSON); complete {
		cookieSnapshot = cookierefresh.ReconcileSnapshotWithCookieString(existing, savedCookies)
		snapshotComplete = true
	}
	if !execution.manual && !execution.remote {
		// reader、ok 是自动引擎 Profile 快照读取能力及其可用状态。
		if reader, ok := a.browser.(browserTokenCaptchaSnapshotReader); ok {
			// profileSnapshot、readErr 是自动 Profile 的精确 Cookie Jar 和读取错误；扁平 Profile 结果不得覆盖账号凭证。
			_, profileSnapshot, readErr := reader.TokenCaptchaCookieSnapshot(ctx, cookieID, execution.headless)
			if readErr != nil {
				a.logger.Warn("读取滑块验证后完整 Cookie Jar 失败，回退 Go 快照合并", "account", cookieID, "err", readErr)
			} else {
				if !snapshotComplete {
					cookieSnapshot = cookierefresh.SnapshotFromCookieString(savedCookies, ".goofish.com")
					snapshotComplete = true
				}
				cookieSnapshot = mergeTokenCaptchaSnapshot(cookieSnapshot, profileSnapshot)
			}
		}
	}
	return savedCookies, cookieSnapshot, snapshotComplete
}

// mergeTokenCaptchaSnapshot 用 Profile 中的精确 x5 系列 Cookie 替换基线快照中的 x5 项。
// base 是当前账号凭证基线，profile 是滑块浏览器退出后的快照；返回值不得删除或覆盖任何非 x5 字段。
func mergeTokenCaptchaSnapshot(base, profile []cookierefresh.BrowserCookie) []cookierefresh.BrowserCookie {
	// profileX5 保存 Profile 中全部 x5 系列 Cookie 及其 Domain、Path、过期时间和安全属性。
	profileX5 := make([]cookierefresh.BrowserCookie, 0)
	// cookie 是当前检查的 Profile Cookie；非 x5 项不得参与覆盖。
	for _, cookie := range cookierefresh.NormalizeSnapshot(profile) {
		if isTokenCaptchaCookieName(cookie.Name) {
			profileX5 = append(profileX5, cookie)
		}
	}
	if len(profileX5) == 0 {
		return cookierefresh.NormalizeSnapshot(base)
	}
	// merged 保留基线中的账号身份与长期登录字段，并为精确 x5 项预留容量。
	merged := make([]cookierefresh.BrowserCookie, 0, len(base)+len(profileX5))
	// cookie 是当前检查的基线 Cookie；旧 x5 项由 Profile 的精确版本整体替换。
	for _, cookie := range cookierefresh.NormalizeSnapshot(base) {
		if !isTokenCaptchaCookieName(cookie.Name) {
			merged = append(merged, cookie)
		}
	}
	return cookierefresh.NormalizeSnapshot(append(merged, profileX5...))
}

// isTokenCaptchaCookieName 判断 name 是否属于滑块签发的 x5 系列 Cookie；返回值仅用于限定 Profile 覆盖范围。
func isTokenCaptchaCookieName(name string) bool {
	// normalizedName 是去除空白并统一大小写后的 Cookie 名称。
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(normalizedName, "x5") || strings.Contains(normalizedName, "x5sec")
}

// persistTokenCaptchaSuccess 原子更新 Cookie metadata、清除旧 Token 并完成风险日志。
func (a *Adapter) persistTokenCaptchaSuccess(ctx context.Context, cookieID, metadataJSON, cookies string, snapshot []cookierefresh.BrowserCookie, snapshotComplete bool, logID int64, start time.Time, captchaEngine string) bool {
	// updatedMetadata 是移除旧快照后按完整性选择写回的新 metadata。
	updatedMetadata := cookierefresh.MetadataWithoutSnapshot(metadataJSON)
	if snapshotComplete {
		updatedMetadata = cookierefresh.MetadataWithSnapshot(metadataJSON, snapshot)
	}
	// updateErr 表示验证完成后保存凭证失败；失败时不得清除旧 Token 或宣称恢复成功。
	updateErr := a.store.Cookies.UpdateRenewalCookie(ctx, cookieID, cookies, updatedMetadata, time.Now().Unix())
	if updateErr != nil {
		a.logger.Warn("保存 token 风控恢复 Cookie 失败", "account", cookieID, "err", updateErr)
		if a.store != nil && a.store.RiskLogs != nil {
			_ = a.store.RiskLogs.Update(ctx, logID, db.RiskControlLog{
				ProcessingStatus: "error",
				ProcessingResult: "滑块完成但保存 Cookie 失败",
				CaptchaEngine:    captchaEngine,
				ErrorMessage:     updateErr.Error(),
				DurationMS:       time.Since(start).Milliseconds(),
			})
		}
		return false
	}
	if a.store.Tokens != nil {
		_ = a.store.Tokens.Clear(ctx, cookieID)
	}
	if a.store != nil && a.store.RiskLogs != nil {
		_ = a.store.RiskLogs.Update(ctx, logID, db.RiskControlLog{
			ProcessingStatus: "success",
			ProcessingResult: fmt.Sprintf("token 风控滑块验证成功（%s），已更新登录凭证，耗时: %.2f秒", captchaEngine, time.Since(start).Seconds()),
			CaptchaEngine:    captchaEngine,
			DurationMS:       time.Since(start).Milliseconds(),
		})
	}
	return true
}

// notifyTokenCaptchaSuccess 根据人工或自动模式发送准确的凭证恢复通知。
func (a *Adapter) notifyTokenCaptchaSuccess(ctx context.Context, cookieID string, manual bool) {
	if manual {
		a.OnAccountEvent(ctx, cookieID, engine.EventSecurityVerification, engine.AlertLevelInfo,
			"token 风控人工验证已完成", "已检测到人工验证结果并更新登录凭证。")
		return
	}
	a.OnAccountEvent(ctx, cookieID, engine.EventSecurityVerification, engine.AlertLevelInfo,
		"token 风控验证已自动恢复", "系统已完成验证并更新登录凭证。")
}
