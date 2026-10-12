> **P0-1 Platform Core Implementation Plan — Task 16.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 16: go-arch-lint, G-0 symbol check, licences, Docker Linux build, packaging and CI

**Files:**
- Create: `.go-arch-lint.yml`, `scripts/g0-nm-check.sh`, `scripts/gen-notices.sh`, `scripts/make-appimage.sh`, `scripts/install-smoke.sh`, `build/notices.tpl`, `build/nfpm.yaml`, `build/linux/tradeloom.desktop`, `build/appimage/AppRun`, `build/Dockerfile.linux`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `THIRD_PARTY_NOTICES.md` (generated)
- Modify: `Makefile` (full version)

**Interfaces:**
- Consumes: every package from Tasks 1–15; `--smoke` from Task 15; `build/appicon.png` from Task 1.
- Produces:
  - Make targets: `test`, `test-race`, `archtest`, `arch-lint`, `frontend-test`, `build`, `g0-check`, `licenses`, `notices`, `deb`, `appimage`, `docker-image`, `docker-release`, `release-in-container`.
  - Artefacts in `dist/`: `tradeloom_<VERSION>_amd64_noorders.deb`, `Tradeloom-<VERSION>-x86_64_noorders.AppImage`, `SHA256SUMS`.
  - CI jobs (`ci.yml`): `go-unit`, `ta`, `arch-lint`, `frontend`, `licence`, `linux-package`, `install-smoke` (matrix ubuntu:22.04, ubuntu:24.04, debian:12 → `.deb`; fedora:39 → AppImage). `ci.yml` is also a reusable workflow (`workflow_call` with input `version`) used by `release.yml` on `v*` tags.
  - `.go-arch-lint.yml` components for P0-1 packages; later sub-plans add their components/vendors (e.g. `kite` with `canUse: [kite]`) in the same file.

- [ ] **Step 1: Verify the checks do not exist yet (failing)**

Run: `go-arch-lint check; make g0-check`
Expected: go-arch-lint prints `failed to assemble project info: not found archfile in '…/.go-arch-lint.yml'`; make prints `No rule to make target 'g0-check'`.

- [ ] **Step 2: Write `.go-arch-lint.yml`**

```yaml
# go-arch-lint v1.19.0 — ARCHITECTURE §2.4 rules 2, 3, 5, 6, 7 and the component graph.
# go-arch-lint does not see standard-library imports, so rule 1/4 (database/sql, net/http), rule 8 (time)
# and the G-0 call check live in internal/archtest. Later sub-plans add their components and vendors here.
version: 3
workdir: .
allow:
  depOnAnyVendor: false
  deepScan: false   # interface injection (store.SettingsRepo → settings.Backend) is the intended design
exclude:
  - ta
  - third_party
  - frontend/node_modules
  - internal/archtest/testdata
excludeFiles:
  - "^.*_test\\.go$"

vendors:
  wails:      { in: [github.com/wailsapp/wails/v2, github.com/wailsapp/wails/v2/**] }
  kite:       { in: [github.com/zerodha/gokiteconnect/v4, github.com/zerodha/gokiteconnect/v4/**] }
  sqlite:     { in: [modernc.org/sqlite, modernc.org/sqlite/**] }
  keyring:    { in: [github.com/zalando/go-keyring] }
  xcrypto:    { in: [golang.org/x/crypto/**] }
  flock:      { in: [github.com/gofrs/flock] }
  lumberjack: { in: [gopkg.in/natefinch/lumberjack.v2] }
  zstd:       { in: [github.com/klauspost/compress/zstd] }
  xtools:     { in: [golang.org/x/tools/go/packages] }

components:
  cmd:        { in: cmd/tradeloom }
  frontend:   { in: frontend }
  app:        { in: internal/app }
  bus:        { in: internal/bus }
  clock:      { in: internal/clock }
  logging:    { in: internal/logging }
  settings:   { in: internal/settings }
  store:      { in: internal/store }
  secrets:    { in: internal/secrets }
  keyringprobe: { in: internal/secrets/keyringprobe }   # S0-4 spike tool
  orderwrite: { in: internal/kite/orderwrite }
  archtest:   { in: internal/archtest }

commonComponents:
  - logging
  - clock

deps:
  cmd:
    mayDependOn: [frontend, app, store]
    canUse: [wails]
  app:
    mayDependOn: [bus, settings, store, secrets]
    canUse: [wails]
  settings:
    mayDependOn: [bus]
  store:
    canUse: [sqlite, flock, zstd]
  secrets:
    canUse: [keyring, xcrypto]
  keyringprobe:
    mayDependOn: [secrets]
  logging:
    canUse: [lumberjack]
  orderwrite:
    canUse: [kite]
  archtest:
    canUse: [xtools]
```

