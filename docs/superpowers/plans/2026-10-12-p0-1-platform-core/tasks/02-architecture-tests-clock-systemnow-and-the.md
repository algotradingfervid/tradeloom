> **P0-1 Platform Core Implementation Plan — Task 2.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 2: Architecture tests, `clock.SystemNow` and the G-0 order-write stub

**Files:**
- Create: `internal/clock/system.go`, `internal/kite/orderwrite/doc.go`, `internal/kite/orderwrite/iface.go`, `internal/kite/orderwrite/writer_stub.go`, `internal/kite/orderwrite/writer_stub_test.go`, `internal/archtest/rules.go`, `internal/archtest/ast.go`, `internal/archtest/rules_test.go`, `internal/archtest/ast_test.go`, `internal/archtest/arch_test.go`, `internal/archtest/testdata/fixture/go.mod`, `…/fixture/fakekite/go.mod`, `…/fixture/fakekite/client.go`, `…/fixture/internal/bad/bad.go`, `…/fixture/internal/kite/orderwrite/writer.go`, `…/fixture/internal/kite/orderwrite/untagged.go`, `…/fixture/internal/kite/reads/reads.go`, `…/fixture/internal/clock/clock.go`, `…/fixture/internal/timeuser/timeuser.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: module layout from Task 1.
- Produces:
  - `package clock`: `func SystemNow() time.Time` (P0-2 keeps this function when it adds `clock.Clock`).
  - `package orderwrite`: `var ErrOrdersNotCompiled = errors.New("Order placement is not included in this build")`; `type Writer interface { PlaceOrder(ctx context.Context, variety string, p kiteconnect.OrderParams) (kiteconnect.OrderResponse, error); ModifyOrder(ctx context.Context, variety, orderID string, p kiteconnect.OrderParams) (kiteconnect.OrderResponse, error); CancelOrder(ctx context.Context, variety, orderID string, parentOrderID *string) (kiteconnect.OrderResponse, error); ExitOrder(ctx context.Context, variety, orderID string, parentOrderID *string) (kiteconnect.OrderResponse, error); ConvertPosition(ctx context.Context, p kiteconnect.ConvertPositionParams) (bool, error); PlaceGTT(ctx context.Context, p kiteconnect.GTTParams) (kiteconnect.GTTResponse, error); ModifyGTT(ctx context.Context, triggerID int, p kiteconnect.GTTParams) (kiteconnect.GTTResponse, error); DeleteGTT(ctx context.Context, triggerID int) (kiteconnect.GTTResponse, error) }`; stub build only: `func New(_ *kiteconnect.Client) Writer`, `func Compiled() bool` (P4's `writer.go` must define the same two functions under `//go:build orders && !noorders`).
  - `package archtest`: `type Pkg struct{ Path string; Imports []string }`; `type Violation struct{ Rule, Pkg, Detail string }`; `func CheckImports(module string, pkgs []Pkg) []Violation`; `func Load(dir string, env []string, tags string, patterns ...string) ([]*packages.Package, error)`; `func Graph(pkgs []*packages.Package) []Pkg`; `func CheckClock(module string, pkgs []*packages.Package) []Violation`; `func CheckOrderWrite(module string, pkgs []*packages.Package) []Violation`.
  - Rule predicates later sub-plans must satisfy (exactly as coded in `rules.go`): rule 7 forbids `internal/app` and `internal/telegram` from importing `internal/{model,indicators,eval,backtest,scanner,universe,live,orders,candles,instruments,kite}` (and sub-packages) — anything the UI needs from those reaches it through `internal/service`; only `internal/orders/...` may import `internal/kite/orderwrite`.

