package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"
)

const (
	// manualTokenCaptchaTimeout 是可见浏览器等待用户完成人工验证的总时长，超时后释放全部浏览器资源。
	manualTokenCaptchaTimeout = 3 * time.Minute
	// manualTokenCaptchaPollInterval 是人工验证期间检查页面地址与新 x5sec 的轮询间隔。
	manualTokenCaptchaPollInterval = 250 * time.Millisecond
	// manualTokenCaptchaNavigationTimeout 是初次打开平台验证地址的导航上限，不限制后续人工操作时间。
	manualTokenCaptchaNavigationTimeout = 10 * time.Second
)

// manualTokenCaptchaState 提供人工验证成功判定所需的最小页面状态，不暴露鼠标或键盘操作能力。
type manualTokenCaptchaState interface {
	// IsClosed 表示用户或浏览器是否已经关闭当前验证页。
	IsClosed() bool
	// URL 返回当前页面地址，用于确认已经离开平台 punish/captcha 页面。
	URL() string
	// Cookies 返回当前隔离上下文的 Cookie；调用方只提取 x5 系列值，不得写入日志。
	Cookies() ([]playwright.Cookie, error)
}

// playwrightManualTokenCaptchaState 把 Playwright 页面和上下文限制为只读成功判定接口。
type playwrightManualTokenCaptchaState struct {
	// page 是用户正在操作的可见验证页，只读取关闭状态和当前地址。
	page playwright.Page
	// context 保存当前人工验证上下文的 Cookie，只在本次调用内以明文存在。
	context playwright.BrowserContext
}

// IsClosed 返回人工验证页是否已被用户或浏览器关闭。
func (s playwrightManualTokenCaptchaState) IsClosed() bool {
	return s.page == nil || s.page.IsClosed()
}

// URL 返回人工验证页当前地址；页面不存在时返回空字符串并由关闭状态优先判定失败。
func (s playwrightManualTokenCaptchaState) URL() string {
	if s.page == nil {
		return ""
	}
	return s.page.URL()
}

// Cookies 读取人工验证隔离上下文的完整 Cookie Jar；返回值不得记录或序列化到前端。
func (s playwrightManualTokenCaptchaState) Cookies() ([]playwright.Cookie, error) {
	if s.context == nil {
		return nil, fmt.Errorf("人工验证浏览器上下文不存在")
	}
	return s.context.Cookies()
}

// TokenCaptchaManualRecover 打开独立可见 Chrome 并等待用户完成人工验证。
// cookieID 只用于日志关联和测试隔离；cookieStr 是本次浏览器上下文使用的明文 Cookie，禁止记录；
// verificationURL 是平台返回的完整验证地址；provider 仅在地址过期且当前浏览器已经关闭后重取地址。
// 成功返回合并新 x5sec 的 Cookie 字符串；超时、取消、关闭窗口或平台地址过期均返回可人工诊断的错误。
func (m *Manager) TokenCaptchaManualRecover(ctx context.Context, cookieID, cookieStr, verificationURL string, provider TokenCaptchaURLProvider) (string, error) {
	// manualRecover 是可测试的人工浏览器执行函数；生产环境使用真实可见 Chrome，且不会调用任何滑块求解器。
	manualRecover := m.tokenCaptchaManualFn
	if manualRecover == nil {
		manualRecover = m.tokenCaptchaManualBrowserRecover
	}
	// currentCookies 是地址刷新后可能更新的明文 Cookie，只在当前调用栈中短暂保留。
	currentCookies := cookieStr
	// currentURL 是本轮实际打开的完整验证地址，错误返回时保留给本机人工诊断但不得写入普通页面日志。
	currentURL := verificationURL
	// refreshCount 统计验证地址过期后的重取次数，沿用自动引擎最多两次的既有上限。
	for refreshCount := 0; ; refreshCount++ {
		// cookies、manualErr 分别是人工完成后的 Cookie 字符串和本轮浏览器错误。
		cookies, manualErr := manualRecover(ctx, cookieID, currentCookies, currentURL, false, nil)
		if manualErr == nil {
			return cookies, nil
		}
		if !errorsIsTokenCaptchaURLExpired(manualErr) || provider == nil || refreshCount >= 2 {
			return "", tokenCaptchaFailure(manualErr, currentURL)
		}
		// freshURL、tokenOK、updatedCookies、providerErr 是关闭过期页面后重新请求平台验证地址的结果。
		freshURL, tokenOK, updatedCookies, providerErr := provider(ctx, currentCookies)
		if providerErr != nil {
			return "", tokenCaptchaFailure(fmt.Errorf("%w且重取失败: %v", errTokenCaptchaURLExpired, providerErr), currentURL)
		}
		if strings.TrimSpace(updatedCookies) != "" {
			currentCookies = updatedCookies
		}
		if tokenOK {
			return currentCookies, nil
		}
		if strings.TrimSpace(freshURL) == "" {
			return "", tokenCaptchaFailure(fmt.Errorf("%w且接口未返回新链接", errTokenCaptchaURLExpired), currentURL)
		}
		currentURL = freshURL
	}
}

