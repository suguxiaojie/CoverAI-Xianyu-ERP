package browser

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// fakeManualTokenCaptchaState 为人工等待测试提供可控的关闭状态、页面地址和 Cookie 序列。
type fakeManualTokenCaptchaState struct {
	// closed 表示验证窗口是否已经关闭。
	closed bool
	// url 是当前页面地址，用于严格验证是否已经离开 punish/captcha。
	url string
	// cookies 是当前上下文 Cookie 快照，测试值不得包含真实凭证。
	cookies []playwright.Cookie
	// cookieErr 是读取 Cookie 时返回的可控错误。
	cookieErr error
}

// IsClosed 返回测试配置的窗口关闭状态。
func (s *fakeManualTokenCaptchaState) IsClosed() bool {
	return s.closed
}

// URL 返回测试配置的当前页面地址。
func (s *fakeManualTokenCaptchaState) URL() string {
	return s.url
}

// Cookies 返回测试 Cookie 快照的副本，避免断言修改原始夹具。
func (s *fakeManualTokenCaptchaState) Cookies() ([]playwright.Cookie, error) {
	return append([]playwright.Cookie(nil), s.cookies...), s.cookieErr
}

// TestWaitForManualTokenCaptchaRequiresFreshCookieAndNavigation 验证人工模式同时要求新 x5sec 和离开验证地址。
func TestWaitForManualTokenCaptchaRequiresFreshCookieAndNavigation(t *testing.T) {
	// previousValues 保存验证前已有的旧 x5sec，防止旧值被误判为用户完成。
	previousValues := map[string]struct{}{"old": {}}
	// state 模拟人工操作完成后出现新 Cookie 且页面已经离开 punish 地址。
	state := &fakeManualTokenCaptchaState{
		url:     "https://www.goofish.com/im",
		cookies: []playwright.Cookie{{Name: "x5sec", Value: "fresh"}},
	}
	// freshCookies、waitErr 分别是严格成功后提取的 x5 Cookie 和等待错误。
	freshCookies, waitErr := waitForManualTokenCaptcha(context.Background(), state, previousValues, time.Second, 10*time.Millisecond)
	if waitErr != nil || freshCookies["x5sec"] != "fresh" {
		t.Fatalf("fresh=%v err=%v", freshCookies, waitErr)
	}

	// punishState 即使已有新 Cookie，仍停留在验证地址时必须超时而不是误判成功。
	punishState := &fakeManualTokenCaptchaState{
		url:     "https://h5api.m.goofish.com/punish?action=captcha",
		cookies: []playwright.Cookie{{Name: "x5sec", Value: "fresh"}},
	}
	// _, punishErr 只关注停留验证页时的失败语义，不保留任何 Cookie 输出。
	_, punishErr := waitForManualTokenCaptcha(context.Background(), punishState, previousValues, 20*time.Millisecond, 5*time.Millisecond)
	if punishErr == nil || !strings.Contains(punishErr.Error(), "超过") {
		t.Fatalf("punish err=%v", punishErr)
	}
}

// TestWaitForManualTokenCaptchaStopsOnCancellationAndClosedWindow 验证账号取消和用户关闭窗口都会立即结束等待。
func TestWaitForManualTokenCaptchaStopsOnCancellationAndClosedWindow(t *testing.T) {
	// canceledCtx 是调用前已经取消的账号运行 Context。
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	// openState 模拟仍停留在验证页且没有新 x5sec 的窗口。
	openState := &fakeManualTokenCaptchaState{url: "https://h5api.m.goofish.com/punish"}
	// _, cancelErr 只验证取消语义，不保留 Cookie 输出。
	_, cancelErr := waitForManualTokenCaptcha(canceledCtx, openState, nil, time.Second, 10*time.Millisecond)
	if !errors.Is(cancelErr, context.Canceled) {
		t.Fatalf("cancel err=%v", cancelErr)
	}

	// closedState 模拟用户主动关闭项目打开的验证窗口。
	closedState := &fakeManualTokenCaptchaState{closed: true}
	// _, closedErr 只验证窗口关闭错误，不保留 Cookie 输出。
	_, closedErr := waitForManualTokenCaptcha(context.Background(), closedState, nil, time.Second, 10*time.Millisecond)
	if closedErr == nil || !strings.Contains(closedErr.Error(), "窗口已关闭") {
		t.Fatalf("closed err=%v", closedErr)
	}
}

