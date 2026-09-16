import Foundation

/// AppConfiguration 保存原生窗口访问的本地服务地址和默认业务入口。
struct AppConfiguration {
    /// serviceURL 是只允许本机回环地址的服务根地址。
    let serviceURL: URL

    /// dashboardURL 是原生窗口首次启动时加载的仪表盘地址。
    var dashboardURL: URL {
        serviceURL.appending(path: "app/dashboard")
    }

    /// load 从运行环境读取可测试的本地地址，无配置时使用正式回环端口。
    static func load(environment: [String: String] = ProcessInfo.processInfo.environment) -> AppConfiguration {
        let configuredValue = environment["XIANYU_SERVICE_URL"]?.trimmingCharacters(in: .whitespacesAndNewlines)
        let fallbackValue = "http://127.0.0.1:59188"
        let serviceURL = URL(string: configuredValue?.isEmpty == false ? configuredValue! : fallbackValue) ?? URL(string: fallbackValue)!
        return AppConfiguration(serviceURL: serviceURL)
    }
}

/// HealthPayload 是本地服务健康接口中原生壳需要的最小字段。
struct HealthPayload: Decodable {
    /// status 表示 HTTP 服务自身的健康状态。
    let status: String
    /// database 表示本地数据库连接状态。
    let database: String

    /// isReady 只在服务和数据库同时正常时返回 true。
    var isReady: Bool {
        status == "ok" && database == "ok"
    }
}

/// isApplicationURL 限制 WKWebView 只在应用内导航同源本地页面，外部链接交给系统。
func isApplicationURL(_ candidate: URL, serviceURL: URL) -> Bool {
    guard let candidateScheme = candidate.scheme?.lowercased(),
          let serviceScheme = serviceURL.scheme?.lowercased(),
          let candidateHost = candidate.host?.lowercased(),
          let serviceHost = serviceURL.host?.lowercased() else {
        return candidate.scheme == "about"
    }
    return candidateScheme == serviceScheme &&
        candidateHost == serviceHost &&
        candidate.port == serviceURL.port
}

/// isRefundVerificationURL 只允许支付宝 PC 支付密码验证使用的 HTTPS 主机。
func isRefundVerificationURL(_ candidate: URL) -> Bool {
    guard candidate.scheme?.lowercased() == "https",
          let hostname = candidate.host?.lowercased() else {
        return false
    }
    return hostname == "pcauth-site.alipay.com" || hostname.hasSuffix(".pcauth-site.alipay.com")
}

/// shouldAllowRefundVerificationFrame 只放行本地 ERP 主页面发起的支付宝子框架导航。
func shouldAllowRefundVerificationFrame(
    mainDocumentURL: URL?,
    candidate: URL,
    targetIsMainFrame: Bool,
    serviceURL: URL
) -> Bool {
    guard !targetIsMainFrame,
          let mainDocumentURL,
          isApplicationURL(mainDocumentURL, serviceURL: serviceURL) else {
        return false
    }
    return isRefundVerificationURL(candidate)
}