// errorsIsTokenCaptchaURLExpired 保持人工编排对包装错误的过期判断，并避免调用方依赖具体错误文本。
func errorsIsTokenCaptchaURLExpired(err error) bool {
	return errors.Is(err, errTokenCaptchaURLExpired)
}

// tokenCaptchaManualBrowserRecover 在独立浏览器上下文中打开验证页并等待人工操作；headless 仅供真实 Chromium 集成测试，生产入口固定传 false。
func (m *Manager) tokenCaptchaManualBrowserRecover(ctx context.Context, cookieID, cookieStr, verificationURL string, headless bool, _ TokenCaptchaURLProvider) (result string, resultErr error) {
	if strings.TrimSpace(cookieStr) == "" {
		return "", fmt.Errorf("Cookie为空，无法处理 token 风控人工验证")
	}
	if strings.TrimSpace(verificationURL) == "" {
		return "", fmt.Errorf("验证链接为空")
	}

	// browserContext、release、contextErr 分别是本轮独立浏览器上下文、幂等释放函数和创建错误。
	browserContext, release, contextErr := m.newManualTokenCaptchaContext(ctx, cookieStr, headless)
	if contextErr != nil {
		return "", contextErr
	}
	defer release()
	// beforeCookies 是导航前的 Cookie 快照，仅用于拒绝把旧 x5sec 重复判为成功。
	beforeCookies, cookieErr := browserContext.Cookies()
	if cookieErr != nil {
		return "", fmt.Errorf("读取 token 风控人工验证前 Cookie 失败: %w", cookieErr)
	}
	// previousX5SecValues 保存已有 x5sec 的值集合，不包含其他 Cookie 内容。
	previousX5SecValues := x5SecValues(beforeCookies)
	// page 是用户将要操作的验证页；有头生产模式不覆盖 Chromium 原生指纹。
	page, pageErr := m.newBrowserPage(browserContext, headless)
	if pageErr != nil {
		return "", fmt.Errorf("新建人工验证页面失败: %w", pageErr)
	}
	defer func() {
		if !page.IsClosed() {
			_ = page.Close()
		}
	}()
	// diagnostic 只在失败时保存脱敏页面状态，不记录 Cookie 或验证地址查询参数。
	diagnostic := newTokenCaptchaDiagnostic(cookieID, "manual", verificationURL, page, m.logger)
	// diagnosticSucceeded 表示人工验证已满足严格成功条件，成功路径不生成失败诊断包。
	diagnosticSucceeded := false
	defer func() {
		if diagnostic != nil && !diagnosticSucceeded {
			diagnostic.capture(page, "manual_failed", resultErr)
		}
	}()

	// navigationErr 是初次访问验证地址的错误；页面仍存在时允许用户观察并操作已加载内容。
	_, navigationErr := page.Goto(verificationURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(float64(manualTokenCaptchaNavigationTimeout.Milliseconds())),
	})
	if navigationErr != nil {
		m.logger.Warn("token风控人工验证页面访问异常", "cookieID", cookieID, "err", navigationErr)
	}
	// content 是初次加载后的页面 HTML，只用于识别平台明确的过期页或浏览器崩溃页。
	content, _ := page.Content()
	if captchaURLExpired(content) {
		return "", errTokenCaptchaURLExpired
	}
	if strings.Contains(content, "STATUS_BREAKPOINT") || strings.Contains(content, "崩溃") {
		return "", fmt.Errorf("token 风控人工验证页面崩溃")
	}
	if diagnostic != nil {
		diagnostic.snapshotInitial(page)
	}
	// title、titleErr 分别是人工验证页标题和读取错误；标题只用于不含凭证的状态日志。
	if title, titleErr := page.Title(); titleErr == nil {
		m.logger.Info("token风控等待人工验证", "cookieID", cookieID, "title", title, "timeout", manualTokenCaptchaTimeout)
	}
	// state 限制等待逻辑只能读取页面状态和 Cookie，不能产生鼠标、键盘或点击事件。
	state := playwrightManualTokenCaptchaState{page: page, context: browserContext}
	// freshCookies、waitErr 是用户完成后产生的新 x5 Cookie 集合和等待阶段错误。
	freshCookies, waitErr := waitForManualTokenCaptcha(ctx, state, previousX5SecValues, manualTokenCaptchaTimeout, manualTokenCaptchaPollInterval)
	if waitErr != nil {
		return "", waitErr
	}
	// merged 以原 Cookie 字符串为基础，只合并人工验证新签发的 x5 系列值。
	merged := parseCookieStr(cookieStr)
	// name、value 是当前待合并的新 x5 Cookie 名称和值；值不得写入日志。
	for name, value := range freshCookies {
		merged[name] = value
	}
	m.logger.Info("token风控人工验证成功", "cookieID", cookieID, "x5_cookie_count", len(freshCookies))
	diagnosticSucceeded = true
	return cookieMarshal(merged), nil
}

