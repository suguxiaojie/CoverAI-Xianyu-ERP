//go:build !windows

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// TestForegroundSignalContextIgnoresHangupAndHandlesTermination 验证 PTY 关闭不会终止服务，明确停服信号仍会取消根 Context。
func TestForegroundSignalContextIgnoresHangupAndHandlesTermination(t *testing.T) {
	signal.Reset(syscall.SIGHUP, syscall.SIGTERM)
	t.Cleanup(func() {
		signal.Reset(syscall.SIGHUP, syscall.SIGTERM)
	})
	// ctx、cancel 是按生产前台入口配置的信号 Context 和订阅释放函数。
	ctx, cancel := foregroundSignalContext(context.Background())
	defer cancel()
	// process 是当前隔离测试进程，只向自身发送可控信号。
	process, processErr := os.FindProcess(os.Getpid())
	if processErr != nil {
		t.Fatal(processErr)
	}
	if // hangupErr 是模拟 Codex／终端 PTY 关闭时发送 SIGHUP 的错误。
	hangupErr := process.Signal(syscall.SIGHUP); hangupErr != nil {
		t.Fatal(hangupErr)
	}
	select {
	case <-ctx.Done():
		t.Fatal("SIGHUP 不应取消服务根 Context")
	case <-time.After(50 * time.Millisecond):
	}
	if // terminateErr 是向同一测试进程发送明确 SIGTERM 停服信号的错误。
	terminateErr := process.Signal(syscall.SIGTERM); terminateErr != nil {
		t.Fatal(terminateErr)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("SIGTERM 未在一秒内取消服务根 Context")
	}
}
