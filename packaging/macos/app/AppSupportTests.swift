import Foundation

/// expect 在原生壳纯函数行为不符合预期时立即终止定向测试。
func expect(_ condition: @autoclosure () -> Bool, _ message: String) {
    if !condition() {
        fputs("FAIL: \(message)\n", stderr)
        exit(1)
    }
}

/// AppSupportTests 是可直接编译运行的原生壳纯函数定向测试入口。
@main
struct AppSupportTests {
    /// main 验证默认路由、同源门禁和健康状态判定。
    static func main() {
        let configuration = AppConfiguration.load(environment: [:])
        expect(configuration.serviceURL.absoluteString == "http://127.0.0.1:59188", "默认服务地址必须使用本机回环端口")
        expect(configuration.dashboardURL.absoluteString == "http://127.0.0.1:59188/app/dashboard", "默认窗口必须进入仪表盘")

        let serviceURL = configuration.serviceURL
        expect(isApplicationURL(URL(string: "http://127.0.0.1:59188/app/chat")!, serviceURL: serviceURL), "同源聊天路由应保持在 App 内")
        expect(!isApplicationURL(URL(string: "http://127.0.0.1:59189/app/chat")!, serviceURL: serviceURL), "不同端口不得在 App 内导航")
        expect(!isApplicationURL(URL(string: "https://www.goofish.com/")!, serviceURL: serviceURL), "外部平台链接必须交给系统")
        expect(isApplicationURL(URL(string: "about:blank")!, serviceURL: serviceURL), "初始空白页应允许在 App 内显示")

        let refundVerificationURL = URL(string: "https://pcauth-site.alipay.com/PASSWORD?token=test")!
        expect(isRefundVerificationURL(refundVerificationURL), "支付宝 PC 验证 HTTPS 地址应通过白名单")
        expect(isRefundVerificationURL(URL(string: "https://secure.pcauth-site.alipay.com/PASSWORD")!), "支付宝 PC 验证子域应通过白名单")
        expect(!isRefundVerificationURL(URL(string: "http://pcauth-site.alipay.com/PASSWORD")!), "支付宝验证不得降级为 HTTP")
        expect(!isRefundVerificationURL(URL(string: "https://pcauth-site.alipay.com.evil.example/PASSWORD")!), "相似恶意域名不得绕过后缀门禁")
        expect(shouldAllowRefundVerificationFrame(mainDocumentURL: URL(string: "http://127.0.0.1:59188/app/chat")!, candidate: refundVerificationURL, targetIsMainFrame: false, serviceURL: serviceURL), "本地 ERP 中的支付宝子框架应允许加载")
        expect(!shouldAllowRefundVerificationFrame(mainDocumentURL: URL(string: "http://127.0.0.1:59188/app/chat")!, candidate: refundVerificationURL, targetIsMainFrame: true, serviceURL: serviceURL), "支付宝验证不得替换 ERP 主页面")
        expect(!shouldAllowRefundVerificationFrame(mainDocumentURL: URL(string: "https://evil.example/")!, candidate: refundVerificationURL, targetIsMainFrame: false, serviceURL: serviceURL), "非本地父页面不得借用支付宝子框架白名单")

        let healthyPayload = HealthPayload(status: "ok", database: "ok")
        let unhealthyPayload = HealthPayload(status: "ok", database: "error")
        expect(healthyPayload.isReady, "服务和数据库均正常时应就绪")
        expect(!unhealthyPayload.isReady, "数据库异常时不得标记就绪")

        print("macOS native app support tests passed")
    }
}
