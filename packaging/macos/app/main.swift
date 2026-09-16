import AppKit
import WebKit

/// NativeAppDelegate 拥有 macOS 主窗口、菜单栏、WKWebView 和后台服务生命周期。
final class NativeAppDelegate: NSObject, NSApplicationDelegate, NSWindowDelegate, WKNavigationDelegate, WKUIDelegate, WKDownloadDelegate {
    /// configuration 是原生壳允许访问的本地服务范围。
    private let configuration = AppConfiguration.load()
    /// serviceController 串行启停安装后的 Go 服务。
    private lazy var serviceController = ServiceController(configuration: configuration)
    /// launchInBackground 表示当前由登录项启动，只显示菜单栏而不打扰用户。
    private let launchInBackground = CommandLine.arguments.contains("--background")
    /// smokeTest 让打包验收在首个本地页面完成后自行退出，不停止真实服务。
    private let smokeTest = CommandLine.arguments.contains("--smoke-test")
    /// window 是用户看到的标准 macOS 主窗口。
    private var window: NSWindow!
    /// webView 在原生窗口中持久显示本地 React 管理界面。
    private var webView: WKWebView!
    /// statusItem 保留原有菜单栏服务入口。
    private var statusItem: NSStatusItem!
    /// statusMenuItem 显示最近一次健康检查的结果。
    private var statusMenuItem: NSMenuItem!
    /// serviceActionItems 在某个服务转换进行时统一禁用，避免并发操作。
    private var serviceActionItems: [NSMenuItem] = []
    /// statusTimer 每五秒更新菜单栏状态，不干预业务页面。
    private var statusTimer: Timer?
    /// transitionInProgress 防止用户快速重复触发启停服务。
    private var transitionInProgress = false
    /// terminationAllowed 确保 Command-Q 只在后台服务已收束后退出原生壳。
    private var terminationAllowed = false
    /// terminationInProgress 防止退出流程重复启动服务停止操作。
    private var terminationInProgress = false
    /// activityToken 告知 macOS 当前 App 维持实时客服连接，窗口隐藏后也不应被 App Nap 暂停。
    private var activityToken: NSObjectProtocol?
    /// activeDownloads 持有正在下载的 WebKit 任务，避免委托在完成前释放。
    private var activeDownloads: [ObjectIdentifier: WKDownload] = [:]

