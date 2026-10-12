# Tradeloom developer entry points (Task 16 writes the full version). P0–P3 builds carry noorders (G-0).
export GOTOOLCHAIN=local

GO      ?= go
TAGS    ?= webkit2_41,noorders
VERSION ?= 0.0.0+$(shell git rev-parse --short HEAD 2>/dev/null || echo local)
LDFLAGS := -X tradeloom/internal/app.Version=$(VERSION)

.PHONY: test test-race frontend-test build

test:
	$(GO) test -count=1 -tags $(TAGS) ./...

test-race:
	$(GO) test -race -count=1 -tags $(TAGS) ./...

frontend-test:
	cd frontend && npm ci && npm run typecheck && npm test

build:
	cd cmd/tradeloom && wails build -m -nosyncgomod -tags "$(TAGS)" -ldflags "$(LDFLAGS)"
