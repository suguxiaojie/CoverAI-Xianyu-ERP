import AppKit
import Darwin
import Foundation

/// nativeAppShowNotification 是第二次打开 App 时通知既有进程恢复主窗口的本地事件名。
let nativeAppShowNotification = Notification.Name("com.ydisks.xianyu-helper.native.show-window")

/// SingleInstanceGuard 用当前用户缓存目录的非阻塞文件锁阻止重复原生壳。
final class SingleInstanceGuard {
    /// descriptor 是当前进程生命周期内持有的独占锁文件描述符。
    private let descriptor: Int32

    /// acquire 获取单实例锁；既有实例存在时请求它显示窗口并返回 nil。
    static func acquire() -> SingleInstanceGuard? {
        let cacheDirectory = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first!
            .appending(path: "YdisksXianyuHelper", directoryHint: .isDirectory)
        try? FileManager.default.createDirectory(at: cacheDirectory, withIntermediateDirectories: true)
        let lockURL = cacheDirectory.appending(path: "native-app.lock")
        let descriptor = Darwin.open(lockURL.path, O_CREAT | O_RDWR, S_IRUSR | S_IWUSR)
        guard descriptor >= 0 else { return nil }
        guard flock(descriptor, LOCK_EX | LOCK_NB) == 0 else {
            Darwin.close(descriptor)
            DistributedNotificationCenter.default().post(name: nativeAppShowNotification, object: nil)
            return nil
        }
        return SingleInstanceGuard(descriptor: descriptor)
    }

    /// init 保存已获取的锁描述符，不在运行期重新打开。
    private init(descriptor: Int32) {
        self.descriptor = descriptor
    }

    /// deinit 在原生壳真正退出时释放锁，锁文件本身可安全保留。
    deinit {
        flock(descriptor, LOCK_UN)
        Darwin.close(descriptor)
    }
}
