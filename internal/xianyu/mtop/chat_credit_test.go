package mtop

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFetchUserCreditParsesStructuredBuyerAndSellerLevels 验证个人主页信用接口按角色解析数字等级和文案。
func TestFetchUserCreditParsesStructuredBuyerAndSellerLevels(t *testing.T) {
	// server 返回包含信用、普通标签和更新 Cookie 的确定性官方响应夹具。
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("api") != "mtop.idle.web.user.page.head" || request.URL.Query().Get("v") != "1.0" {
			t.Fatalf("api=%q v=%q", request.URL.Query().Get("api"), request.URL.Query().Get("v"))
		}
		// body 是本次 POST 表单，必须携带目标 userId 且不能把它放进日志。
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil || !strings.Contains(string(body), "2272060441") {
			t.Fatalf("body=%q err=%v", string(body), readErr)
		}
		writer.Header().Add("Set-Cookie", "_m_h5_tk=fresh_2; Path=/")
		_, _ = writer.Write([]byte(`{"ret":["SUCCESS::调用成功"],"data":{"module":{"base":{"ylzTags":[{"attributes":{"role":"seller","level":5},"code":"cs_seller_level","text":"卖家信用极好","type":"ylzLevel"},{"attributes":{"role":"buyer","level":4},"code":"cs_buyer_level","text":"买家信用优秀","type":"ylzLevel"},{"code":"other","type":"identity"}]}}}}`))
	}))
	defer server.Close()
	// client 使用本地端点和固定签名 Cookie，测试不会访问真实平台。
	client := &ClientImpl{HTTPClient: server.Client(), UserCreditURL: server.URL}
	// result、fetchErr 是结构化信用结果和请求错误。
	result, fetchErr := client.FetchUserCredit(context.Background(), "unb=1; _m_h5_tk=token_1;", "2272060441")
	if fetchErr != nil || result == nil || len(result.Levels) != 2 {
		t.Fatalf("result=%+v err=%v", result, fetchErr)
	}
	if result.Levels[0].Role != "seller" || result.Levels[0].Level != 5 || result.Levels[0].Text != "卖家信用极好" {
		t.Fatalf("seller=%+v", result.Levels[0])
	}
	if result.Levels[1].Role != "buyer" || result.Levels[1].Level != 4 || result.Levels[1].Code != "cs_buyer_level" {
		t.Fatalf("buyer=%+v", result.Levels[1])
	}
	if !strings.Contains(result.UpdatedCookies, "fresh_2") {
		t.Fatalf("updated cookies missing refreshed token")
	}
}

// TestFetchUserCreditRejectsMalformedProfileResponse 验证缺少公开基础资料时不会伪造未知信用等级。
func TestFetchUserCreditRejectsMalformedProfileResponse(t *testing.T) {
	// server 返回成功码但缺少 module.base 的不完整响应。
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ret":["SUCCESS::调用成功"],"data":{}}`))
	}))
	defer server.Close()
	// client 使用本地不完整响应端点。
	client := &ClientImpl{HTTPClient: server.Client(), UserCreditURL: server.URL}
	// result、fetchErr 是应失败的不完整信用解析结果。
	result, fetchErr := client.FetchUserCredit(context.Background(), "_m_h5_tk=token_1;", "buyer-1")
	if fetchErr == nil || result != nil || !strings.Contains(fetchErr.Error(), "module.base") {
		t.Fatalf("result=%+v err=%v", result, fetchErr)
	}
}