Run: `go-arch-lint check`
Expected: `OK - No warnings found`.
Prove it bites, then revert:

```bash
printf 'package store\n\nimport _ "github.com/zalando/go-keyring"\n' > internal/store/zz_probe.go
go-arch-lint check --output-color=false | grep "shouldn't depend"
rm internal/store/zz_probe.go
```
Expected: `Component store shouldn't depend on github.com/zalando/go-keyring in …/internal/store/zz_probe.go:3`.

- [ ] **Step 3: Write the G-0 symbol check and the full Makefile**

`scripts/g0-nm-check.sh` (Wails strips production binaries with `-w -s` — `pkg/commands/build/base.go` — so the check links its own unstripped binary with the same `desktop,production` tags Wails passes):

```bash
#!/usr/bin/env bash
# G-0 check 2 (ARCHITECTURE §2.5): the noorders binary must not link any Kite order-write method.
# Usage: scripts/g0-nm-check.sh <unstripped binary>   (wails build strips with -s -w, so CI builds a
# separate unstripped binary with plain `go build` for this check).
set -euo pipefail
bin="${1:?usage: g0-nm-check.sh <unstripped binary>}"
syms="$(go tool nm -sort name "$bin")"
if ! grep -qE ' main\.main$' <<<"$syms"; then
  echo "g0: $bin has no Go symbol table (stripped?) — refusing to pass" >&2
  exit 2
fi
if grep -E 'gokiteconnect/v4\.\(\*Client\)\.(PlaceOrder|ModifyOrder|CancelOrder|ExitOrder|ConvertPosition|PlaceGTT|ModifyGTT|DeleteGTT)$' <<<"$syms"; then
  echo "g0: order-write methods are linked into $bin" >&2
  exit 1
fi
echo "g0: ok — no order-write methods in $bin"
```

`Makefile` (full replacement):

```make
# Tradeloom developer and CI entry points. P0–P3 builds always carry the noorders tag (G-0).
GO        ?= go
TAGS      ?= webkit2_41,noorders
VERSION   ?= 0.0.0+$(shell git rev-parse --short HEAD 2>/dev/null || echo local)
LDFLAGS   := -X tradeloom/internal/app.Version=$(VERSION)
DOCKER_IMAGE ?= tradeloom-build:p0
ALLOWED_LICENSES := MIT,BSD-2-Clause,BSD-3-Clause,Apache-2.0,ISC,MPL-2.0

.PHONY: test test-race archtest arch-lint frontend-test build g0-check licenses notices deb appimage \
        docker-image docker-release release-in-container

test:
	$(GO) test -count=1 -tags $(TAGS) ./...

test-race:
	$(GO) test -race -count=1 -tags $(TAGS) ./...

archtest:
	$(GO) test -count=1 -tags $(TAGS) ./internal/archtest/...

arch-lint:
	go-arch-lint check

frontend-test:
	cd frontend && npm ci && npm run typecheck && npm test

# Production binary (stripped by wails): build/bin/tradeloom.
build:
	cd cmd/tradeloom && wails build -m -nosyncgomod -tags "$(TAGS)" -ldflags "$(LDFLAGS)"

# G-0 check 2 needs symbols, so it links a separate unstripped binary with the same tags wails uses.
g0-check:
	$(GO) build -tags desktop,production,$(TAGS) -ldflags "$(LDFLAGS)" -o build/bin/tradeloom-g0 ./cmd/tradeloom
	scripts/g0-nm-check.sh build/bin/tradeloom-g0

licenses:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go-licenses check ./cmd/tradeloom \
	  --allowed_licenses=$(ALLOWED_LICENSES) --ignore tradeloom --ignore modernc.org/mathutil
	cd frontend && npx --yes license-checker@25.0.1 --production --excludePrivatePackages \
	  --onlyAllow 'MIT;ISC;BSD-2-Clause;BSD-3-Clause;Apache-2.0'

notices:
	scripts/gen-notices.sh > THIRD_PARTY_NOTICES.md

deb:
	mkdir -p dist
	VERSION=$(VERSION) nfpm package -f build/nfpm.yaml -p deb -t dist/tradeloom_$(VERSION)_amd64_noorders.deb

appimage:
	VERSION=$(VERSION) scripts/make-appimage.sh

docker-image:
	docker build -f build/Dockerfile.linux -t $(DOCKER_IMAGE) build

# Full Linux release build inside the container: binary, G-0 symbol check, .deb, AppImage.
docker-release: docker-image
	docker run --rm -v "$(CURDIR)":/src -w /src $(DOCKER_IMAGE) make release-in-container VERSION=$(VERSION)

release-in-container: build g0-check deb appimage
	cd dist && sha256sum * > SHA256SUMS
```