- [ ] **Step 1: Write the fixture module (the AST checks' test data)**

`internal/archtest/testdata/fixture/go.mod`:

```gomod
module fixture

go 1.25

require github.com/zerodha/gokiteconnect/v4 v4.0.0

replace github.com/zerodha/gokiteconnect/v4 => ./fakekite
```

`internal/archtest/testdata/fixture/fakekite/go.mod`:

```gomod
module github.com/zerodha/gokiteconnect/v4

go 1.25
```

`internal/archtest/testdata/fixture/fakekite/client.go`:

```go
// Package kiteconnect is a minimal stand-in for gokiteconnect used only by archtest fixtures.
package kiteconnect

type Client struct{}
type OrderParams struct{}

func (c *Client) PlaceOrder(variety string, p OrderParams) (string, error)      { return "", nil }
func (c *Client) ModifyOrder(variety, id string, p OrderParams) (string, error) { return "", nil }
func (c *Client) GetOrders() ([]string, error)                                  { return nil, nil }
```

`internal/archtest/testdata/fixture/internal/bad/bad.go`:

```go
package bad

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Place is a G-0 violation: order-write call outside internal/kite/orderwrite.
func Place(c *kiteconnect.Client) { c.PlaceOrder("regular", kiteconnect.OrderParams{}) }

type notKite struct{}

func (notKite) PlaceOrder() {}

// Harmless calls a same-named method on another type: not a violation.
func Harmless() { notKite{}.PlaceOrder() }
```

`internal/archtest/testdata/fixture/internal/kite/orderwrite/writer.go`:

```go
//go:build orders && !noorders

package orderwrite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Place is allowed: tagged file in internal/kite/orderwrite.
func Place(c *kiteconnect.Client) { c.PlaceOrder("regular", kiteconnect.OrderParams{}) }
```

`internal/archtest/testdata/fixture/internal/kite/orderwrite/untagged.go`:

```go
package orderwrite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Modify is a G-0 violation: right package, but compiled without the orders tag.
func Modify(c *kiteconnect.Client) { c.ModifyOrder("regular", "1", kiteconnect.OrderParams{}) }
```

`internal/archtest/testdata/fixture/internal/kite/reads/reads.go`:

```go
package reads

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Orders is allowed: read-only call.
func Orders(c *kiteconnect.Client) { c.GetOrders() }
```

`internal/archtest/testdata/fixture/internal/clock/clock.go`:

```go
package clock

import "time"

// Now is allowed: internal/clock owns the wall clock.
func Now() time.Time { return time.Now() }
```

`internal/archtest/testdata/fixture/internal/timeuser/timeuser.go`:

```go
package timeuser

import (
	"time"
	tm "time"
)

// Wait has two rule-8 violations (one through an import alias) and one allowed use.
func Wait() time.Duration {
	_ = time.Now()
	tm.Sleep(time.Millisecond)
	return time.Duration(5) * time.Second
}
```

- [ ] **Step 2: Write the failing tests**

`internal/archtest/rules_test.go`:

```go
package archtest

import (
	"strings"
	"testing"
)

func TestCheckImports_Table(t *testing.T) {
	cases := []struct {
		name     string
		pkg      Pkg
		wantRule string // "" means no violation
	}{
		{"eval imports sqlite", Pkg{"tradeloom/internal/eval", []string{"modernc.org/sqlite"}}, "rule1"},
		{"clock imports net/http", Pkg{"tradeloom/internal/clock", []string{"net/http"}}, "rule1"},
		{"calendar imports time", Pkg{"tradeloom/internal/calendar", []string{"time"}}, ""},
		{"service imports wails", Pkg{"tradeloom/internal/service", []string{"github.com/wailsapp/wails/v2/pkg/runtime"}}, "rule2"},
		{"app imports wails", Pkg{"tradeloom/internal/app", []string{"github.com/wailsapp/wails/v2/pkg/runtime"}}, ""},
		{"cmd imports wails", Pkg{"tradeloom/cmd/tradeloom", []string{"github.com/wailsapp/wails/v2"}}, ""},
		{"candles imports kite lib", Pkg{"tradeloom/internal/candles", []string{"github.com/zerodha/gokiteconnect/v4"}}, "rule3"},
		{"kite/ticker imports kite lib", Pkg{"tradeloom/internal/kite/ticker", []string{"github.com/zerodha/gokiteconnect/v4/ticker"}}, ""},
		{"settings imports database/sql", Pkg{"tradeloom/internal/settings", []string{"database/sql"}}, "rule4"},
		{"candles/sqlite imports sqlite", Pkg{"tradeloom/internal/candles/sqlite", []string{"modernc.org/sqlite"}}, ""},
		{"candles/sqlitex is not candles/sqlite", Pkg{"tradeloom/internal/candles/sqlitex", []string{"database/sql"}}, "rule4"},
		{"notify imports telegram lib", Pkg{"tradeloom/internal/notify", []string{"github.com/go-telegram/bot"}}, "rule5"},
		{"store imports keyring", Pkg{"tradeloom/internal/store", []string{"github.com/zalando/go-keyring"}}, "rule6"},
		{"logging imports x/crypto", Pkg{"tradeloom/internal/logging", []string{"golang.org/x/crypto/argon2"}}, "rule6"},
		{"secrets imports x/crypto", Pkg{"tradeloom/internal/secrets", []string{"golang.org/x/crypto/chacha20poly1305"}}, ""},
		{"app bypasses service", Pkg{"tradeloom/internal/app", []string{"tradeloom/internal/candles"}}, "rule7"},
		{"telegram bypasses service", Pkg{"tradeloom/internal/telegram", []string{"tradeloom/internal/live"}}, "rule7"},
		{"app uses service", Pkg{"tradeloom/internal/app", []string{"tradeloom/internal/service", "tradeloom/internal/store"}}, ""},
		{"service imports orderwrite", Pkg{"tradeloom/internal/service", []string{"tradeloom/internal/kite/orderwrite"}}, "g0-import"},
		{"orders imports orderwrite", Pkg{"tradeloom/internal/orders", []string{"tradeloom/internal/kite/orderwrite"}}, ""},
		{"external package ignored", Pkg{"github.com/x/y", []string{"database/sql"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := CheckImports("tradeloom", []Pkg{tc.pkg})
			switch {
			case tc.wantRule == "" && len(v) != 0:
				t.Fatalf("unexpected violations %v", v)
			case tc.wantRule != "" && (len(v) == 0 || v[0].Rule != tc.wantRule):
				t.Fatalf("want %s, got %v", tc.wantRule, v)
			}
		})
	}
}

func TestUnder(t *testing.T) {
	if !under("internal/kite/ticker", "internal/kite") || under("internal/kitex", "internal/kite") || !under("a", "a") {
		t.Fatal("under")
	}
	if !strings.HasPrefix(Violation{"r", "p", "d"}.String(), "r: p") {
		t.Fatal("String")
	}
}
```

`internal/archtest/ast_test.go` (the fixture is loaded with `GOWORK=off` because it is not part of `go.work`, and `GOPROXY=off` because its only dependency is a local `replace`):

```go
package archtest

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, tags string) []string {
	t.Helper()
	dir, _ := filepath.Abs(filepath.Join("testdata", "fixture"))
	env := append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
	pkgs, err := Load(dir, env, tags, "./...")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, v := range append(CheckOrderWrite("fixture", pkgs), CheckClock("fixture", pkgs)...) {
		got = append(got, v.Rule+" "+strings.SplitN(v.Pkg, ":", 2)[0])
	}
	sort.Strings(got)
	return got
}

func TestASTChecks_Fixture(t *testing.T) {
	cases := []struct {
		tags string
		want []string
	}{
		{"noorders", []string{"g0-call bad/bad.go", "g0-call orderwrite/untagged.go", "rule8 timeuser/timeuser.go", "rule8 timeuser/timeuser.go"}},
		// With the orders tag the tagged writer.go is loaded and allowed; violations are unchanged.
		{"orders", []string{"g0-call bad/bad.go", "g0-call orderwrite/untagged.go", "rule8 timeuser/timeuser.go", "rule8 timeuser/timeuser.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.tags, func(t *testing.T) {
			got := loadFixture(t, tc.tags)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("\n got %v\nwant %v", got, tc.want)
			}
		})
	}
}
```

`internal/archtest/arch_test.go`:

```go
package archtest

import (
	"os"
	"path/filepath"
	"testing"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if b, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && len(b) > 0 && string(b[:len("module tradeloom\n")]) == "module tradeloom\n" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root not found")
		}
		dir = parent
	}
}

// The real module obeys rules 1–8 and G-0 in both build configurations.
func TestArchitecture(t *testing.T) {
	if testing.Short() {
		t.Skip("loads and type-checks the whole module")
	}
	root := moduleRoot(t)
	for _, tags := range []string{"webkit2_41,noorders", "webkit2_41,orders"} {
		t.Run(tags, func(t *testing.T) {
			pkgs, err := Load(root, nil, tags, "./...")
			if err != nil {
				t.Fatal(err)
			}
			vs := CheckImports("tradeloom", Graph(pkgs))
			vs = append(vs, CheckClock("tradeloom", pkgs)...)
			vs = append(vs, CheckOrderWrite("tradeloom", pkgs)...)
			for _, v := range vs {
				t.Error(v)
			}
		})
	}
}
```

`internal/kite/orderwrite/writer_stub_test.go`:

```go
//go:build !orders || noorders

package orderwrite

import (
	"context"
	"errors"
	"testing"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

func TestStubRefusesEveryWrite(t *testing.T) {
	w := New(kiteconnect.New("not-a-real-key"))
	ctx := context.Background()
	calls := map[string]func() error{
		"PlaceOrder":      func() error { _, err := w.PlaceOrder(ctx, "regular", kiteconnect.OrderParams{}); return err },
		"ModifyOrder":     func() error { _, err := w.ModifyOrder(ctx, "regular", "1", kiteconnect.OrderParams{}); return err },
		"CancelOrder":     func() error { _, err := w.CancelOrder(ctx, "regular", "1", nil); return err },
		"ExitOrder":       func() error { _, err := w.ExitOrder(ctx, "regular", "1", nil); return err },
		"ConvertPosition": func() error { _, err := w.ConvertPosition(ctx, kiteconnect.ConvertPositionParams{}); return err },
		"PlaceGTT":        func() error { _, err := w.PlaceGTT(ctx, kiteconnect.GTTParams{}); return err },
		"ModifyGTT":       func() error { _, err := w.ModifyGTT(ctx, 1, kiteconnect.GTTParams{}); return err },
		"DeleteGTT":       func() error { _, err := w.DeleteGTT(ctx, 1); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, ErrOrdersNotCompiled) {
			t.Errorf("%s: want ErrOrdersNotCompiled, got %v", name, err)
		}
	}
	if Compiled() {
		t.Fatal("stub build reports Compiled() == true")
	}
}
```

- [ ] **Step 3: Run them to verify they fail**

Run: `go test -tags noorders ./internal/archtest/ ./internal/kite/orderwrite/`
Expected: FAIL — archtest: build error `undefined: Pkg`; orderwrite: `no required module provides package github.com/zerodha/gokiteconnect/v4`.

- [ ] **Step 4: Write the implementation**

```bash
go get golang.org/x/tools@v0.49.0 github.com/zerodha/gokiteconnect/v4@v4.4.3
```

`internal/clock/system.go`:

```go
// Package clock is the only package allowed to read the wall clock (ARCHITECTURE §2.4 rule 8).
// P0-1 provides SystemNow; P0-2 adds the offset-corrected Clock interface, timers and the estimator.
package clock

import "time"

// SystemNow returns the operating-system wall clock. Packages receive it as a
// func() time.Time value instead of calling time.Now themselves.
func SystemNow() time.Time { return time.Now() }
```

`internal/kite/orderwrite/doc.go`:

```go
// Package orderwrite is the only package allowed to call Kite order-write endpoints
// (PlaceOrder, ModifyOrder, CancelOrder, ExitOrder, ConvertPosition, PlaceGTT, ModifyGTT,
// DeleteGTT) — ARCHITECTURE §2.5, ROADMAP §6 gate G-0.
//
// Files:
//   - iface.go        (no build tag) the Writer interface and ErrOrdersNotCompiled.
//   - writer_stub.go  //go:build !orders || noorders — every method returns ErrOrdersNotCompiled.
//   - writer.go       //go:build orders && !noorders — the real implementation; added in P4.
//
// P0–P3 builds pass -tags webkit2_41,noorders. Building with neither tag, or with both, yields the
// stub. CI enforces the gate three ways: internal/archtest (AST: order-write calls on
// *kiteconnect.Client only in tagged files of this package; only internal/orders may import this
// package), `go tool nm` on the noorders binary (scripts/g0-nm-check.sh), and the mock-Kite
// recording suite (P0-3).
package orderwrite
```

`internal/kite/orderwrite/iface.go`:

```go
package orderwrite

import (
	"context"
	"errors"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// ErrOrdersNotCompiled is returned by every Writer method in builds without the orders tag.
// The UI shows its text verbatim.
var ErrOrdersNotCompiled = errors.New("Order placement is not included in this build")

// Writer is the Kite order-write surface. Parameter and result types are gokiteconnect v4's.
type Writer interface {
	PlaceOrder(ctx context.Context, variety string, p kiteconnect.OrderParams) (kiteconnect.OrderResponse, error)
	ModifyOrder(ctx context.Context, variety, orderID string, p kiteconnect.OrderParams) (kiteconnect.OrderResponse, error)
	CancelOrder(ctx context.Context, variety, orderID string, parentOrderID *string) (kiteconnect.OrderResponse, error)
	ExitOrder(ctx context.Context, variety, orderID string, parentOrderID *string) (kiteconnect.OrderResponse, error)
	ConvertPosition(ctx context.Context, p kiteconnect.ConvertPositionParams) (bool, error)
	PlaceGTT(ctx context.Context, p kiteconnect.GTTParams) (kiteconnect.GTTResponse, error)
	ModifyGTT(ctx context.Context, triggerID int, p kiteconnect.GTTParams) (kiteconnect.GTTResponse, error)
	DeleteGTT(ctx context.Context, triggerID int) (kiteconnect.GTTResponse, error)
}
```

`internal/kite/orderwrite/writer_stub.go`:

```go
//go:build !orders || noorders

package orderwrite

import (
	"context"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// Compiled reports whether this binary contains the real order-write implementation.
func Compiled() bool { return false }

type stub struct{}

// New returns the stub Writer; the client is ignored so no order-write method is linked.
func New(_ *kiteconnect.Client) Writer { return stub{} }

func (stub) PlaceOrder(context.Context, string, kiteconnect.OrderParams) (kiteconnect.OrderResponse, error) {
	return kiteconnect.OrderResponse{}, ErrOrdersNotCompiled
}

func (stub) ModifyOrder(context.Context, string, string, kiteconnect.OrderParams) (kiteconnect.OrderResponse, error) {
	return kiteconnect.OrderResponse{}, ErrOrdersNotCompiled
}

func (stub) CancelOrder(context.Context, string, string, *string) (kiteconnect.OrderResponse, error) {
	return kiteconnect.OrderResponse{}, ErrOrdersNotCompiled
}

func (stub) ExitOrder(context.Context, string, string, *string) (kiteconnect.OrderResponse, error) {
	return kiteconnect.OrderResponse{}, ErrOrdersNotCompiled
}

func (stub) ConvertPosition(context.Context, kiteconnect.ConvertPositionParams) (bool, error) {
	return false, ErrOrdersNotCompiled
}

func (stub) PlaceGTT(context.Context, kiteconnect.GTTParams) (kiteconnect.GTTResponse, error) {
	return kiteconnect.GTTResponse{}, ErrOrdersNotCompiled
}

func (stub) ModifyGTT(context.Context, int, kiteconnect.GTTParams) (kiteconnect.GTTResponse, error) {
	return kiteconnect.GTTResponse{}, ErrOrdersNotCompiled
}

func (stub) DeleteGTT(context.Context, int) (kiteconnect.GTTResponse, error) {
	return kiteconnect.GTTResponse{}, ErrOrdersNotCompiled
}
```

`internal/archtest/rules.go`:

```go
// Package archtest enforces ARCHITECTURE §2.4 rules 1–8 and the G-0 order-write gate (§2.5)
// as ordinary Go tests, so a developer without go-arch-lint installed still fails locally.
package archtest

import (
	"fmt"
	"sort"
	"strings"
)

// Pkg is one package of the import graph (direct imports only).
type Pkg struct {
	Path    string
	Imports []string
}

// Violation is one broken rule.
type Violation struct {
	Rule   string // "rule1" … "rule8", "g0-import", "g0-call"
	Pkg    string // package import path (or file for AST rules)
	Detail string
}

func (v Violation) String() string { return fmt.Sprintf("%s: %s: %s", v.Rule, v.Pkg, v.Detail) }

// under reports whether path is p or below p ("a/b" is under "a", "ab" is not).
func under(path, p string) bool { return path == p || strings.HasPrefix(path, p+"/") }

func underAny(path string, ps []string) bool {
	for _, p := range ps {
		if under(path, p) {
			return true
		}
	}
	return false
}

// importRule: importers of any path under Targets must be under one of Allowed (module-relative).
type importRule struct {
	name    string
	targets []string
	allowed []string // module-relative package prefixes
}

var ioPackages = []string{"database/sql", "modernc.org/sqlite", "net/http", "github.com/zerodha/gokiteconnect", "github.com/wailsapp/wails"}

// pureEngine are the rule-1 packages: no I/O imports at all.
var pureEngine = []string{"internal/model", "internal/eval", "internal/indicators", "internal/backtest", "internal/calendar", "internal/clock"}

// bypassForbidden are the engine packages internal/app and internal/telegram must reach only through internal/service (rule 7).
var bypassForbidden = []string{"internal/model", "internal/indicators", "internal/eval", "internal/backtest", "internal/scanner",
	"internal/universe", "internal/live", "internal/orders", "internal/candles", "internal/instruments", "internal/kite"}

var importRules = []importRule{
	{"rule2", []string{"github.com/wailsapp/wails"}, []string{"internal/app", "cmd/tradeloom"}},
	{"rule3", []string{"github.com/zerodha/gokiteconnect"}, []string{"internal/kite"}},
	{"rule4", []string{"database/sql", "modernc.org/sqlite"}, []string{"internal/store", "internal/candles/sqlite"}},
	{"rule5", []string{"github.com/go-telegram/bot"}, []string{"internal/telegram"}},
	{"rule6", []string{"github.com/zalando/go-keyring", "golang.org/x/crypto"}, []string{"internal/secrets"}},
	{"g0-import", []string{"internal/kite/orderwrite"}, []string{"internal/orders", "internal/kite/orderwrite"}},
}

// CheckImports applies rules 1–7 and the G-0 import rule to pkgs. module is the module path ("tradeloom").
func CheckImports(module string, pkgs []Pkg) []Violation {
	rel := func(p string) (string, bool) {
		if p == module {
			return "", true
		}
		if strings.HasPrefix(p, module+"/") {
			return strings.TrimPrefix(p, module+"/"), true
		}
		return "", false
	}
	var out []Violation
	for _, p := range pkgs {
		self, ok := rel(p.Path)
		if !ok {
			continue
		}
		for _, imp := range p.Imports {
			impRel, internal := rel(imp)
			target := imp
			if internal {
				target = impRel
			}
			if underAny(self, pureEngine) && underAny(imp, ioPackages) {
				out = append(out, Violation{"rule1", p.Path, "imports I/O package " + imp})
			}
			for _, r := range importRules {
				if underAny(target, r.targets) && !underAny(self, r.allowed) {
					out = append(out, Violation{r.name, p.Path, "imports " + imp + " (allowed only in " + strings.Join(r.allowed, ", ") + ")"})
				}
			}
			if internal && (under(self, "internal/app") || under(self, "internal/telegram")) && underAny(impRel, bypassForbidden) {
				out = append(out, Violation{"rule7", p.Path, "imports engine package " + imp + " directly; go through internal/service"})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}
```

`internal/archtest/ast.go`:

```go
package archtest

import (
	"go/ast"
	"go/build/constraint"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// bannedTime are the time functions that read the wall clock or start timers (rule 8).
var bannedTime = map[string]bool{"Now": true, "Since": true, "Until": true, "Sleep": true, "After": true,
	"AfterFunc": true, "Tick": true, "NewTimer": true, "NewTicker": true}

// orderWriteMethods are the Kite order-write methods gated by G-0.
var orderWriteMethods = map[string]bool{"PlaceOrder": true, "ModifyOrder": true, "CancelOrder": true, "ExitOrder": true,
	"ConvertPosition": true, "PlaceGTT": true, "ModifyGTT": true, "DeleteGTT": true}

const kiteClient = "github.com/zerodha/gokiteconnect/v4.Client"

// Load type-checks every package matching patterns in dir with the given build tags (non-test files only).
func Load(dir string, env []string, tags string, patterns ...string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule,
		Dir:        dir,
		Env:        env,
		BuildFlags: []string{"-tags=" + tags},
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	var errs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			errs = append(errs, e.Error())
		}
	})
	if len(errs) > 0 {
		return nil, &loadError{errs}
	}
	return pkgs, nil
}

type loadError struct{ errs []string }

func (e *loadError) Error() string { return "archtest: load: " + strings.Join(e.errs, "; ") }

// Graph converts loaded packages to the import graph used by CheckImports.
func Graph(pkgs []*packages.Package) []Pkg {
	out := make([]Pkg, 0, len(pkgs))
	for _, p := range pkgs {
		g := Pkg{Path: p.PkgPath}
		for imp := range p.Imports {
			g.Imports = append(g.Imports, imp)
		}
		out = append(out, g)
	}
	return out
}

func relPkg(module, path string) string {
	return strings.TrimPrefix(strings.TrimPrefix(path, module), "/")
}

// CheckClock reports calls to banned time functions outside internal/clock (rule 8).
func CheckClock(module string, pkgs []*packages.Package) []Violation {
	var out []Violation
	for _, p := range pkgs {
		if under(relPkg(module, p.PkgPath), "internal/clock") {
			continue
		}
		for _, f := range p.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || !bannedTime[sel.Sel.Name] {
					return true
				}
				id, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				if pn, ok := p.TypesInfo.Uses[id].(*types.PkgName); ok && pn.Imported().Path() == "time" {
					out = append(out, Violation{"rule8", position(p.Fset, sel.Pos()), "time." + sel.Sel.Name + " outside internal/clock; use clock.SystemNow / clock.Clock"})
				}
				return true
			})
		}
	}
	return out
}

// CheckOrderWrite reports order-write calls on *kiteconnect.Client outside files of
// internal/kite/orderwrite whose build constraint requires the orders tag (G-0 check 1).
func CheckOrderWrite(module string, pkgs []*packages.Package) []Violation {
	var out []Violation
	for _, p := range pkgs {
		inPkg := relPkg(module, p.PkgPath) == "internal/kite/orderwrite"
		for _, f := range p.Syntax {
			tagged := requiresOrdersTag(f)
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !orderWriteMethods[sel.Sel.Name] {
					return true
				}
				s, ok := p.TypesInfo.Selections[sel]
				if !ok || !isKiteClient(s.Recv()) {
					return true
				}
				if !(inPkg && tagged) {
					out = append(out, Violation{"g0-call", position(p.Fset, call.Pos()),
						"(*kiteconnect.Client)." + sel.Sel.Name + " is only allowed in internal/kite/orderwrite files built with the orders tag"})
				}
				return true
			})
		}
	}
	return out
}

func isKiteClient(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path()+"."+named.Obj().Name() == kiteClient
}

// requiresOrdersTag is true when the file's //go:build line is satisfied with "orders" but not without it.
func requiresOrdersTag(f *ast.File) bool {
	for _, cg := range f.Comments {
		if cg.Pos() > f.Package {
			break
		}
		for _, c := range cg.List {
			if !constraint.IsGoBuild(c.Text) {
				continue
			}
			expr, err := constraint.Parse(c.Text)
			if err != nil {
				return false
			}
			with := expr.Eval(func(tag string) bool { return tag == "orders" })
			without := expr.Eval(func(tag string) bool { return false })
			return with && !without
		}
	}
	return false
}

func position(fset *token.FileSet, pos token.Pos) string {
	pp := fset.Position(pos)
	return filepath.ToSlash(filepath.Base(filepath.Dir(pp.Filename))) + "/" + filepath.Base(pp.Filename) + ":" + strconv.Itoa(pp.Line)
}
```

Then: `go mod tidy && grep -E '^go ' go.mod` → `go 1.25.0`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -count=1 -tags noorders ./internal/archtest/ ./internal/kite/orderwrite/ && go test -count=1 -tags orders,noorders ./internal/kite/orderwrite/ && go vet -tags orders ./internal/kite/orderwrite/`
Expected: `ok` for each package; vet prints nothing (the `orders`-only build compiles `iface.go` alone).

- [ ] **Step 6: Prove the real-module check bites, then revert**

```bash
printf 'package app\n\nimport "time"\n\nvar _ = time.Now\n' > internal/app/zz_probe.go
go test -count=1 -tags noorders ./internal/archtest/ 2>&1 | grep rule8
rm internal/app/zz_probe.go
```
Expected: `rule8: app/zz_probe.go:5: time.Now outside internal/clock; use clock.SystemNow / clock.Clock` (twice, once per build configuration).

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/clock internal/kite/orderwrite internal/archtest
git commit -m "feat(archtest): enforce dependency rules 1-8 and the G-0 order-write gate; add orderwrite stub" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
