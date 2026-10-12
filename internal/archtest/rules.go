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
	Rule   string // "rule1" … "rule8", "g0-import", "g0-http", "g0-call", "g0-kitehttp", …
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
	// g0-http (Ruling 69): only packages under internal/kite may speak HTTP, so a hand-rolled order
	// POST cannot be written elsewhere (the nm check sees only gokiteconnect's write methods). It
	// covers net/http's sub-packages (httptest, httputil, …) too. Test files are exempt: Load does
	// not load them (packages.Config.Tests is false), and they are not linked into the binary.
	{"g0-http", []string{"net/http"}, []string{"internal/kite"}},
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
