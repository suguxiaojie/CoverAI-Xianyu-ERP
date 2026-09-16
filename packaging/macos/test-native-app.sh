#!/bin/sh
set -eu

# 编译并运行不依赖真实服务、数据库或图形会话的原生壳纯函数测试。
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
TEST_DIR="$(mktemp -d /tmp/coverai-native-tests.XXXXXX)"
TEST_BIN="$TEST_DIR/app-support-tests"

xcrun swiftc \
  -swift-version 5 \
  -parse-as-library \
  "$SCRIPT_DIR/app/AppSupport.swift" \
  "$SCRIPT_DIR/app/AppSupportTests.swift" \
  -o "$TEST_BIN"
"$TEST_BIN"
