package browser

import (
	"net/url"
	"testing"
)

// TestRedFlowerReceiveURLRestrictsOfficialTarget 验证自动收花地址只能由固定官方路径和纯数字订单号构造。
func TestRedFlowerReceiveURLRestrictsOfficialTarget(t *testing.T) {
	// target、buildErr 是合法订单号构造出的官方地址和错误。
	target, buildErr := RedFlowerReceiveURL("5127372398162002704")
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	// parsed、parseErr 是构造结果的结构化 URL 和解析错误。
	parsed, parseErr := url.Parse(target)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if parsed.Scheme != "https" || parsed.Host != "h5.m.goofish.com" || parsed.Path != "/wow/moyu/moyu-project/temp-pages/pages/red-flower-play" {
		t.Fatalf("unexpected target: %s", target)
	}
	if parsed.Query().Get("role") != "seller" || parsed.Query().Get("orderId") != "5127372398162002704" {
		t.Fatalf("unexpected query: %s", parsed.RawQuery)
	}
	// invalid 是会尝试改写目标地址的非法订单值。
	for _, invalid := range []string{"", "order-1", "https://evil.example/steal"} {
		// _, invalidErr 是非法订单构造结果和预期错误。
		_, invalidErr := RedFlowerReceiveURL(invalid)
		if invalidErr == nil {
			t.Fatalf("invalid order should fail: %q", invalid)
		}
	}
}
