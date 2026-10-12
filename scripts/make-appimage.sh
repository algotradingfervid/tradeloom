#!/usr/bin/env bash
# Builds dist/Tradeloom-<VERSION>-x86_64_noorders.AppImage from build/bin/tradeloom (ARCHITECTURE §10.3).
set -euo pipefail
export GOTOOLCHAIN=local
cd "$(dirname "$0")/.."
VERSION="${VERSION:?set VERSION}"
APPDIR=build/appimage/AppDir
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin" dist
install -m 0755 build/bin/tradeloom "$APPDIR/usr/bin/tradeloom"
install -m 0755 build/appimage/AppRun "$APPDIR/AppRun"
install -m 0644 build/linux/tradeloom.desktop "$APPDIR/tradeloom.desktop"
install -m 0644 build/appicon.png "$APPDIR/tradeloom.png"
install -D -m 0644 THIRD_PARTY_NOTICES.md "$APPDIR/usr/share/doc/tradeloom/THIRD_PARTY_NOTICES.md"
ARCH=x86_64 "${APPIMAGETOOL:-appimagetool}" "$APPDIR" "dist/Tradeloom-${VERSION}-x86_64_noorders.AppImage"
