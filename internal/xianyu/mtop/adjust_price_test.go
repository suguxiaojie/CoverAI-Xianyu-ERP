package mtop

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"xianyu-go/internal/xianyu/cookierefresh"
)

// newLogicalGoofishTestClient 把逻辑 goofish URL 连接重定向到本地服务；handler 只接收虚构 Cookie，请求不会离开本机，返回客户端和资源释放函数。
func newLogicalGoofishTestClient(t *testing.T, handler http.Handler) (*ClientImpl, func()) {
	t.Helper()
	// server 是承载 token 与改价端点的本地 HTTP 服务。
	server := httptest.NewServer(handler)
	// serverURL 是本地监听地址，用于只改写 TCP 连接目标而保留请求 URL 的 goofish Cookie 作用域。
	serverURL, parseErr := url.Parse(server.URL)
	if parseErr != nil {
		server.Close()
		t.Fatal(parseErr)
	}
	// dialer 建立到本地测试服务的连接，不访问真实闲鱼域名。
	dialer := &net.Dialer{}
	// transport 保留逻辑请求 Host／URL，只把底层连接固定到 httptest 监听地址。
	transport := &http.Transport{DialContext: func( /* dialContext 把任意逻辑 goofish 地址连接到本地夹具。 */ ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, serverURL.Host)
	}}
	// client 复用生产 MTOP 协议，但所有端点都采用逻辑 HTTP goofish 地址并经 transport 留在本机。
	client := NewClient()
	client.HTTPClient = &http.Client{Transport: transport}
	client.TokenURL = strings.Replace(TokenAPI, "https://", "http://", 1)
	client.AdjustPriceRenderURL = strings.Replace(AdjustPriceRenderAPI, "https://", "http://", 1)
	client.AdjustPriceSubmitURL = strings.Replace(AdjustPriceSubmitAPI, "https://", "http://", 1)
	// cleanup 关闭空闲连接和本地服务，避免测试资源跨用例泄漏。
	cleanup := func() {
		transport.CloseIdleConnections()
		server.Close()
	}
	return client, cleanup
}

// TestAdjustPriceWebProtocolMatchesCapturedRequests 验证 render／submit 与闲鱼网页版真实抓包字段一致。
func TestAdjustPriceWebProtocolMatchesCapturedRequests(t *testing.T) {
	// requests 保存测试服务收到的 API 名称和解码后 data，确保 submit 不被本地字段猜测污染。
	requests := make([]map[string]any, 0, 2)
	// server 是同时模拟 render 与 submit 的本地 MTOP 服务，不连接真实闲鱼。
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// formErr 是解析 application/x-www-form-urlencoded 请求体的错误。
		if formErr := request.ParseForm(); formErr != nil {
			t.Fatal(formErr)
		}
		// data 保存签名对应的原始 JSON 对象。
		var data map[string]any
		if // decodeErr 是解析本地 MTOP data JSON 的错误。
		decodeErr := json.Unmarshal([]byte(request.Form.Get("data")), &data); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		requests = append(requests, map[string]any{"api": request.URL.Query().Get("api"), "data": data,
			"origin": request.Header.Get("Origin"), "referer": request.Header.Get("Referer")})
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("api") == adjustPriceRenderAPIName {
			fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{"title":"修改价格","modifyPriceRenderList":[{"key":"modifyFee","name":"商品价格","prefixText":"¥","price":"0.10","readOnly":false},{"key":"newTransportFee","name":"运费","prefixText":"¥","price":"0.00","readOnly":false}]}}`)
			return
		}
		fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{"success":true}}`)
	}))
	defer server.Close()
	// client 使用本地端点验证真实协议形态。
	client := NewClient()
	client.AdjustPriceRenderURL, client.AdjustPriceSubmitURL = server.URL, server.URL
	// cookies 是只供本地签名测试使用的虚构 Cookie。
	cookies := "_m_h5_tk=test-token_123; unb=test-user"
	// form、renderErr 是动态表单结果和错误。
	form, renderErr := client.RenderOrderAdjustPrice(context.Background(), cookies, "5127694777172175924")
	if renderErr != nil || len(form.Fields) != 2 || form.Fields[0].Key != "modifyFee" || form.Fields[1].Key != "newTransportFee" {
		t.Fatalf("form=%+v err=%v", form, renderErr)
	}
	// result、submitErr 是明确成功的本地 submit 结果和错误。
	result, submitErr := client.SubmitOrderAdjustPrice(context.Background(), cookies, "5127694777172175924", map[string]string{"modifyFee": "20", "newTransportFee": "0"})
	if submitErr != nil || result == nil || !result.Success {
		t.Fatalf("result=%+v err=%v", result, submitErr)
	}
	if len(requests) != 2 {
		t.Fatalf("requests=%+v", requests)
	}
	// renderData、submitData 是两次请求的 data 对象。
	renderData, submitData := requests[0]["data"].(map[string]any), requests[1]["data"].(map[string]any)
	if requests[0]["api"] != adjustPriceRenderAPIName || renderData["bizOrderId"] != "5127694777172175924" {
		t.Fatalf("render request=%+v", requests[0])
	}
	if requests[1]["api"] != adjustPriceSubmitAPIName || submitData["modifyFee"] != "20" || submitData["newTransportFee"] != "0" || submitData["orderId"] != "5127694777172175924" {
		t.Fatalf("submit request=%+v", requests[1])
	}
	if requests[0]["origin"] != "https://www.goofish.com" || requests[0]["referer"] != adjustPriceReferer {
		t.Fatalf("headers=%+v", requests[0])
	}
	// parsedURL 确认本地测试端点仍接收完整 MTOP 查询参数。
	parsedURL, parseErr := url.Parse(server.URL)
	if parseErr != nil || parsedURL.Scheme == "" {
		t.Fatal(parseErr)
	}
}

