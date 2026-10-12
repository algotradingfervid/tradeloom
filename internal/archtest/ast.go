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
			for _, imp := range f.Imports {
				if imp.Name != nil && imp.Name.Name == "." && imp.Path.Value == `"time"` {
					out = append(out, Violation{"rule8", position(p.Fset, imp.Pos()), "dot-import of time outside internal/clock"})
				}
			}
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
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || !orderWriteMethods[sel.Sel.Name] {
					return true
				}
				// Any selector (call, method value, method expression, promoted via embedding) whose
				// resolved method is declared on kiteconnect.Client.
				s, ok := p.TypesInfo.Selections[sel]
				if !ok {
					return true
				}
				fn, ok := s.Obj().(*types.Func)
				if !ok || !orderWriteMethods[fn.Name()] {
					return true
				}
				recv := fn.Type().(*types.Signature).Recv()
				if recv == nil || !isKiteClient(recv.Type()) {
					return true
				}
				if !(inPkg && tagged) {
					out = append(out, Violation{"g0-call", position(p.Fset, sel.Pos()),
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
			// Must link only with orders and never when noorders is also set.
			both := expr.Eval(func(tag string) bool { return tag == "orders" || tag == "noorders" })
			return with && !without && !both
		}
	}
	return false
}

func position(fset *token.FileSet, pos token.Pos) string {
	pp := fset.Position(pos)
	return filepath.ToSlash(filepath.Base(filepath.Dir(pp.Filename))) + "/" + filepath.Base(pp.Filename) + ":" + strconv.Itoa(pp.Line)
}
