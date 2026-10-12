> **P0-1 Platform Core Implementation Plan — Task 1.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 1: Repository skeleton and Wails hello window

**Files:**
- Create: `go.mod`, `go.work`, `Makefile`, `cmd/tradeloom/main.go`, `cmd/tradeloom/wails.json`, `internal/app/app.go`, `internal/app/app_test.go`, `frontend/package.json`, `frontend/tsconfig.json`, `frontend/vite.config.ts`, `frontend/index.html`, `frontend/src/main.tsx`, `frontend/src/App.tsx`, `frontend/src/App.test.tsx`, `frontend/public/.gitkeep`, `frontend/dist/.gitkeep`, `frontend/assets.go`, `frontend/assets_test.go`
- Generated and committed: `frontend/package-lock.json`, `frontend/wailsjs/**`, `build/appicon.png`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `package app`: `var Version = "dev"` (set with `-ldflags "-X tradeloom/internal/app.Version=<v>"`); `type App struct{…}`; `func New() *App`; `func (a *App) OnStartup(ctx context.Context)`; `func (a *App) Hello() string`.
  - `package frontend` (import path `tradeloom/frontend`): `func Assets() fs.FS` — contents of `frontend/dist` with `index.html` at the root.
  - Build command: `cd cmd/tradeloom && wails build -m -nosyncgomod -tags "webkit2_41,noorders"` → `build/bin/tradeloom` (Linux) / `build/bin/tradeloom.app` (macOS).

- [ ] **Step 1: Create the module, workspace and ignore rules**

```bash
cd /path/to/streak-desktop        # repo root; every later command runs from here unless it cd's
export GOTOOLCHAIN=local
cat > go.mod <<'EOF'
module tradeloom

go 1.25.0

ignore ./frontend/node_modules

replace tradeloom/ta => ./ta
EOF
cat > go.work <<'EOF'
go 1.25.0

use (
	.
	./ta
)
EOF
```

Replace `.gitignore` with:

```gitignore
.DS_Store
*.log
node_modules/
/dist/
build/bin/
build/darwin/
build/windows/
build/appimage/AppDir/
frontend/dist/*
!frontend/dist/.gitkeep
frontend/package.json.md5
go.work.sum
```

(`ignore` is the Go 1.25 go.mod directive that keeps `./...` out of `node_modules`; the `replace` stays even before any package imports `tradeloom/ta` — `go mod tidy` keeps replace directives.)

- [ ] **Step 2: Write the failing Go tests**

`internal/app/app_test.go`:

```go
package app

import (
	"context"
	"testing"
)

func TestHello(t *testing.T) {
	a := New()
	a.OnStartup(context.Background())
	if got := a.Hello(); got != "Tradeloom dev" {
		t.Fatalf("Hello() = %q", got)
	}
}
```

`frontend/assets_test.go`:

```go
package frontend

import (
	"io/fs"
	"testing"
)

func TestAssets_RootIsDist(t *testing.T) {
	if _, err := fs.Stat(Assets(), ".gitkeep"); err != nil {
		t.Fatalf("dist/.gitkeep must be embedded at the FS root: %v", err)
	}
}
```

- [ ] **Step 3: Run them to verify they fail**

Run: `go test -tags noorders ./internal/app/ ./frontend/`
Expected: FAIL — build errors `internal/app/app_test.go:…: undefined: New` and `frontend/assets_test.go:…: undefined: Assets`.

- [ ] **Step 4: Write the Go implementation**

`internal/app/app.go`:

```go
// Package app is the Wails glue (ARCHITECTURE §2.2): lifecycle callbacks, bound API structs
// and the bus → runtime.EventsEmit bridge. P0-1 provides the hello-window App (Task 1) and Boot (Task 15);
// P0-3 adds bindings, the event bridge and the Wails single-instance callback.
package app

import (
	"context"
)

// Version is set at build time: -ldflags "-X tradeloom/internal/app.Version=v0.1.0".
var Version = "dev"

// App is bound to the frontend. Exported methods become TypeScript functions in frontend/wailsjs.
type App struct {
	ctx context.Context
}

// New returns the hello-window App.
func New() *App { return &App{} }

// OnStartup is options.App.OnStartup; Wails passes the runtime context used by runtime.* calls.
func (a *App) OnStartup(ctx context.Context) { a.ctx = ctx }

// Hello returns the window greeting; it proves bindings work end to end.
func (a *App) Hello() string { return "Tradeloom " + Version }
```

`frontend/assets.go`:

```go
// Package frontend embeds the built web UI (frontend/dist) into the binary.
// `npm run build` (run by `wails build`) fills dist/; dist/.gitkeep keeps the directory present
// so `go build ./...` and `go test ./...` work before the first frontend build.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the contents of dist/ (index.html at the root).
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded at compile time; Sub cannot fail for a valid directory name
	}
	return sub
}
```

`cmd/tradeloom/main.go` (hello window; Task 15 replaces it):

```go
// Command tradeloom is the desktop application (the only package besides internal/app that imports Wails).
package main

import (
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"tradeloom/frontend"
	"tradeloom/internal/app"
)

func main() {
	a := app.New()
	err := wails.Run(&options.App{
		Title:       "Tradeloom",
		Width:       1440,
		Height:      900,
		MinWidth:    1280, // FR-SHELL layouts are designed for 1280 px; P0-4 relies on this floor
		MinHeight:   720,
		AssetServer: &assetserver.Options{Assets: frontend.Assets()},
		OnStartup:   a.OnStartup,
		Bind:        []interface{}{a},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "tradeloom:", err)
		os.Exit(1)
	}
}
```