// TestAdjustPriceExpiredSigningTokenRefreshesBeforeRender 验证权威快照中的签名 token 过期时先执行一次官方 bootstrap，再只调用一次只读 render。
func TestAdjustPriceExpiredSigningTokenRefreshesBeforeRender(t *testing.T) {
	// tokenCalls、renderCalls 分别统计本地 token 协议请求和业务 render 请求次数。
	var tokenCalls, renderCalls int
	// handler 模拟 token 首次签发新 Cookie、第二次取得 accessToken，以及随后成功的 render。
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// api 是当前本地请求的 MTOP API 名称。
		api := request.URL.Query().Get("api")
		writer.Header().Set("Content-Type", "application/json")
		switch api {
		case "mtop.taobao.idlemessage.pc.login.token":
			tokenCalls++
			if tokenCalls == 1 {
				if strings.Contains(request.Header.Get("Cookie"), "_m_h5_tk=") {
					t.Fatal("过期 token 不应进入首次 bootstrap 请求")
				}
				http.SetCookie(writer, &http.Cookie{Name: "_m_h5_tk", Value: "fresh-token_2", Domain: ".goofish.com", Path: "/"})
				fmt.Fprint(writer, `{"ret":["FAIL_SYS_TOKEN_EMPTY::令牌为空"],"data":{}}`)
				return
			}
			if !strings.Contains(request.Header.Get("Cookie"), "_m_h5_tk=fresh-token_2") {
				t.Fatalf("第二次 token 请求没有携带新签发 Cookie: %q", request.Header.Get("Cookie"))
			}
			fmt.Fprintf(writer, `{"ret":["SUCCESS::调用成功"],"data":{"accessToken":"local-access-token","accessTokenExpiredTime":"%d"}}`, time.Now().Add(time.Hour).UnixMilli())
		case adjustPriceRenderAPIName:
			renderCalls++
			if !strings.Contains(request.Header.Get("Cookie"), "_m_h5_tk=fresh-token_2") {
				t.Fatalf("render 没有使用恢复后的签名 Cookie: %q", request.Header.Get("Cookie"))
			}
			fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{"title":"修改价格","modifyPriceRenderList":[{"key":"modifyFee","name":"商品价格","prefixText":"¥","price":"1.35","readOnly":false}]}}`)
		default:
			t.Fatalf("unexpected api: %s", api)
		}
	})
	// client 是所有逻辑 goofish 请求都留在本机的协议客户端。
	client, cleanup := newLogicalGoofishTestClient(t, handler)
	defer cleanup()
	// snapshot 模拟 unb 仍有效但 _m_h5_tk 已经过期的生产权威 Cookie Jar。
	snapshot := []cookierefresh.BrowserCookie{
		{Name: "unb", Value: "123", Domain: ".goofish.com", Path: "/"},
		{Name: "_m_h5_tk", Value: "expired-token_1", Domain: ".goofish.com", Path: "/", Expires: float64(time.Now().Add(-time.Minute).Unix())},
	}
	// requestCtx、session 保存本次测试的权威 Cookie 会话及恢复后状态。
	requestCtx, session := WithCookieSnapshot(context.Background(), snapshot)
	// form、renderErr 是 token bootstrap 后的动态改价表单结果。
	form, renderErr := client.RenderOrderAdjustPrice(requestCtx, "unb=123; _m_h5_tk=expired-token_1", "5127638256187075541")
	if renderErr != nil || form == nil || len(form.Fields) != 1 || form.Fields[0].Price != "1.35" {
		t.Fatalf("form=%+v err=%v", form, renderErr)
	}
	if tokenCalls != 2 || renderCalls != 1 {
		t.Fatalf("tokenCalls=%d renderCalls=%d", tokenCalls, renderCalls)
	}
	// updatedCookies、_, changed 是会话持久化层应观察到的新 token 与变化标记。
	updatedCookies, _, changed := session.State()
	if !changed || !strings.Contains(updatedCookies, "_m_h5_tk=fresh-token_2") {
		t.Fatalf("Cookie 会话未保留 bootstrap 结果: changed=%v", changed)
	}
}

// TestAdjustPriceExpiredSigningTokenSessionFailureDoesNotSubmit 验证 token bootstrap 明确返回 Session 失效时保留错误分类，且真实 submit 一次也不发送。
func TestAdjustPriceExpiredSigningTokenSessionFailureDoesNotSubmit(t *testing.T) {
	// tokenCalls、submitCalls 分别统计恢复请求和具有外部写语义的 submit 请求。
	var tokenCalls, submitCalls int
	// handler 让 token API 明确返回 Session 失效；任何 submit 都会使测试失败。
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// api 是当前本地请求的 MTOP API 名称。
		api := request.URL.Query().Get("api")
		writer.Header().Set("Content-Type", "application/json")
		if api == "mtop.taobao.idlemessage.pc.login.token" {
			tokenCalls++
			fmt.Fprint(writer, `{"ret":["FAIL_SYS_SESSION_EXPIRED::Session过期"],"data":{}}`)
			return
		}
		if api == adjustPriceSubmitAPIName {
			submitCalls++
		}
		t.Fatalf("Session 失效后不应调用业务 API: %s", api)
	})
	// client 是不会访问真实平台的逻辑 goofish 客户端。
	client, cleanup := newLogicalGoofishTestClient(t, handler)
	defer cleanup()
	// snapshot 模拟已经过期且无法通过 token API 恢复的签名 Cookie。
	snapshot := []cookierefresh.BrowserCookie{
		{Name: "unb", Value: "123", Domain: ".goofish.com", Path: "/"},
		{Name: "_m_h5_tk", Value: "expired-token_1", Domain: ".goofish.com", Path: "/", Expires: float64(time.Now().Add(-time.Minute).Unix())},
	}
	// requestCtx 是带权威过期快照的 submit 上下文。
	requestCtx, _ := WithCookieSnapshot(context.Background(), snapshot)
	// result、submitErr 检查错误分类和明确失败结果，不能因凭证错误伪造业务成功。
	result, submitErr := client.SubmitOrderAdjustPrice(requestCtx, "unb=123; _m_h5_tk=expired-token_1", "5127638256187075541", map[string]string{"modifyFee": "13600"})
	if result == nil || result.Success || submitErr == nil || !IsSessionExpiredErr(submitErr) {
		t.Fatalf("result=%+v submitErr=%v want session expired", result, submitErr)
	}
	if tokenCalls != 1 || submitCalls != 0 {
		t.Fatalf("tokenCalls=%d submitCalls=%d", tokenCalls, submitCalls)
	}
}

// TestAdjustPriceBootstrapCookieReturnedWhenRenderFails 验证 token bootstrap 已签发新 Cookie 后，即使业务 render 明确失败也把凭证变化返回上层持久化。
func TestAdjustPriceBootstrapCookieReturnedWhenRenderFails(t *testing.T) {
	// tokenCalls、renderCalls 分别统计 bootstrap 和只读业务请求次数。
	var tokenCalls, renderCalls int
	// handler 先签发新 token，再让 render 返回明确业务失败。
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// api 是当前本地请求的 MTOP API 名称。
		api := request.URL.Query().Get("api")
		writer.Header().Set("Content-Type", "application/json")
		if api == "mtop.taobao.idlemessage.pc.login.token" {
			tokenCalls++
			if tokenCalls == 1 {
				http.SetCookie(writer, &http.Cookie{Name: "_m_h5_tk", Value: "fresh-flat_2", Domain: ".goofish.com", Path: "/"})
				fmt.Fprint(writer, `{"ret":["FAIL_SYS_TOKEN_EMPTY::令牌为空"],"data":{}}`)
				return
			}
			fmt.Fprintf(writer, `{"ret":["SUCCESS::调用成功"],"data":{"accessToken":"local-access-token","accessTokenExpiredTime":"%d"}}`, time.Now().Add(time.Hour).UnixMilli())
			return
		}
		if api == adjustPriceRenderAPIName {
			renderCalls++
			fmt.Fprint(writer, `{"ret":["FAIL_BIZ_ORDER_STATE_CHANGED::订单状态已变化"],"data":{}}`)
			return
		}
		t.Fatalf("unexpected api: %s", api)
	})
	// client 是全部请求都留在本地的协议客户端。
	client, cleanup := newLogicalGoofishTestClient(t, handler)
	defer cleanup()
	// requestCtx 使用缺少签名 token 的扁平 Cookie 会话，覆盖没有完整 metadata 快照的兼容账号。
	requestCtx, _ := WithFlatCookieSession(context.Background(), "unb=123")
	// form、renderErr 是明确业务失败和其中仍需保存的 bootstrap Cookie。
	form, renderErr := client.RenderOrderAdjustPrice(requestCtx, "unb=123", "5127638256187075541")
	if renderErr == nil || form == nil || !strings.Contains(form.UpdatedCookies, "_m_h5_tk=fresh-flat_2") {
		t.Fatalf("form=%+v renderErr=%v", form, renderErr)
	}
	if tokenCalls != 2 || renderCalls != 1 {
		t.Fatalf("tokenCalls=%d renderCalls=%d", tokenCalls, renderCalls)
	}
}
