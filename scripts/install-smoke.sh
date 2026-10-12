#!/usr/bin/env bash
# S0-3 install matrix (ARCHITECTURE §10.3): install the package built by CI on this distro and run
# `tradeloom --smoke` under Xvfb. Runs as root inside a throwaway distro container.
set -euo pipefail
export GOTOOLCHAIN=local
kind="${1:?usage: install-smoke.sh deb|appimage}"
export DEBIAN_FRONTEND=noninteractive
# Containers only: WebKitGTK's bubblewrap sandbox cannot create namespaces inside an unprivileged container.
export WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1
export TRADELOOM_DATA_DIR="$(mktemp -d)/tradeloom"
case "$kind" in
  deb)
    apt-get update
    apt-get install -y --no-install-recommends xvfb xauth dbus ./dist/tradeloom_*_amd64_noorders.deb
    dbus-run-session -- xvfb-run -a tradeloom --smoke
    ;;
  appimage)
    if ! dnf -y makecache; then
      # Fedora 39 is end-of-life: point the repos at the archive.
      sed -i -e 's|^metalink=|#metalink=|' \
             -e 's|^#baseurl=http://download.example/pub/fedora/linux|baseurl=https://archives.fedoraproject.org/pub/archive/fedora/linux|' \
             /etc/yum.repos.d/*.repo
      dnf -y makecache
    fi
    dnf install -y gtk3 webkit2gtk4.1 xorg-x11-server-Xvfb xorg-x11-xauth dbus-daemon
    app="$(ls dist/Tradeloom-*-x86_64_noorders.AppImage)"
    chmod +x "$app"
    APPIMAGE_EXTRACT_AND_RUN=1 dbus-run-session -- xvfb-run -a "$app" --smoke
    ;;
  *) echo "unknown kind $kind" >&2; exit 2 ;;
esac
test -f "$TRADELOOM_DATA_DIR/app.db"
grep -qx clean "$TRADELOOM_DATA_DIR/lock"
echo "smoke ok: $(. /etc/os-release; echo "$PRETTY_NAME") ($kind)"
