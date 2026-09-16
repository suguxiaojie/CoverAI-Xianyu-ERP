package browser

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
)

const (
	// redFlowerReceiveBaseURL 是唯一允许自动打开的闲鱼官方收花页面。
	redFlowerReceiveBaseURL = "https://h5.m.goofish.com/wow/moyu/moyu-project/temp-pages/pages/red-flower-play"
	// redFlowerReceiveNavigationTimeout 限制官方页面初次导航等待时间。
	redFlowerReceiveNavigationTimeout = 8 * time.Second
	// redFlowerReceivePagePollInterval 是观察用户或页面关闭状态的轮询间隔。
	redFlowerReceivePagePollInterval = 200 * time.Millisecond
)

// RedFlowerReceiveURL 使用固定官方地址和纯数字订单号构造卖家收花页面。
func RedFlowerReceiveURL(orderID string) (string, error) {
	// normalizedOrderID 是去除首尾空白后的平台订单标识。
	normalizedOrderID := strings.TrimSpace(orderID)
	if !isNumericRedFlowerOrderID(normalizedOrderID) {
		return "", errors.New("自动收花订单 ID 必须是纯数字")
	}
	// target 是仅由代码内固定域名和路径构造的官方地址，不接受事件载荷覆盖。
	target, parseErr := url.Parse(redFlowerReceiveBaseURL)
	if parseErr != nil {
		return "", fmt.Errorf("解析官方收花地址: %w", parseErr)
	}
	// query 保存官方页面要求的卖家角色和订单号。
	query := target.Query()
	query.Set("role", "seller")
	query.Set("orderId", normalizedOrderID)
	target.RawQuery = query.Encode()
	return target.String(), nil
}

// isNumericRedFlowerOrderID 判断订单标识是否只包含 ASCII 数字。
func isNumericRedFlowerOrderID(orderID string) bool {
	if len(orderID) < 10 || len(orderID) > 30 {
		return false
	}
	// character 是当前待校验的订单号字符。
	for _, character := range orderID {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

// OpenRedFlowerReceive 在不复用账号持久化 Profile 的独立 Chromium 中打开官方收花页。
// 返回的 opened 只表示页面导航已经开始，不代表平台已经确认收花；调用方必须等待 WS 结果。
func (m *Manager) OpenRedFlowerReceive(ctx context.Context, accountID, cookieStr, orderID string, showBrowser bool) (opened bool, resultErr error) {
	// targetURL、urlErr 是受固定域名、路径和订单号约束的官方页面及构造错误。
	targetURL, urlErr := RedFlowerReceiveURL(orderID)
	if urlErr != nil {
		return false, urlErr
	}
	return m.openRedFlowerReceiveURL(ctx, accountID, cookieStr, targetURL, showBrowser)
}

// openRedFlowerReceiveURL 打开已由调用方约束的页面地址；独立方法允许本地页面做真实浏览器验证。
func (m *Manager) openRedFlowerReceiveURL(ctx context.Context, accountID, cookieStr, targetURL string, showBrowser bool) (opened bool, resultErr error) {
	if m == nil {
		return false, errors.New("自动收花浏览器未初始化")
	}
	if strings.TrimSpace(cookieStr) == "" {
		return false, errors.New("自动收花 Cookie 为空")
	}
	// browserContext、release、contextErr 分别是隔离浏览器上下文、释放函数和创建错误。
	browserContext, release, contextErr := m.newManualTokenCaptchaContext(ctx, cookieStr, !showBrowser)
	if contextErr != nil {
		return false, contextErr
	}
	defer release()
	// page 是本次自动收花专用页面，不产生鼠标或键盘事件。
	page, pageErr := m.newBrowserPage(browserContext, !showBrowser)
	if pageErr != nil {
		return false, fmt.Errorf("创建自动收花页面: %w", pageErr)
	}
	defer func() {
		if !page.IsClosed() {
			_ = page.Close()
		}
	}()
	opened = true
	// _, navigationErr 是官方页面导航结果；一旦导航开始，错误也可能对应远端已收到请求，因此保留 opened=true。
	_, navigationErr := page.Goto(targetURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(float64(redFlowerReceiveNavigationTimeout.Milliseconds())),
	})
	if navigationErr != nil {
		return true, fmt.Errorf("访问自动收花官方页面: %w", navigationErr)
	}
	if m.logger != nil {
		m.logger.Info("自动收花官方页面已打开，等待平台结果", "account", accountID, "show_browser", showBrowser)
	}
	// ticker 定期观察页面是否被用户或站点关闭，不读取页面敏感内容。
	ticker := time.NewTicker(redFlowerReceivePagePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		case <-ticker.C:
			if page.IsClosed() {
				return true, nil
			}
		}
	}
}
