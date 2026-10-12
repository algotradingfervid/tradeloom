# Tradeloom developer and CI entry points. P0–P3 builds always carry the noorders tag (G-0).
export GOTOOLCHAIN=local

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