    /// applicationDidFinishLaunching 创建原生 UI，然后确保本地服务就绪并加载仪表盘。
    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(launchInBackground ? .accessory : .regular)
        DistributedNotificationCenter.default().addObserver(
            self,
            selector: #selector(handleShowWindowNotification(_:)),
            name: nativeAppShowNotification,
            object: nil
        )
        activityToken = ProcessInfo.processInfo.beginActivity(
            options: [.userInitiatedAllowingIdleSystemSleep, .suddenTerminationDisabled],
            reason: "保持实时客服消息和提示音连接"
        )
        buildApplicationMenu()
        buildStatusItem()
        buildWindow()
        showLoadingPage(title: "正在启动 CoverAI 闲鱼助手", detail: "正在连接本地服务…")
        if !launchInBackground {
            showMainWindow()
        }
        ensureServiceAndLoadPage()
        statusTimer = Timer.scheduledTimer(withTimeInterval: 5, repeats: true) { [weak self] _ in
            self?.refreshServiceStatus()
        }
    }

    /// applicationShouldHandleReopen 让 Dock 点击或再次打开 App 恢复已隐藏的主窗口。
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        showMainWindow()
        return true
    }

    /// applicationShouldTerminate 保留原托盘“退出前停止服务”的安全语义。
    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        if terminationAllowed { return .terminateNow }
        if terminationInProgress { return .terminateLater }
        terminationInProgress = true
        setTransitionState(active: true, title: "服务状态：正在退出")
        serviceController.perform(.stop) { [weak self] result in
            guard let self else { return }
            if case .failure(let error) = result {
                self.presentError(title: "退出前停止服务失败", error: error)
                self.terminationInProgress = false
                self.setTransitionState(active: false, title: "服务状态：操作失败")
                sender.reply(toApplicationShouldTerminate: false)
                return
            }
            self.terminationAllowed = true
            sender.reply(toApplicationShouldTerminate: true)
        }
        return .terminateLater
    }

    /// applicationWillTerminate 停止本地状态轮询。
    func applicationWillTerminate(_ notification: Notification) {
        statusTimer?.invalidate()
        DistributedNotificationCenter.default().removeObserver(self)
        if let activityToken {
            ProcessInfo.processInfo.endActivity(activityToken)
        }
    }

    /// windowShouldClose 只隐藏主窗口，让后台服务和菜单栏继续运行。
    func windowShouldClose(_ sender: NSWindow) -> Bool {
        sender.orderOut(nil)
        return false
    }

    /// buildWindow 创建拥有永久网站数据和自动音频播放能力的 WKWebView。
    private func buildWindow() {
        let webConfiguration = WKWebViewConfiguration()
        webConfiguration.websiteDataStore = .default()
        webConfiguration.mediaTypesRequiringUserActionForPlayback = []
        let nativeRuntimeScript = WKUserScript(
            source: "window.__COVERAI_NATIVE_APP__ = true;",
            injectionTime: .atDocumentStart,
            forMainFrameOnly: false
        )
        webConfiguration.userContentController.addUserScript(nativeRuntimeScript)

        webView = WKWebView(frame: .zero, configuration: webConfiguration)
        webView.navigationDelegate = self
        webView.uiDelegate = self
        webView.allowsBackForwardNavigationGestures = true

        window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1440, height: 900),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.title = "CoverAI 闲鱼助手"
        window.titlebarAppearsTransparent = false
        window.minSize = NSSize(width: 1080, height: 700)
        window.center()
        window.delegate = self
        window.contentView = webView
    }

    /// buildApplicationMenu 提供标准退出、复制粘贴、刷新和窗口快捷键。
    private func buildApplicationMenu() {
        let mainMenu = NSMenu()
        let applicationItem = NSMenuItem()
        let applicationMenu = NSMenu()
        applicationMenu.addItem(withTitle: "关于 CoverAI 闲鱼助手", action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)), keyEquivalent: "")
        applicationMenu.addItem(.separator())
        applicationMenu.addItem(withTitle: "退出 CoverAI 闲鱼助手", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        applicationItem.submenu = applicationMenu
        mainMenu.addItem(applicationItem)

        let editItem = NSMenuItem()
        let editMenu = NSMenu(title: "编辑")
        editMenu.addItem(withTitle: "撤销", action: Selector(("undo:")), keyEquivalent: "z")
        editMenu.addItem(withTitle: "重做", action: Selector(("redo:")), keyEquivalent: "Z")
        editMenu.addItem(.separator())
        editMenu.addItem(withTitle: "剪切", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        editMenu.addItem(withTitle: "复制", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        editMenu.addItem(withTitle: "粘贴", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        editMenu.addItem(withTitle: "全选", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")
        editItem.submenu = editMenu
        mainMenu.addItem(editItem)

        let viewItem = NSMenuItem()
        let viewMenu = NSMenu(title: "显示")
        let reloadItem = NSMenuItem(title: "刷新", action: #selector(reloadPage), keyEquivalent: "r")
        reloadItem.target = self
        viewMenu.addItem(reloadItem)
        viewItem.submenu = viewMenu
        mainMenu.addItem(viewItem)

        let windowItem = NSMenuItem()
        let windowMenu = NSMenu(title: "窗口")
        windowMenu.addItem(withTitle: "最小化", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
        let showItem = NSMenuItem(title: "显示主窗口", action: #selector(showMainWindowAction), keyEquivalent: "0")
        showItem.target = self
        windowMenu.addItem(showItem)
        windowItem.submenu = windowMenu
        mainMenu.addItem(windowItem)
        NSApp.mainMenu = mainMenu
    }

    /// buildStatusItem 创建显示窗口、服务控制、日志和退出入口。
    private func buildStatusItem() {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        if let button = statusItem.button {
            button.image = NSImage(systemSymbolName: "bolt.horizontal.circle", accessibilityDescription: "CoverAI 闲鱼助手")
        }
        let menu = NSMenu()
        let showItem = NSMenuItem(title: "显示主窗口", action: #selector(showMainWindowAction), keyEquivalent: "")
        showItem.target = self
        menu.addItem(showItem)
        statusMenuItem = NSMenuItem(title: "服务状态：检查中", action: nil, keyEquivalent: "")
        statusMenuItem.isEnabled = false
        menu.addItem(statusMenuItem)
        menu.addItem(.separator())

        let startItem = NSMenuItem(title: "启动服务", action: #selector(startServiceAction), keyEquivalent: "")
        let stopItem = NSMenuItem(title: "停止服务", action: #selector(stopServiceAction), keyEquivalent: "")
        let restartItem = NSMenuItem(title: "重启服务", action: #selector(restartServiceAction), keyEquivalent: "")
        for item in [startItem, stopItem, restartItem] {
            item.target = self
            menu.addItem(item)
        }
        serviceActionItems = [startItem, stopItem, restartItem]
        menu.addItem(.separator())
        let logsItem = NSMenuItem(title: "打开日志目录", action: #selector(openLogsAction), keyEquivalent: "")
        logsItem.target = self
        menu.addItem(logsItem)
        let quitItem = NSMenuItem(title: "退出并停止服务", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "")
        menu.addItem(quitItem)
        statusItem.menu = menu
    }

    /// ensureServiceAndLoadPage 在首次打开时复用已健康服务，否则先通过 launchd 启动。
    private func ensureServiceAndLoadPage() {
        serviceController.probe { [weak self] ready in
            guard let self else { return }
            if ready {
                self.markServiceReadyAndLoad()
                return
            }
            self.setTransitionState(active: true, title: "服务状态：正在启动")
            self.serviceController.perform(.start) { [weak self] result in
                guard let self else { return }
                self.setTransitionState(active: false, title: "服务状态：运行正常")
                switch result {
                case .success:
                    self.markServiceReadyAndLoad()
                case .failure(let error):
                    self.showLoadingPage(title: "本地服务启动失败", detail: error.localizedDescription, showsRetry: true)
                    self.presentError(title: "无法启动 CoverAI 闲鱼助手", error: error)
                }
            }
        }
    }

    /// markServiceReadyAndLoad 同步菜单栏状态并在当前原生窗口加载仪表盘。
    private func markServiceReadyAndLoad() {
        statusMenuItem.title = "服务状态：运行正常"
        webView.load(URLRequest(url: configuration.dashboardURL, cachePolicy: .reloadIgnoringLocalCacheData))
    }

    /// refreshServiceStatus 定时更新菜单栏状态，服务转换期间不覆盖中间文案。
    private func refreshServiceStatus() {
        if transitionInProgress { return }
        serviceController.probe { [weak self] ready in
            self?.statusMenuItem.title = ready ? "服务状态：运行正常" : "服务状态：未运行"
        }
    }

    /// runServiceAction 处理菜单触发的状态转换，成功后原地恢复页面。
    private func runServiceAction(_ action: ServiceController.Action, transitionTitle: String) {
        if transitionInProgress { return }
        setTransitionState(active: true, title: transitionTitle)
        serviceController.perform(action) { [weak self] result in
            guard let self else { return }
            switch result {
            case .success:
                let running = action != .stop
                self.setTransitionState(active: false, title: running ? "服务状态：运行正常" : "服务状态：未运行")
                if running {
                    self.webView.load(URLRequest(url: self.configuration.dashboardURL, cachePolicy: .reloadIgnoringLocalCacheData))
                } else {
                    self.showLoadingPage(title: "后台服务已停止", detail: "可从菜单栏重新启动。", showsRetry: true)
                }
            case .failure(let error):
                self.setTransitionState(active: false, title: "服务状态：操作失败")
                self.presentError(title: "服务操作失败", error: error)
            }
        }
    }

    /// setTransitionState 统一更新菜单文案和服务操作可用性。
    private func setTransitionState(active: Bool, title: String) {
        transitionInProgress = active
        statusMenuItem.title = title
        serviceActionItems.forEach { $0.isEnabled = !active }
    }

    /// showLoadingPage 使用内嵌 HTML 呈现启动、停止或错误状态，不导航外部页面。
    private func showLoadingPage(title: String, detail: String, showsRetry: Bool = false) {
        let escapedTitle = title.replacingOccurrences(of: "&", with: "&amp;").replacingOccurrences(of: "<", with: "&lt;")
        let escapedDetail = detail.replacingOccurrences(of: "&", with: "&amp;").replacingOccurrences(of: "<", with: "&lt;")
        let retryButton = showsRetry ? "<button onclick=\"window.location.href='\(configuration.dashboardURL.absoluteString)'\">重试</button>" : ""
        let html = """
        <!doctype html><meta charset="utf-8"><style>
        body{margin:0;background:#f8fafc;color:#0f172a;font:15px -apple-system,BlinkMacSystemFont,sans-serif;display:grid;place-items:center;height:100vh}
        main{text-align:center;padding:48px}.logo{width:64px;height:64px;border-radius:18px;background:linear-gradient(145deg,#0ea5e9,#2563eb);margin:0 auto 22px;box-shadow:0 16px 35px #0ea5e933}
        h1{font-size:24px;margin:0 0 12px}p{color:#64748b;margin:0;max-width:560px;line-height:1.7}button{margin-top:24px;border:0;border-radius:10px;background:#0284c7;color:white;padding:10px 22px;font-weight:700}
        </style><main><div class="logo"></div><h1>\(escapedTitle)</h1><p>\(escapedDetail)</p>\(retryButton)</main>
        """
        webView.loadHTMLString(html, baseURL: configuration.serviceURL)
    }

    /// showMainWindow 切换为标准 Dock App 并把已有窗口移到最前。
    private func showMainWindow() {
        NSApp.setActivationPolicy(.regular)
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    /// presentError 以原生弹窗呈现不含凭据的本地错误。
    private func presentError(title: String, error: Error) {
        let alert = NSAlert()
        alert.messageText = title
        alert.informativeText = error.localizedDescription
        alert.alertStyle = .warning
        if window.isVisible {
            alert.beginSheetModal(for: window)
        }
    }

    /// showMainWindowAction 响应菜单栏或窗口菜单的显示动作。
    @objc private func showMainWindowAction() {
        showMainWindow()
    }

    /// handleShowWindowNotification 响应第二次打开 App 发出的跨进程显示请求。
    @objc private func handleShowWindowNotification(_ notification: Notification) {
        showMainWindow()
    }

    /// reloadPage 使用原生 Command-R 刷新当前管理页，提示音授权保持开启。
    @objc private func reloadPage() {
        if isApplicationURL(webView.url ?? configuration.dashboardURL, serviceURL: configuration.serviceURL) {
            webView.reload()
        } else {
            webView.load(URLRequest(url: configuration.dashboardURL))
        }
    }

    /// startServiceAction 从菜单栏启动后台服务。
    @objc private func startServiceAction() {
        runServiceAction(.start, transitionTitle: "服务状态：正在启动")
    }

    /// stopServiceAction 从菜单栏停止后台服务。
    @objc private func stopServiceAction() {
        runServiceAction(.stop, transitionTitle: "服务状态：正在停止")
    }

    /// restartServiceAction 从菜单栏重启后台服务。
    @objc private func restartServiceAction() {
        runServiceAction(.restart, transitionTitle: "服务状态：正在重启")
    }

    /// openLogsAction 打开当前用户的服务与原生壳日志目录。
    @objc private func openLogsAction() {
        let logsURL = FileManager.default.homeDirectoryForCurrentUser.appending(path: "Library/Logs/YdisksXianyuHelper")
        try? FileManager.default.createDirectory(at: logsURL, withIntermediateDirectories: true)
        NSWorkspace.shared.open(logsURL)
    }

    /// webView decidePolicyFor 保留同源本地路由，并只为退款密码放行支付宝白名单子框架。
    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction, decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        guard let url = navigationAction.request.url else {
            decisionHandler(.cancel)
            return
        }
        if isApplicationURL(url, serviceURL: configuration.serviceURL) {
            decisionHandler(.allow)
            return
        }
        let targetIsMainFrame = navigationAction.targetFrame?.isMainFrame ?? true
        if shouldAllowRefundVerificationFrame(
            mainDocumentURL: webView.url,
            candidate: url,
            targetIsMainFrame: targetIsMainFrame,
            serviceURL: configuration.serviceURL
        ) {
            decisionHandler(.allow)
            return
        }
        if navigationAction.navigationType == .linkActivated {
            NSWorkspace.shared.open(url)
        }
        decisionHandler(.cancel)
    }

    /// webView createWebViewWith 把 target=_blank 的本地下载或路由留在当前窗口，显式外链则交给系统。
    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration, for navigationAction: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? {
        guard navigationAction.targetFrame == nil, let url = navigationAction.request.url else { return nil }
        if isApplicationURL(url, serviceURL: self.configuration.serviceURL) {
            webView.load(navigationAction.request)
        } else {
            NSWorkspace.shared.open(url)
        }
        return nil
    }

    /// webView decidePolicyForResponse 把无法内联显示的 CSV 等附件交给 WebKit 下载流程。
    func webView(_ webView: WKWebView, decidePolicyFor navigationResponse: WKNavigationResponse, decisionHandler: @escaping (WKNavigationResponsePolicy) -> Void) {
        decisionHandler(navigationResponse.canShowMIMEType ? .allow : .download)
    }

    /// webView navigationActionDidBecome 接管由链接导航转换的下载任务。
    func webView(_ webView: WKWebView, navigationAction: WKNavigationAction, didBecome download: WKDownload) {
        retainDownload(download)
    }

    /// webView navigationResponseDidBecome 接管由 HTTP 响应转换的下载任务。
    func webView(_ webView: WKWebView, navigationResponse: WKNavigationResponse, didBecome download: WKDownload) {
        retainDownload(download)
    }

    /// retainDownload 保留下载并设置当前原生壳为委托。
    private func retainDownload(_ download: WKDownload) {
        activeDownloads[ObjectIdentifier(download)] = download
        download.delegate = self
    }

    /// download decideDestinationUsing 把附件保存到当前用户下载目录，重名时添加数字后缀。
    func download(_ download: WKDownload, decideDestinationUsing response: URLResponse, suggestedFilename: String, completionHandler: @escaping (URL?) -> Void) {
        let downloadsDirectory = FileManager.default.urls(for: .downloadsDirectory, in: .userDomainMask).first!
        let suggestedLeafName = URL(fileURLWithPath: suggestedFilename).lastPathComponent
        let safeFilename = suggestedLeafName.isEmpty ? "CoverAI-下载" : suggestedLeafName
        var destination = downloadsDirectory.appending(path: safeFilename)
        let baseName = destination.deletingPathExtension().lastPathComponent
        let fileExtension = destination.pathExtension
        var suffix = 1
        while FileManager.default.fileExists(atPath: destination.path) {
            let nextName = fileExtension.isEmpty ? "\(baseName)-\(suffix)" : "\(baseName)-\(suffix).\(fileExtension)"
            destination = downloadsDirectory.appending(path: nextName)
            suffix += 1
        }
        completionHandler(destination)
    }

    /// downloadDidFinish 释放已完成下载并在 Finder 中打开下载目录。
    func downloadDidFinish(_ download: WKDownload) {
        activeDownloads.removeValue(forKey: ObjectIdentifier(download))
        let downloadsDirectory = FileManager.default.urls(for: .downloadsDirectory, in: .userDomainMask).first!
        NSWorkspace.shared.open(downloadsDirectory)
    }

    /// download didFailWithError 释放失败任务并向用户显示原生错误。
    func download(_ download: WKDownload, didFailWithError error: Error, resumeData: Data?) {
        activeDownloads.removeValue(forKey: ObjectIdentifier(download))
        presentError(title: "文件下载失败", error: error)
    }

    /// webView didFinish 在本地页面完成导航后同步窗口标题并记录可验收证据。
    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        if let pageTitle = webView.title, !pageTitle.isEmpty {
            window.title = pageTitle
        }
        NSLog("CoverAI native window loaded: %@", webView.url?.absoluteString ?? "unknown")
        if smokeTest, let loadedURL = webView.url, loadedURL.path.hasPrefix("/app/") {
            terminationAllowed = true
            DispatchQueue.main.async {
                NSApp.terminate(nil)
            }
        }
    }

    /// webView didFail 在顶层导航失败时显示可重试的本地错误页。
    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        showLoadingPage(title: "无法加载管理界面", detail: error.localizedDescription, showsRetry: true)
    }

    /// webView didFailProvisionalNavigation 处理服务尚未就绪时的首次连接失败。
    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        showLoadingPage(title: "本地服务连接中断", detail: error.localizedDescription, showsRetry: true)
    }

    /// webViewWebContentProcessDidTerminate 在 WebKit 内容进程被系统回收后自动恢复当前页面。
    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
        webView.reload()
    }

    /// webView runJavaScriptAlertPanelWithMessage 把页面 alert 转为原生提示框。
    func webView(_ webView: WKWebView, runJavaScriptAlertPanelWithMessage message: String, initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping () -> Void) {
        let alert = NSAlert()
        alert.messageText = message
        alert.beginSheetModal(for: window) { _ in completionHandler() }
    }

    /// webView runJavaScriptConfirmPanelWithMessage 把页面 confirm 转为带确认和取消的原生提示框。
    func webView(_ webView: WKWebView, runJavaScriptConfirmPanelWithMessage message: String, initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (Bool) -> Void) {
        let alert = NSAlert()
        alert.messageText = message
        alert.addButton(withTitle: "确定")
        alert.addButton(withTitle: "取消")
        alert.beginSheetModal(for: window) { response in completionHandler(response == .alertFirstButtonReturn) }
    }

    /// webView runOpenPanelWith 使图片和文件上传使用标准 macOS 文件选择器。
    func webView(_ webView: WKWebView, runOpenPanelWith parameters: WKOpenPanelParameters, initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping ([URL]?) -> Void) {
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = parameters.allowsMultipleSelection
        panel.canChooseDirectories = parameters.allowsDirectories
        panel.canChooseFiles = true
        panel.beginSheetModal(for: window) { response in
            completionHandler(response == .OK ? panel.urls : nil)
        }
    }
}

guard let instanceGuard = SingleInstanceGuard.acquire() else {
    exit(0)
}
let application = NSApplication.shared
let applicationDelegate = NativeAppDelegate()
withExtendedLifetime(instanceGuard) {
    application.delegate = applicationDelegate
    application.run()
}
