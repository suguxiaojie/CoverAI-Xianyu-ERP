#!/bin/sh
set -eu

# 使用 macOS 系统 AppKit/WebKit 编译原生窗口壳，不引入 Electron 或第三方 WebView runtime。
VERSION="${1:?version is required}"
DIST_DIR="${2:?dist directory is required}"
ARCH="${3:?architecture is required (arm64 or amd64)}"
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BIN_DIR="$DIST_DIR/$ARCH"
OUTPUT="$BIN_DIR/xianyu-app"

case "$ARCH" in
  arm64) TARGET_ARCH="arm64" ;;
  amd64) TARGET_ARCH="x86_64" ;;
  *) echo "不支持的 macOS 架构：$ARCH" >&2; exit 1 ;;
esac

SDK_PATH="$(xcrun --sdk macosx --show-sdk-path)"
mkdir -p "$BIN_DIR"
xcrun swiftc \
  -swift-version 5 \
  -O \
  -whole-module-optimization \
  -target "$TARGET_ARCH-apple-macos13.0" \
  -sdk "$SDK_PATH" \
  -framework AppKit \
  -framework WebKit \
  "$SCRIPT_DIR/app/AppSupport.swift" \
  "$SCRIPT_DIR/app/ServiceController.swift" \
  "$SCRIPT_DIR/app/SingleInstance.swift" \
  "$SCRIPT_DIR/app/main.swift" \
  -o "$OUTPUT"

actual_archs="$(lipo -archs "$OUTPUT")"
case " $actual_archs " in
  *" $TARGET_ARCH "*) ;;
  *) echo "原生壳架构错误：期望 $TARGET_ARCH，实际 $actual_archs" >&2; exit 1 ;;
esac
chmod 0755 "$OUTPUT"
echo "已构建 macOS $ARCH 原生窗口壳：version=$VERSION output=$OUTPUT"
