import Foundation

/// ServiceController 串行管理 macOS LaunchAgent，并用健康接口确认操作的最终状态。
final class ServiceController {
    /// Action 是原生菜单允许触发的三种服务状态转换。
    enum Action {
        case start
        case stop
        case restart
    }

    /// configuration 提供健康检查的本地地址。
    private let configuration: AppConfiguration
    /// label 是安装脚本注册的服务 LaunchAgent 标识。
    private let label: String
    /// operationQueue 保证启动、停止和重启不会并发改写 launchd 状态。
    private let operationQueue = DispatchQueue(label: "com.ydisks.xianyu-helper.native-service")
    /// session 为健康检查使用短超时且不缓存的本地 HTTP 会话。
    private let session: URLSession

    /// init 构造只依赖本地地址和可覆盖的 LaunchAgent 标识。
    init(configuration: AppConfiguration, environment: [String: String] = ProcessInfo.processInfo.environment) {
        self.configuration = configuration
        self.label = environment["XIANYU_SERVICE_NAME"] ?? "com.ydisks.xianyu-helper.server"
        let sessionConfiguration = URLSessionConfiguration.ephemeral
        sessionConfiguration.requestCachePolicy = .reloadIgnoringLocalCacheData
        sessionConfiguration.timeoutIntervalForRequest = 2
        sessionConfiguration.timeoutIntervalForResource = 2
        self.session = URLSession(configuration: sessionConfiguration)
    }

    /// probe 异步读取健康接口，完成回调回到主线程更新 UI。
    func probe(completion: @escaping (Bool) -> Void) {
        operationQueue.async { [weak self] in
            let ready = self?.probeSynchronously() ?? false
            DispatchQueue.main.async {
                completion(ready)
            }
        }
    }

    /// perform 串行执行指定服务动作，并在健康状态收敛后返回结果。
    func perform(_ action: Action, completion: @escaping (Result<Void, Error>) -> Void) {
        operationQueue.async { [weak self] in
            guard let self else { return }
            let result: Result<Void, Error>
            do {
                try self.performSynchronously(action)
                result = .success(())
            } catch {
                result = .failure(error)
            }
            DispatchQueue.main.async {
                completion(result)
            }
        }
    }

    /// performSynchronously 在专用队列中完成 launchctl 转换和健康收敛。
    private func performSynchronously(_ action: Action) throws {
        switch action {
        case .start:
            if probeSynchronously() { return }
            try startSynchronously()
            try waitForService(wantRunning: true, timeout: 30)
        case .stop:
            try stopSynchronously()
            try waitForService(wantRunning: false, timeout: 30)
        case .restart:
            try stopSynchronously()
            try startSynchronously()
            try waitForService(wantRunning: true, timeout: 30)
        }
    }

    /// startSynchronously 复用已注册 job，或从安装脚本生成的 plist 恢复注册。
    private func startSynchronously() throws {
        let domain = "gui/\(getuid())"
        let target = "\(domain)/\(label)"
        if runLaunchctl(["print", target]) {
            if runLaunchctl(["kickstart", target]) { return }
            _ = runLaunchctl(["bootout", target])
            try waitForLaunchAgentToDisappear(target: target, timeout: 10)
        }
        let plistURL = FileManager.default.homeDirectoryForCurrentUser
            .appending(path: "Library/LaunchAgents/\(label).plist")
        guard FileManager.default.fileExists(atPath: plistURL.path) else {
            throw NativeAppError("未找到已安装的后台服务配置：\(plistURL.path)")
        }
        try requireLaunchctl(["bootstrap", domain, plistURL.path])
        try requireLaunchctl(["kickstart", target])
    }

    /// stopSynchronously 卸载当前用户域的服务 job，已停止时保持幂等。
    private func stopSynchronously() throws {
        let target = "gui/\(getuid())/\(label)"
        if !runLaunchctl(["print", target]) { return }
        try requireLaunchctl(["bootout", target])
        try waitForLaunchAgentToDisappear(target: target, timeout: 10)
    }

    /// waitForLaunchAgentToDisappear 等待 launchd 完成卸载，避免立即 bootstrap 时冲突。
    private func waitForLaunchAgentToDisappear(target: String, timeout: TimeInterval) throws {
        let deadline = Date().addingTimeInterval(timeout)
        while runLaunchctl(["print", target]) {
            if Date() >= deadline {
                throw NativeAppError("等待后台服务退出超时")
            }
            Thread.sleep(forTimeInterval: 0.2)
        }
    }

    /// waitForService 以端口真实可达性为准确认启动或停止完成。
    private func waitForService(wantRunning: Bool, timeout: TimeInterval) throws {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            let state = probeStateSynchronously()
            if wantRunning && state.ready { return }
            if !wantRunning && !state.reachable { return }
            Thread.sleep(forTimeInterval: 0.25)
        }
        throw NativeAppError(wantRunning ? "等待后台服务启动超时" : "等待后台服务停止超时")
    }

    /// probeSynchronously 在专用队列中使用两秒超时读取最小健康载荷。
    private func probeSynchronously() -> Bool {
        probeStateSynchronously().ready
    }

    /// probeStateSynchronously 区分“完全不可达”与“进程存在但不健康”，停止验收只接受前者。
    private func probeStateSynchronously() -> (reachable: Bool, ready: Bool) {
        let healthURL = configuration.serviceURL.appending(path: "health")
        var request = URLRequest(url: healthURL)
        request.cachePolicy = .reloadIgnoringLocalCacheData
        let semaphore = DispatchSemaphore(value: 0)
        var reachable = false
        var ready = false
        let task = session.dataTask(with: request) { data, response, error in
            defer { semaphore.signal() }
            guard error == nil, let response = response as? HTTPURLResponse else { return }
            reachable = true
            guard (200..<300).contains(response.statusCode),
                  let data,
                  let payload = try? JSONDecoder().decode(HealthPayload.self, from: data) else {
                return
            }
            ready = payload.isReady
        }
        task.resume()
        _ = semaphore.wait(timeout: .now() + 3)
        return (reachable, ready)
    }

    /// requireLaunchctl 执行必须成功的 launchctl 命令，失败时保留脱敏输出。
    private func requireLaunchctl(_ arguments: [String]) throws {
        let result = launchctlResult(arguments)
        if result.succeeded { return }
        throw NativeAppError(result.output.isEmpty ? "launchctl \(arguments.joined(separator: " ")) 失败" : result.output)
    }

    /// runLaunchctl 执行只需布尔结果的 launchctl 探测。
    private func runLaunchctl(_ arguments: [String]) -> Bool {
        launchctlResult(arguments).succeeded
    }

    /// launchctlResult 同步执行系统 launchctl，不记录业务凭据或环境值。
    private func launchctlResult(_ arguments: [String]) -> (succeeded: Bool, output: String) {
        let process = Process()
        let outputPipe = Pipe()
        process.executableURL = URL(fileURLWithPath: "/bin/launchctl")
        process.arguments = arguments
        process.standardOutput = outputPipe
        process.standardError = outputPipe
        do {
            try process.run()
            process.waitUntilExit()
        } catch {
            return (false, error.localizedDescription)
        }
        let output = String(data: outputPipe.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8)?
            .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return (process.terminationStatus == 0, output)
    }
}

/// NativeAppError 是原生壳可直接显示的本地服务控制错误。
struct NativeAppError: LocalizedError {
    /// message 是不含凭据的用户可读错误文本。
    let message: String

    /// init 创建一条可直接显示的错误。
    init(_ message: String) {
        self.message = message
    }

    /// errorDescription 向 AppKit 暴露用户可读描述。
    var errorDescription: String? { message }
}