Run (Linux; on macOS prefix `CGO_LDFLAGS="-framework UniformTypeIdentifiers"` because plain `go build` does not add the framework Wails' CLI adds):
`chmod +x scripts/*.sh && make g0-check`
Expected: `g0: ok — no order-write methods in build/bin/tradeloom-g0`.
Prove it bites, then revert:

```bash
mkdir -p internal/g0probe && cat > internal/g0probe/main.go <<'EOF'
package main

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

func main() { kiteconnect.New("x").PlaceOrder("regular", kiteconnect.OrderParams{}) }
EOF
go build -o /tmp/g0probe ./internal/g0probe && scripts/g0-nm-check.sh /tmp/g0probe; echo "exit=$?"
rm -rf internal/g0probe /tmp/g0probe
```
Expected: a line ending in `T github.com/zerodha/gokiteconnect/v4.(*Client).PlaceOrder`, then `g0: order-write methods are linked into /tmp/g0probe`, `exit=1`. (A stripped binary makes the script exit 2 instead of passing.)

- [ ] **Step 4: Licence check and notices**

`build/notices.tpl` (go-licenses v1.6.0 executes it over `[]libraryData{Name, LicenseURL, LicenseName, LicensePath, Version}` with method `LicenseText`):

````text
# Third-party notices

Generated by `make notices` from the Go dependency graph of `cmd/tradeloom` and the npm production
dependencies of `frontend/`. Do not edit by hand; CI fails when this file is stale.
{{ range . }}
## {{ .Name }} {{ .Version }} — {{ .LicenseName }}

```
{{ .LicenseText }}
```
{{ end }}
````

`scripts/gen-notices.sh`:

````bash
#!/usr/bin/env bash
# Writes THIRD_PARTY_NOTICES.md to stdout: Go licences of the cmd/tradeloom graph + npm production deps.
# Requires go-licenses v1.6.0 on PATH and `npm ci` done in frontend/.
set -euo pipefail
cd "$(dirname "$0")/.."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go-licenses report ./cmd/tradeloom \
  --ignore tradeloom --ignore modernc.org/mathutil --template build/notices.tpl 2>/dev/null
# go-licenses cannot classify modernc.org/mathutil's LICENSE (BSD-3-Clause, checked by hand); append it verbatim.
dir="$(go list -m -f '{{.Dir}}' modernc.org/mathutil)"
ver="$(go list -m -f '{{.Version}}' modernc.org/mathutil)"
printf '\n## modernc.org/mathutil %s — BSD-3-Clause\n\n```\n' "$ver"
cat "$dir/LICENSE"
printf '```\n\n# npm production dependencies (frontend/)\n\n'
(cd frontend && npx --yes license-checker@25.0.1 --production --excludePrivatePackages --markdown)
````

Run: `(cd frontend && npm ci) && make licenses && make notices && grep -c '^## ' THIRD_PARTY_NOTICES.md && make notices && git diff --stat THIRD_PARTY_NOTICES.md`
Expected: `make licenses` exits 0 (go-licenses reports only MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0 for the `cmd/tradeloom` graph; license-checker lists react, react-dom, scheduler as MIT); `THIRD_PARTY_NOTICES.md` has ≈ 26 `## ` sections, ends with the npm section, and regenerating it produces no diff (deterministic). `modernc.org/mathutil` is excluded from go-licenses because its classifier cannot match the file; its BSD-3-Clause `LICENSE` is appended verbatim by the script.

- [ ] **Step 5: Packaging files and the Linux build container**

`build/linux/tradeloom.desktop`:

```ini
[Desktop Entry]
Type=Application
Name=Tradeloom
GenericName=Trading strategy desktop
Comment=Strategy builder, charts and watchlists for Zerodha Kite
Exec=tradeloom
Icon=tradeloom
Terminal=false
Categories=Office;Finance;
StartupWMClass=tradeloom
```

`build/nfpm.yaml` (nfpm v2.47.0 expands `${VERSION}` from the environment; `libgtk-3-0t64 | libgtk-3-0` covers Ubuntu 24.04's time_t package rename and Ubuntu 22.04/Debian 12):

```yaml
# nfpm v2.47.0 — .deb for Ubuntu 22.04/24.04 and Debian 12 (ARCHITECTURE §10.3).
# Run from the repository root (make deb): VERSION=0.1.0 nfpm package -f build/nfpm.yaml -p deb -t dist/tradeloom_0.1.0_amd64_noorders.deb
name: tradeloom
arch: amd64
platform: linux
version: ${VERSION}
section: misc
priority: optional
maintainer: Tradeloom <tradeloom@localhost>
description: |
  Tradeloom: strategy builder, charts and watchlists for Zerodha Kite.
  This build cannot place orders (noorders).
homepage: https://localhost/tradeloom
license: Proprietary
depends:
  - libgtk-3-0t64 | libgtk-3-0
  - libwebkit2gtk-4.1-0
contents:
  - src: build/bin/tradeloom
    dst: /usr/bin/tradeloom
    file_info:
      mode: 0755
  - src: build/linux/tradeloom.desktop
    dst: /usr/share/applications/tradeloom.desktop
    file_info:
      mode: 0644
  - src: build/appicon.png
    dst: /usr/share/pixmaps/tradeloom.png
    file_info:
      mode: 0644
  - src: THIRD_PARTY_NOTICES.md
    dst: /usr/share/doc/tradeloom/THIRD_PARTY_NOTICES.md
    file_info:
      mode: 0644
```

`build/appimage/AppRun`:

```bash
#!/bin/sh
# AppImage entry point. WebKit2GTK 4.1 and GTK 3 come from the host system (not bundled).
HERE="$(dirname "$(readlink -f "$0")")"
exec "$HERE/usr/bin/tradeloom" "$@"
```

`scripts/make-appimage.sh`:

```bash
#!/usr/bin/env bash
# Builds dist/Tradeloom-<VERSION>-x86_64_noorders.AppImage from build/bin/tradeloom (ARCHITECTURE §10.3).
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:?set VERSION}"
APPDIR=build/appimage/AppDir
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin" dist
install -m 0755 build/bin/tradeloom "$APPDIR/usr/bin/tradeloom"
install -m 0755 build/appimage/AppRun "$APPDIR/AppRun"
install -m 0644 build/linux/tradeloom.desktop "$APPDIR/tradeloom.desktop"
install -m 0644 build/appicon.png "$APPDIR/tradeloom.png"
ARCH=x86_64 "${APPIMAGETOOL:-appimagetool}" "$APPDIR" "dist/Tradeloom-${VERSION}-x86_64_noorders.AppImage"
```

`build/Dockerfile.linux`:

```dockerfile
# Linux build container (ARCHITECTURE §10.2). ubuntu:22.04 keeps the binary's glibc floor at 2.35.
# Build:  docker build -f build/Dockerfile.linux -t tradeloom-build:p0 build
# Use:    make docker-release VERSION=0.1.0
FROM ubuntu:22.04

ARG GO_VERSION=1.25.14
ARG NODE_VERSION=22.23.3
ARG WAILS_VERSION=v2.16.0
ARG NFPM_VERSION=2.47.0
ARG GO_LICENSES_VERSION=v1.6.0

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev \
      ca-certificates curl git xz-utils file desktop-file-utils make \
 && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
RUN curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz" \
      | tar -C /usr/local --strip-components=1 -xJ
RUN curl -fsSL -o /tmp/nfpm.deb "https://github.com/goreleaser/nfpm/releases/download/v${NFPM_VERSION}/nfpm_${NFPM_VERSION}_amd64.deb" \
 && dpkg -i /tmp/nfpm.deb && rm /tmp/nfpm.deb
RUN curl -fsSL -o /usr/local/bin/appimagetool \
      "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage" \
 && chmod +x /usr/local/bin/appimagetool

ENV GOPATH=/go \
    PATH=/usr/local/go/bin:/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    GOTOOLCHAIN=local \
    GOWORK=off \
    APPIMAGE_EXTRACT_AND_RUN=1
RUN go install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}" \
 && go install "github.com/google/go-licenses@${GO_LICENSES_VERSION}"
RUN git config --system --add safe.directory /src
WORKDIR /src
```

`scripts/install-smoke.sh`:

```bash
#!/usr/bin/env bash
# S0-3 install matrix (ARCHITECTURE §10.3): install the package built by CI on this distro and run
# `tradeloom --smoke` under Xvfb. Runs as root inside a throwaway distro container.
set -euo pipefail
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
```

Run (Linux host with Docker, or skip to Step 6 and let CI run it): `chmod +x build/appimage/AppRun scripts/*.sh && make docker-release VERSION=0.0.1 && ls dist && dpkg-deb -I dist/tradeloom_0.0.1_amd64_noorders.deb | grep Depends && dpkg-deb -c dist/tradeloom_0.0.1_amd64_noorders.deb`
Expected: `dist/` holds `SHA256SUMS`, `Tradeloom-0.0.1-x86_64_noorders.AppImage`, `tradeloom_0.0.1_amd64_noorders.deb`; the log contains `g0: ok`; `Depends: libgtk-3-0t64 | libgtk-3-0, libwebkit2gtk-4.1-0`; the data listing is `./usr/bin/tradeloom`, `./usr/share/applications/tradeloom.desktop`, `./usr/share/doc/tradeloom/THIRD_PARTY_NOTICES.md`, `./usr/share/pixmaps/tradeloom.png`.

- [ ] **Step 6: CI and release workflows**

`.github/workflows/ci.yml`:

```yaml
# ARCHITECTURE §10.4 jobs for P0-1: go-unit (incl. archtest, redaction, G-0 stub), ta, arch-lint,
# frontend, licence, linux-package (Docker build + G-0 nm check + .deb + AppImage), install-smoke (S0-3).
name: ci

on:
  push:
  pull_request:
  workflow_call:
    inputs:
      version:
        type: string
        required: true

permissions:
  contents: read

env:
  GO_VERSION: "1.25.14"
  NODE_VERSION: "22.23.3"
  GOWORK: "off"
  GOTOOLCHAIN: local
  TAGS: webkit2_41,noorders

jobs:
  go-unit:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: go.sum
      - name: GTK/WebKit headers (cmd/tradeloom compiles with cgo)
        run: sudo apt-get update && sudo apt-get install -y --no-install-recommends libgtk-3-dev libwebkit2gtk-4.1-dev
      - run: go vet -tags "$TAGS" ./...
      - name: go test (archtest, redaction, journal, backup grep, G-0 stub)
        run: go test -race -count=1 -tags "$TAGS" ./...
      - name: G-0 — noorders wins when both tags are set
        run: go test -count=1 -tags webkit2_41,orders,noorders ./internal/kite/orderwrite/...

  ta:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: ta/go.sum
      - run: cd ta && go vet ./... && go test -count=1 ./...

  arch-lint:
    runs-on: ubuntu-24.04
    env:
      CGO_ENABLED: "0"
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: go.sum
      - run: go install github.com/fe3dback/go-arch-lint@v1.19.0
      - run: go-arch-lint check

  frontend:
    runs-on: ubuntu-24.04
    defaults:
      run:
        working-directory: frontend
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-node@v7
        with:
          node-version: ${{ env.NODE_VERSION }}
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - run: npm ci
      - run: npm run typecheck
      - run: npm test
      - run: npm run build

  licence:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: go.sum
      - uses: actions/setup-node@v7
        with:
          node-version: ${{ env.NODE_VERSION }}
      - run: go install github.com/google/go-licenses@v1.6.0
      - run: cd frontend && npm ci
      - run: make licenses
      - name: THIRD_PARTY_NOTICES.md is current
        run: make notices && git diff --exit-code THIRD_PARTY_NOTICES.md

  linux-package:
    needs: [go-unit, arch-lint, frontend, licence]
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7
      - name: Build in the ubuntu:22.04 container (wails build, G-0 nm check, .deb, AppImage)
        run: make docker-release VERSION="${{ inputs.version || format('0.0.0+ci.{0}', github.run_number) }}"
      - uses: actions/upload-artifact@v7
        with:
          name: linux-packages
          path: dist/*
          if-no-files-found: error

  install-smoke:
    needs: linux-package
    runs-on: ubuntu-24.04
    strategy:
      fail-fast: false
      matrix:
        include:
          - { image: "ubuntu:22.04", kind: deb }
          - { image: "ubuntu:24.04", kind: deb }
          - { image: "debian:12", kind: deb }
          - { image: "fedora:39", kind: appimage }
    container: ${{ matrix.image }}
    steps:
      - uses: actions/checkout@v7
      - uses: actions/download-artifact@v7
        with:
          name: linux-packages
          path: dist
      - run: bash scripts/install-smoke.sh ${{ matrix.kind }}
```

`.github/workflows/release.yml`:

```yaml
# ARCHITECTURE §10.4 release.yml: on a version tag, run the whole CI pipeline (including the
# install matrix) for that version and publish the noorders packages with SHA-256 sums.
name: release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  ci:
    uses: ./.github/workflows/ci.yml
    with:
      version: ${{ github.ref_name }}

  publish:
    needs: ci
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/download-artifact@v7
        with:
          name: linux-packages
          path: dist
      - name: GitHub release
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          gh release create "${{ github.ref_name }}" dist/* \
            --repo "${{ github.repository }}" \
            --title "Tradeloom ${{ github.ref_name }} (noorders)" \
            --notes "P0-P3 build. Order placement is not compiled in (gate G-0)."
```

Run: `python3 -c 'import sys,yaml; [yaml.safe_load(open(f)) for f in sys.argv[1:]]; print("yaml ok")' .github/workflows/ci.yml .github/workflows/release.yml .go-arch-lint.yml build/nfpm.yaml`
Expected: `yaml ok` (needs PyYAML; any YAML parser will do).

- [ ] **Step 7: Commit, push and watch CI**

```bash
git add .go-arch-lint.yml Makefile scripts build/notices.tpl build/nfpm.yaml build/linux build/appimage/AppRun \
  build/Dockerfile.linux .github THIRD_PARTY_NOTICES.md
git commit -m "build: go-arch-lint, G-0 nm check, licence notices, Docker Linux build, .deb/AppImage and CI" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git push -u origin HEAD
gh run watch --exit-status
```
Expected: all `ci` jobs green, including the four `install-smoke` legs (each ends with `smoke ok: <distro> (deb|appimage)`). A red `install-smoke` leg is not fixed here — it is the input to Task 17.

---
