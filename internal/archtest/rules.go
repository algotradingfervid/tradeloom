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

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func underAny(path string, ps []string) bool {
	for _, p := range ps {
		if under(path, p) {
			return true
		}
	}
	return false
}

// importRule: importers of any path under targets (or equal to one, when exact) must be under one of
// allowed (module-relative). An empty allowed list bans the import everywhere in the module.
type importRule struct {
	name    string
	targets []string
	allowed []string // module-relative package prefixes
	exact   bool     // targets match only themselves, not their sub-packages ("net" but not "net/url")
}

var ioPackages = []string{"database/sql", "modernc.org/sqlite", "net/http", "github.com/zerodha/gokiteconnect", "github.com/wailsapp/wails"}

// pureEngine are the rule-1 packages: no I/O imports at all.
var pureEngine = []string{"internal/model", "internal/eval", "internal/indicators", "internal/backtest", "internal/calendar", "internal/clock"}

// bypassForbidden are the engine packages internal/app and internal/telegram must reach only through internal/service (rule 7).
var bypassForbidden = []string{"internal/model", "internal/indicators", "internal/eval", "internal/backtest", "internal/scanner",
	"internal/universe", "internal/live", "internal/orders", "internal/candles", "internal/instruments", "internal/kite"}

var importRules = []importRule{
	{"rule2", []string{"github.com/wailsapp/wails"}, []string{"internal/app", "cmd/tradeloom"}, false},
	{"rule3", []string{"github.com/zerodha/gokiteconnect"}, []string{"internal/kite"}, false},
	{"rule4", []string{"database/sql", "modernc.org/sqlite"}, []string{"internal/store", "internal/candles/sqlite"}, false},
	{"rule5", []string{"github.com/go-telegram/bot"}, []string{"internal/telegram"}, false},
	{"rule6", []string{"github.com/zalando/go-keyring", "golang.org/x/crypto"}, []string{"internal/secrets"}, false},
	{"g0-import", []string{"internal/kite/orderwrite"}, []string{"internal/orders", "internal/kite/orderwrite"}, false},
	// g0-http (Ruling 69): only packages under internal/kite may speak HTTP, so a hand-rolled order
	// POST cannot be written elsewhere (the nm check sees only gokiteconnect's write methods). It
	// covers net/http's sub-packages (httptest, httputil, …) too. Test files are exempt: Load does
	// not load them (packages.Config.Tests is false), and they are not linked into the binary.
	// golang.org/x/net/... (http2, websocket, proxy, …) is an HTTP stack too (Ruling 79 I-2).
	{"g0-http", []string{"net/http", "golang.org/x/net"}, []string{"internal/kite"}, false},
	// g0-net (Ruling 79 I-2): raw sockets and TLS only under internal/kite; unsafe nowhere (it can reach
	// unexported gokiteconnect state); os/exec only under internal/app (the browser opener), so no other
	// package can shell out to curl. Exact targets: net/url, net/netip and crypto/tlsx are not affected.
	{"g0-net", []string{"net", "crypto/tls"}, []string{"internal/kite"}, true},
	{"g0-net", []string{"unsafe"}, nil, true},
	{"g0-net", []string{"os/exec"}, []string{"internal/app"}, true},
	// g0-mock (Ruling 79 M-3): the recording fake is for tests only; non-test code may not import it.
	{"g0-mock", []string{"internal/kite/mock"}, []string{"internal/kite/mock"}, false},
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
				hit := underAny(target, r.targets)
				if r.exact {
					hit = containsStr(r.targets, target)
				}
				if hit && !underAny(self, r.allowed) {
					where := "allowed only in " + strings.Join(r.allowed, ", ")
					if len(r.allowed) == 0 {
						where = "not allowed in this module"
					}
					out = append(out, Violation{r.name, p.Path, "imports " + imp + " (" + where + ")"})
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