// newManualTokenCaptchaContext 创建不使用账号持久化 Profile 的独立浏览器上下文。
// 它不持有账号续期互斥锁或凭证锁等待用户，release 负责关闭上下文、浏览器并结束 Manager 生命周期登记。
func (m *Manager) newManualTokenCaptchaContext(ctx context.Context, cookieStr string, headless bool) (playwright.BrowserContext, func(), error) {
	// beginErr 表示 Manager 已关闭或调用方在创建浏览器前已经取消。
	if beginErr := m.beginOperation(ctx); beginErr != nil {
		return nil, nil, beginErr
	}
	// finishOnce 保证失败清理与调用方 release 只结束一次活动操作登记。
	var finishOnce sync.Once
	// finishOperation 从 Manager 生命周期中移除本次人工浏览器操作。
	finishOperation := func() {
		finishOnce.Do(m.endOperation)
	}
	// initErr 表示 Playwright runtime 初始化或指纹探测失败。
	if initErr := m.initContext(ctx); initErr != nil {
		finishOperation()
		return nil, nil, initErr
	}
	// browser 是本轮人工验证专用的 Chromium 进程，不复用账号持久化 Profile，避免等待用户时占用 Profile 锁。
	browser, browserErr := m.pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless:       playwright.Bool(headless),
		Args:           chromiumLaunchArgs(),
		ExecutablePath: chromiumExecutablePath(),
	})
	if browserErr != nil {
		finishOperation()
		return nil, nil, fmt.Errorf("启动 token 风控人工验证浏览器失败: %w", browserErr)
	}
	// contextOptions 保持项目统一的桌面尺寸、语言和时区；有头模式使用 Chromium 原生 UA。
	contextOptions := playwright.BrowserNewContextOptions{
		Viewport:   &playwright.Size{Width: 1980, Height: 1024},
		Locale:     playwright.String(defaultLang),
		TimezoneId: playwright.String(defaultTZ),
	}
	if headless {
		contextOptions.UserAgent = m.headlessUserAgent()
	}
	// browserContext 是本轮人工验证隔离上下文，关闭后所有明文 Cookie 从浏览器内存释放。
	browserContext, contextErr := browser.NewContext(contextOptions)
	if contextErr != nil {
		_ = browser.Close()
		finishOperation()
		return nil, nil, fmt.Errorf("创建 token 风控人工验证上下文失败: %w", contextErr)
	}
	// scriptErr 表示浏览器上下文无法注入既有 stealth 脚本；记录后继续让用户观察真实页面。
	if scriptErr := browserContext.AddInitScript(playwright.Script{Content: playwright.String(stealthScript())}); scriptErr != nil {
		m.logger.Warn("人工验证上下文注入 stealth 脚本失败", "err", scriptErr)
	}
	// cookieErr 表示明文 Cookie 无法注入本轮隔离上下文；失败时必须同步释放浏览器。
	if cookieErr := addCookieStr(browserContext, cookieStr); cookieErr != nil {
		_ = browserContext.Close()
		_ = browser.Close()
		finishOperation()
		return nil, nil, fmt.Errorf("人工验证上下文注入 Cookie 失败: %w", cookieErr)
	}
	// releaseOnce 保证调用方重复释放时只关闭一次浏览器资源。
	var releaseOnce sync.Once
	// release 按上下文、浏览器、Manager 登记的顺序释放资源，不持有任何 Manager mutex 执行浏览器 I/O。
	release := func() {
		releaseOnce.Do(func() {
			_ = browserContext.Close()
			_ = browser.Close()
			finishOperation()
		})
	}
	return browserContext, release, nil
}

