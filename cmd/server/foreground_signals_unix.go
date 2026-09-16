//go:build !windows

package main

import (
	"context"
	"os/signal"
	"syscall"
)

// foregroundSignalContext 忽略终端断开产生的 SIGHUP，只让 SIGINT／SIGTERM 触发可等待的优雅停服。
func foregroundSignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	signal.Ignore(syscall.SIGHUP)
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}
