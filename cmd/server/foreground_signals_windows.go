//go:build windows

package main

import (
	"context"
	"os"
	"os/signal"
)

// foregroundSignalContext 在 Windows 前台模式下使用系统中断信号取消服务根 Context。
func foregroundSignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt)
}