// TestTokenCaptchaManualRecoverRefreshesExpiredURLWithoutAutomaticEngines 验证人工编排只重取过期地址，不调用主或备用自动滑块。
func TestTokenCaptchaManualRecoverRefreshesExpiredURLWithoutAutomaticEngines(t *testing.T) {
	// manualCalls 统计人工浏览器调用次数，第一次模拟地址过期，第二次返回用户完成后的新 Cookie。
	manualCalls := 0
	// primaryCalls 和 fallbackCalls 证明人工入口没有触碰任何自动引擎。
	primaryCalls, fallbackCalls := 0, 0
	// manager 注入三类引擎替身，只有人工替身允许被调用。
	manager := &Manager{
		tokenCaptchaManualFn: func(_ context.Context, _ string, cookies, verificationURL string, _ bool, _ TokenCaptchaURLProvider) (string, error) {
			manualCalls++
			if manualCalls == 1 {
				return "", errTokenCaptchaURLExpired
			}
			if verificationURL != "https://fresh.example/punish" || !strings.Contains(cookies, "_m_h5_tk=fresh") {
				t.Fatalf("url=%q cookies=%q", verificationURL, cookies)
			}
			return cookies + "; x5sec=manual-fresh", nil
		},
		tokenCaptchaPrimaryFn: func(context.Context, string, string, string, bool, TokenCaptchaURLProvider) (string, error) {
			primaryCalls++
			return "", errors.New("主引擎不应运行")
		},
		tokenCaptchaFallbackFn: func(context.Context, string, string, string, bool, TokenCaptchaURLProvider) (string, error) {
			fallbackCalls++
			return "", errors.New("备用引擎不应运行")
		},
	}
	// providerCalls 统计人工页面关闭后重取验证地址的次数。
	providerCalls := 0
	// provider 返回新的验证地址与更新后的非真实测试 Cookie。
	provider := func(context.Context, string) (string, bool, string, error) {
		providerCalls++
		return "https://fresh.example/punish", false, "unb=1; _m_h5_tk=fresh", nil
	}
	// cookies、recoverErr 分别是人工编排返回的 Cookie 和错误。
	cookies, recoverErr := manager.TokenCaptchaManualRecover(context.Background(), "cid", "unb=1", "https://expired.example/punish", provider)
	if recoverErr != nil || !strings.Contains(cookies, "x5sec=manual-fresh") {
		t.Fatalf("cookies=%q err=%v", cookies, recoverErr)
	}
	if manualCalls != 2 || providerCalls != 1 || primaryCalls != 0 || fallbackCalls != 0 {
		t.Fatalf("manual=%d provider=%d primary=%d fallback=%d", manualCalls, providerCalls, primaryCalls, fallbackCalls)
	}
}

// TestTokenCaptchaManualBrowserIntegration 在真实 Chromium 中验证人工路径不产生输入事件，并在页面自行模拟用户完成后收集新 x5sec。
func TestTokenCaptchaManualBrowserIntegration(t *testing.T) {
	if os.Getenv("RUN_BROWSER_INTEGRATION") != "1" {
		t.Skip("set RUN_BROWSER_INTEGRATION=1 to verify manual token CAPTCHA behavior")
	}
	// inputEvents 统计浏览器页面收到的鼠标、指针或键盘事件；人工路径测试期间必须保持为零。
	var inputEvents atomic.Int32
	// server 是本地验证页夹具，延迟签发新 x5sec 并离开 captcha 地址，不访问真实闲鱼服务。
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/input" {
			inputEvents.Add(1)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// 页面脚本仅模拟用户已经完成后的平台结果；事件探针用于证明 Go 人工路径没有自动移动鼠标或输入键盘。
		_, _ = io.WriteString(w, `<!doctype html><html><head><title>人工验证测试</title></head><body>
<div id="nc_1_n1t"><span id="nc_1_n1z">等待人工操作</span></div>
<script>
for (const eventName of ['pointerdown', 'mousedown', 'mousemove', 'mouseup', 'keydown']) {
  document.addEventListener(eventName, () => fetch('/input', {method: 'POST'}), {once: true});
}
setTimeout(() => {
  document.cookie = 'x5sec=manual-integration-fresh; path=/';
  history.replaceState({}, '', '/done');
  document.title = '人工验证完成';
}, 300);
</script></body></html>`)
	}))
	defer server.Close()

	// manager 拥有本次测试 Playwright 生命周期，defer Close 同步释放可能残留的浏览器资源。
	manager := NewManager(nil)
	defer manager.Close()
	// testCtx 限制本地真实浏览器集成测试总耗时，避免失败时等待生产三分钟上限。
	testCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// cookies、recoverErr 是无头测试入口模拟可见人工路径后的 Cookie 与错误；生产入口固定使用有头模式。
	cookies, recoverErr := manager.tokenCaptchaManualBrowserRecover(testCtx, "manual-integration", "unb=1; x5sec=old", server.URL+"/captcha", true, nil)
	if recoverErr != nil || !strings.Contains(cookies, "x5sec=manual-integration-fresh") {
		t.Fatalf("cookies=%q err=%v", cookies, recoverErr)
	}
	if inputEvents.Load() != 0 {
		t.Fatalf("人工路径不应产生任何自动输入事件，got=%d", inputEvents.Load())
	}
}
