#!/bin/sh
set -eu

# 将保留 LaunchAgent 和数据目录安装逻辑的 PKG 封装为用户可挂载的 DMG。
VERSION="${1:?version is required}"
DIST_DIR="${2:?dist directory is required}"
ARCH="${3:?architecture is required (arm64 or amd64)}"
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
PACKAGE_PATH="$DIST_DIR/CoverAI-Xianyu-Helper-$VERSION-$ARCH.pkg"
DMG_ROOT="$DIST_DIR/dmgroot-$ARCH"
DMG_PATH="$DIST_DIR/CoverAI-Xianyu-Helper-$VERSION-$ARCH.dmg"
TEMP_DMG_PATH="$DIST_DIR/.CoverAI-Xianyu-Helper-$VERSION-$ARCH.dmg"

case "$ARCH" in
  arm64|amd64) ;;
  *) echo "不支持的 macOS 架构：$ARCH" >&2; exit 1 ;;
esac

if [ ! -f "$PACKAGE_PATH" ]; then
  echo "缺少待封装的 PKG：$PACKAGE_PATH" >&2
  exit 1
fi

rm -rf "$DMG_ROOT"
mkdir -p "$DMG_ROOT"
cp "$PACKAGE_PATH" "$DMG_ROOT/安装 CoverAI 闲鱼助手.pkg"
cp "$SCRIPT_DIR/DMG安装说明.txt" "$DMG_ROOT/安装说明.txt"
rm -f "$DMG_PATH" "$TEMP_DMG_PATH"
hdiutil create \
  -quiet \
  -volname "CoverAI 闲鱼助手" \
  -srcfolder "$DMG_ROOT" \
  -format UDZO \
  -imagekey zlib-level=9 \
  "$TEMP_DMG_PATH"
mv "$TEMP_DMG_PATH" "$DMG_PATH"

if [ -n "${MACOS_SIGNING_IDENTITY:-}" ]; then
  if [ -n "${MACOS_SIGNING_KEYCHAIN:-}" ]; then
    codesign --force --sign "$MACOS_SIGNING_IDENTITY" --keychain "$MACOS_SIGNING_KEYCHAIN" "$DMG_PATH"
  else
    codesign --force --sign "$MACOS_SIGNING_IDENTITY" "$DMG_PATH"
  fi
  codesign --verify --verbose=2 "$DMG_PATH"
fi

hdiutil imageinfo "$DMG_PATH" >/dev/null
echo "已生成 macOS DMG：$DMG_PATH"