`cmd/tradeloom/wails.json`:

```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "tradeloom",
  "outputfilename": "tradeloom",
  "frontend:dir": "../../frontend",
  "wailsjsdir": "../../frontend",
  "build:dir": "../../build",
  "frontend:install": "npm ci",
  "frontend:build": "npm run build",
  "frontend:dev:watcher": "npm run dev",
  "frontend:dev:serverUrl": "auto",
  "author": {
    "name": "Tradeloom"
  }
}
```

Create the embed placeholders and fetch Wails:

```bash
mkdir -p frontend/dist frontend/public
touch frontend/dist/.gitkeep frontend/public/.gitkeep
go get github.com/wailsapp/wails/v2@v2.16.0
go mod tidy
grep -E '^go ' go.mod   # must still print: go 1.25.0
```

- [ ] **Step 5: Run the Go tests to verify they pass**

Run: `go test -tags noorders ./internal/app/ ./frontend/`
Expected: `ok  tradeloom/internal/app` and `ok  tradeloom/frontend`.

- [ ] **Step 6: Write the frontend scaffold and its failing test**

`frontend/package.json`:

```json
{
  "name": "tradeloom-frontend",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "engines": {
    "node": ">=22"
  },
  "scripts": {
    "dev": "vite",
    "build": "tsc --noEmit && vite build",
    "typecheck": "tsc --noEmit",
    "test": "vitest run",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "19.3.0",
    "react-dom": "19.3.0"
  },
  "devDependencies": {
    "@testing-library/dom": "10.4.2",
    "@testing-library/react": "16.3.3",
    "@types/react": "19.3.0",
    "@types/react-dom": "19.3.0",
    "@vitejs/plugin-react": "6.1.2",
    "jsdom": "30.1.2",
    "typescript": "5.9.3",
    "vite": "8.3.4",
    "vitest": "5.0.3"
  }
}
```

`frontend/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["DOM", "DOM.Iterable", "ES2022"],
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "isolatedModules": true,
    "resolveJsonModule": true,
    "skipLibCheck": true,
    "noEmit": true,
    "types": ["vitest/globals"]
  },
  "include": ["src", "vite.config.ts"]
}
```

`frontend/vite.config.ts` (the `public/.gitkeep` file is copied into `dist/` on every build, so `dist/.gitkeep` survives `emptyOutDir`):

```ts
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Wails serves frontend/dist from the embedded FS (frontend/assets.go).
export default defineConfig({
  plugins: [react()],
  build: { outDir: 'dist', emptyOutDir: true },
  test: { environment: 'jsdom', globals: true },
})
```

`frontend/index.html`:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Tradeloom</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

`frontend/src/main.tsx`:

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

`frontend/src/App.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { App } from './App'

test('renders the product name', () => {
  render(<App />)
  expect(screen.getByRole('heading', { name: 'Tradeloom' })).toBeTruthy()
})
```

Run: `cd frontend && npm install --no-audit --no-fund && npx vitest run; cd ..`
Expected: FAIL — `Failed to resolve import "./App"`.

- [ ] **Step 7: Write `frontend/src/App.tsx` and verify the frontend**

```tsx
// P0-1 hello window. P0-4 replaces this with the application shell.
export function App() {
  return (
    <main>
      <h1>Tradeloom</h1>
      <p>Foundations build — no order placement.</p>
    </main>
  )
}
```

Run: `cd frontend && npx vitest run && npm run build && ls -a dist; cd ..`
Expected: `Tests  1 passed (1)`; build prints `dist/index.html` and `dist/assets/index-*.js`; `ls` shows `.gitkeep`, `assets`, `index.html`. Then `git status --short frontend/dist` prints nothing (only `.gitkeep` is tracked and it is unchanged).

- [ ] **Step 8: Write the initial Makefile**

```make
# Tradeloom developer entry points (Task 16 writes the full version). P0–P3 builds carry noorders (G-0).
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
```

- [ ] **Step 9: Build and open the hello window**

Run: `make test && make build`
Expected: every package `ok` (or `[no test files]` for `cmd/tradeloom`); Wails prints `Generating bindings: Done`, `Compiling frontend: Done`, `Compiling application: Done` and `Built '…/build/bin/tradeloom…'`. Wails also writes `frontend/wailsjs/go/app/App.d.ts` (containing `export function Hello():Promise<string>;`), `frontend/wailsjs/runtime/*` and `build/appicon.png`.
Then launch `build/bin/tradeloom` (Linux) or `open build/bin/tradeloom.app` (macOS): a 1280×800 window titled "Tradeloom" shows the heading "Tradeloom". Close it.

- [ ] **Step 10: Commit**

```bash
git add go.mod go.sum go.work .gitignore Makefile cmd/tradeloom internal/app frontend/package.json frontend/package-lock.json \
  frontend/tsconfig.json frontend/vite.config.ts frontend/index.html frontend/src frontend/public/.gitkeep \
  frontend/dist/.gitkeep frontend/assets.go frontend/assets_test.go frontend/wailsjs build/appicon.png
git commit -m "feat: repo skeleton with Wails hello window, Vite React TS frontend and Makefile" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