// waitForManualTokenCaptcha 在不产生任何输入事件的前提下等待新 x5sec 和页面离开验证地址。
// ctx 由账号运行时拥有；state 只读浏览器状态；previousValues 只含旧 x5sec；timeout 和 pollInterval 必须大于零。
func waitForManualTokenCaptcha(ctx context.Context, state manualTokenCaptchaState, previousValues map[string]struct{}, timeout, pollInterval time.Duration) (map[string]string, error) {
	if ctx == nil {
		return nil, fmt.Errorf("人工验证等待需要 Context")
	}
	if state == nil {
		return nil, fmt.Errorf("人工验证页面状态不存在")
	}
	if timeout <= 0 || pollInterval <= 0 {
		return nil, fmt.Errorf("人工验证等待超时和轮询间隔必须大于 0")
	}
	// timeoutTimer 限制用户人工操作的总等待时间，函数返回时负责停止。
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	// pollTicker 驱动只读页面状态检查，函数返回时负责停止。
	pollTicker := time.NewTicker(pollInterval)
	defer pollTicker.Stop()
	for {
		if state.IsClosed() {
			return nil, fmt.Errorf("人工验证窗口已关闭")
		}
		// cookies、cookieErr 是当前上下文 Cookie 和读取错误；Cookie 内容不得进入错误或日志。
		cookies, cookieErr := state.Cookies()
		if cookieErr != nil {
			return nil, fmt.Errorf("读取人工验证 Cookie 失败: %w", cookieErr)
		}
		// x5、fresh 分别是当前 x5 系列 Cookie 和是否出现相对旧值全新的 x5sec。
		x5, fresh := freshX5Cookies(cookies, previousValues)
		if fresh && !isPunishURL(state.URL()) {
			return x5, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("等待人工完成 token 风控验证: %w", ctx.Err())
		case <-timeoutTimer.C:
			return nil, fmt.Errorf("等待人工完成 token 风控验证超过 %s", timeout)
		case <-pollTicker.C:
		}
	}
}
