package archtest

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// bannedTime are the time functions that read the wall clock or start timers (rule 8).
var bannedTime = map[string]bool{"Now": true, "Since": true, "Until": true, "Sleep": true, "After": true,
	"AfterFunc": true, "Tick": true, "NewTimer": true, "NewTicker": true}

// orderWriteMethods are the Kite order-write methods gated by G-0: regular/GTT orders, alerts (an
// alert can carry an ATO order basket) and mutual-fund orders/SIPs (Ruling 47) and
// InitiateHoldingsAuth (CDSL authorisation that enables selling holdings, Ruling 48). Keep this list in
// sync with scripts/g0-nm-check.sh.
var orderWriteMethods = map[string]bool{"PlaceOrder": true, "ModifyOrder": true, "CancelOrder": true, "ExitOrder": true,
	"ConvertPosition": true, "PlaceGTT": true, "ModifyGTT": true, "DeleteGTT": true,
	"CreateAlert": true, "ModifyAlert": true, "DeleteAlerts": true,
	"PlaceMFOrder": true, "CancelMFOrder": true, "PlaceMFSIP": true, "ModifyMFSIP": true, "CancelMFSIP": true,
	"InitiateHoldingsAuth": true}

// kitePkg matches every major version of gokiteconnect, so a module bump cannot silently disable G-0.
var kitePkg = regexp.MustCompile(`^github\.com/zerodha/gokiteconnect/v[0-9]+$`)

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
	return kitePkg.MatchString(named.Obj().Pkg().Path()) && named.Obj().Name() == "Client"
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

// CheckKiteIndirect reports G-0 bypasses that reach *kiteconnect.Client without naming it
// (Ruling 39). CheckOrderWrite resolves a selector to its declared method, so a generic
// function constrained by an interface with CancelOrder, instantiated by inference with
// *kiteconnect.Client and handed to kite.(*Client).Call, slipped past it. Four rules:
//
//   - g0-kitecall: any reference (call, method value, method expression, promoted) to
//     kite.(*Client).Call outside packages under internal/kite.
//   - g0-generic: a selector naming an order-write method whose receiver is a type parameter or
//     interface, outside internal/kite/orderwrite, unless the method is declared there
//     (orderwrite.Writer, which the build tags gate).
//   - g0-iface: an interface type outside internal/kite/orderwrite declaring an order-write method.
//   - g0-typearg: outside internal/kite, a generic instantiation whose type arguments mention
//     kiteconnect.Client. Rule 3 forbids naming it there, so only inference can produce one.
//
// Residual: reflection (reflect.ValueOf(c).MethodByName, reflect.MakeFunc) is not visible to
// the AST; the `go tool nm` check on the noorders binary is the backstop.
func CheckKiteIndirect(module string, pkgs []*packages.Package) []Violation {
	var out []Violation
	for _, p := range pkgs {
		rel := relPkg(module, p.PkgPath)
		inKite := under(rel, "internal/kite")
		inOrderWrite := rel == "internal/kite/orderwrite"
		for _, f := range p.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.SelectorExpr:
					s, ok := p.TypesInfo.Selections[n]
					if !ok {
						return true
					}
					fn, ok := s.Obj().(*types.Func)
					if !ok {
						return true
					}
					if !inKite && fn.Name() == "Call" && isNamed(recvType(fn), module+"/internal/kite", "Client") {
						out = append(out, Violation{"g0-kitecall", position(p.Fset, n.Pos()),
							"kite.(*Client).Call is only allowed in packages under internal/kite"})
					}
					if !inOrderWrite && orderWriteMethods[fn.Name()] && isAbstract(s.Recv()) &&
						(fn.Pkg() == nil || fn.Pkg().Path() != module+"/internal/kite/orderwrite") {
						out = append(out, Violation{"g0-generic", position(p.Fset, n.Pos()),
							n.Sel.Name + " selected through a type parameter or interface outside internal/kite/orderwrite"})
					}
				case *ast.InterfaceType:
					if inOrderWrite || n.Methods == nil {
						return true
					}
					for _, m := range n.Methods.List {
						for _, name := range m.Names {
							if orderWriteMethods[name.Name] {
								out = append(out, Violation{"g0-iface", position(p.Fset, name.Pos()),
									"interface declares order-write method " + name.Name + " outside internal/kite/orderwrite"})
							}
						}
					}
				}
				return true
			})
		}
		if inKite {
			continue
		}
		for id, inst := range p.TypesInfo.Instances {
			for i := 0; i < inst.TypeArgs.Len(); i++ {
				if mentionsKiteClient(inst.TypeArgs.At(i), map[types.Type]bool{}) {
					out = append(out, Violation{"g0-typearg", position(p.Fset, id.Pos()),
						id.Name + " instantiated with a type argument mentioning *kiteconnect.Client outside internal/kite"})
					break
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func recvType(fn *types.Func) types.Type {
	if r := fn.Type().(*types.Signature).Recv(); r != nil {
		return r.Type()
	}
	return nil
}

func isNamed(t types.Type, pkg, name string) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == pkg && named.Obj().Name() == name
}

// isAbstract reports whether a selection's receiver is a type parameter or an interface.
func isAbstract(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if _, ok := t.(*types.TypeParam); ok {
		return true
	}
	return types.IsInterface(t)
}

// mentionsKiteClient reports whether t is kiteconnect.Client or is built from it by type
// constructors (pointer, func, slice, …) or type arguments.
func mentionsKiteClient(t types.Type, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	switch t := t.(type) {
	case *types.Named:
		if isKiteClient(t) {
			return true
		}
		for i := 0; i < t.TypeArgs().Len(); i++ {
			if mentionsKiteClient(t.TypeArgs().At(i), seen) {
				return true
			}
		}
		// Not the underlying type: kite.Client wraps a *kiteconnect.Client field and may be
		// named outside internal/kite (atomic.Pointer[kite.Client] is fine).
		return false
	case *types.Alias:
		return mentionsKiteClient(types.Unalias(t), seen)
	case *types.Pointer:
		return mentionsKiteClient(t.Elem(), seen)
	case *types.Slice:
		return mentionsKiteClient(t.Elem(), seen)
	case *types.Array:
		return mentionsKiteClient(t.Elem(), seen)
	case *types.Chan:
		return mentionsKiteClient(t.Elem(), seen)
	case *types.Map:
		return mentionsKiteClient(t.Key(), seen) || mentionsKiteClient(t.Elem(), seen)
	case *types.Signature:
		return mentionsKiteClient(t.Params(), seen) || mentionsKiteClient(t.Results(), seen)
	case *types.Tuple:
		for i := 0; i < t.Len(); i++ {
			if mentionsKiteClient(t.At(i).Type(), seen) {
				return true
			}
		}
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			if mentionsKiteClient(t.Field(i).Type(), seen) {
				return true
			}
		}
	case *types.Interface:
		for i := 0; i < t.NumMethods(); i++ {
			if mentionsKiteClient(t.Method(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}

// kiteRawWriteNames are the gokiteconnect identifiers that send a request without one of its
// order-write methods (Ruling 69): the raw HTTP client (NewHTTPClient, the HTTPClient type and its
// methods) and the write endpoint paths. A write sent through them links no (*Client).PlaceOrder etc.,
// so the nm check cannot see it. The names are verified against gokiteconnect v4.4.3 (connect.go,
// alerts.go, http.go); TestKiteRawWrite_NamesResolve fails if a module bump drops one.
var kiteRawWriteNames = []string{"NewHTTPClient", "HTTPClient",
	"URIPlaceOrder", "URIModifyOrder", "URICancelOrder", "URIConvertPosition", "URIInitHoldingsAuth",
	"URIPlaceGTT", "URIModifyGTT", "URIDeleteGTT", "URIAlerts", "URIAlert",
	"URIPlaceMFOrder", "URICancelMFOrder", "URIPlaceMFSIP", "URIModifyMFSIP", "URICancelMFSIP"}

// kitePackages returns every loaded gokiteconnect package (any major version), deduplicated.
func kitePackages(pkgs []*packages.Package) []*types.Package {
	seen := map[*types.Package]bool{}
	var out []*types.Package
	for _, p := range pkgs {
		if p.Types == nil {
			continue
		}
		cands := append([]*types.Package{p.Types}, p.Types.Imports()...)
		for _, tp := range cands {
			if !seen[tp] && kitePkg.MatchString(tp.Path()) {
				seen[tp] = true
				out = append(out, tp)
			}
		}
	}
	return out
}

// kiteRawWriteObjects resolves kiteRawWriteNames to their types.Objects in the loaded gokiteconnect
// packages, adding every method of the HTTPClient interface and the same-named methods of each
// package type that implements it (the unexported *httpClient that NewHTTPClient returns).
func kiteRawWriteObjects(pkgs []*packages.Package) map[types.Object]string {
	banned := map[types.Object]string{}
	for _, kp := range kitePackages(pkgs) {
		scope := kp.Scope()
		for _, name := range kiteRawWriteNames {
			if obj := scope.Lookup(name); obj != nil {
				banned[obj] = name
			}
		}
		tn, ok := scope.Lookup("HTTPClient").(*types.TypeName)
		if !ok {
			continue
		}
		iface, ok := tn.Type().Underlying().(*types.Interface)
		if !ok {
			continue
		}
		methods := map[string]bool{}
		for i := 0; i < iface.NumMethods(); i++ {
			m := iface.Method(i)
			banned[m] = "HTTPClient." + m.Name()
			methods[m.Name()] = true
		}
		for _, n := range scope.Names() {
			impl, ok := scope.Lookup(n).(*types.TypeName)
			if !ok || impl == tn {
				continue
			}
			named, ok := impl.Type().(*types.Named)
			if !ok || types.IsInterface(named) || !types.Implements(types.NewPointer(named), iface) {
				continue
			}
			for i := 0; i < named.NumMethods(); i++ {
				if m := named.Method(i); methods[m.Name()] {
					banned[m] = "(*" + impl.Name() + ")." + m.Name()
				}
			}
		}
	}
	return banned
}

// KiteRawWriteMissing lists the kiteRawWriteNames that no loaded gokiteconnect package declares.
func KiteRawWriteMissing(pkgs []*packages.Package) []string {
	found := map[string]bool{}
	for _, name := range kiteRawWriteObjects(pkgs) {
		found[name] = true
	}
	var out []string
	for _, n := range kiteRawWriteNames {
		if !found[n] {
			out = append(out, n)
		}
	}
	if len(kitePackages(pkgs)) == 0 {
		out = append(out, "(gokiteconnect not loaded)")
	}
	return out
}

// CheckKiteRawWrite reports every reference to a kiteRawWriteObjects identifier outside files of
// internal/kite/orderwrite whose build constraint requires the orders tag (g0-kitehttp, Ruling 69;
// the same file rule as g0-call). It matches types.Object identity through TypesInfo.Uses, so qualified,
// renamed-import, dot-import, method-value, promoted and alias-declaration references are all caught;
// a later use of a local alias resolves to the alias, but declaring the alias is itself flagged.
// Test files are not loaded and are exempt. Residual: a hand-written path string ("/orders/regular")
// sent through net/http under internal/kite is not visible here; g0-http confines net/http to
// internal/kite and the mock-Kite recording suite is the dynamic backstop.
func CheckKiteRawWrite(module string, pkgs []*packages.Package) []Violation {
	banned := kiteRawWriteObjects(pkgs)
	if len(banned) == 0 {
		return nil
	}
	var out []Violation
	for _, p := range pkgs {
		inPkg := relPkg(module, p.PkgPath) == "internal/kite/orderwrite"
		allowedFile := map[*token.File]bool{}
		for _, f := range p.Syntax {
			if inPkg && requiresOrdersTag(f) {
				allowedFile[p.Fset.File(f.Pos())] = true
			}
		}
		for id, obj := range p.TypesInfo.Uses {
			name, ok := banned[obj]
			if !ok || allowedFile[p.Fset.File(id.Pos())] {
				continue
			}
			out = append(out, Violation{"g0-kitehttp", position(p.Fset, id.Pos()),
				"kiteconnect." + name + " is only allowed in internal/kite/orderwrite files built with the orders tag"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// writePathPrefixes are the Kite REST paths that change account state (Ruling 69 addendum, g0-path):
// orders, GTT, alerts, mutual funds, position conversion and holdings authorisation. /orders and
// /portfolio/positions also prefix read endpoints; the rule bans those too outside the mock, because
// production code reaches Kite only through gokiteconnect's read methods. The entries omit the leading
// "/" (writePathPrefix adds it) so this list does not trip g0-path itself without exempting archtest.
var writePathPrefixes = []string{"orders", "gtt", "alerts", "mf/", "portfolio/positions", "portfolio/holdings/authorise"}

// kitePath trims leading whitespace and a host ("https://api.kite.trade/x" or "api.kite.trade/x")
// from s, leaving the path.
func kitePath(s string) string {
	s = strings.TrimLeft(s, " \t\r\n")
	if i := strings.Index(s, "://"); i > 0 && !strings.ContainsAny(s[:i], "/ \t") {
		s = s[i+3:]
		if j := strings.IndexByte(s, '/'); j >= 0 {
			return s[j:]
		}
		return ""
	}
	if !strings.HasPrefix(s, "/") {
		if j := strings.IndexByte(s, '/'); j > 0 && strings.ContainsAny(s[:j], ".:") && !strings.ContainsAny(s[:j], " \t") {
			return s[j:]
		}
	}
	return s
}

// writePathPrefix returns the write-path prefix s starts with (case-folded), or "".
func writePathPrefix(s string) string {
	p := strings.ToLower(kitePath(s))
	if !strings.HasPrefix(p, "/") {
		return ""
	}
	for _, w := range writePathPrefixes {
		if strings.HasPrefix(p[1:], w) {
			return p[:1] + w
		}
	}
	return ""
}

// CheckKitePaths reports every string constant expression whose folded value is a Kite write path
// (g0-path, Ruling 69 addendum). It reads constant values from TypesInfo.Types, so a write path built by
// concatenating constants ("/" + seg) is caught like a literal; an expression nested in a reported one is
// not reported again. Exempt: internal/kite/mock (the fake Kite), orders-tagged files of
// internal/kite/orderwrite, packages under a testdata directory, and test files (not loaded).
// Residual: a path assembled at run time (fmt.Sprintf, strings.Join, byte slices) is not a constant;
// g0-http confines net/http to internal/kite and the mock-Kite recording suite is the dynamic backstop.
func CheckKitePaths(module string, pkgs []*packages.Package) []Violation {
	type hit struct {
		pos, end token.Pos
		val, pre string
	}
	var out []Violation
	for _, p := range pkgs {
		rel := relPkg(module, p.PkgPath)
		if under(rel, "internal/kite/mock") || rel == "testdata" || strings.HasPrefix(rel, "testdata/") ||
			strings.Contains(rel, "/testdata/") || strings.HasSuffix(rel, "/testdata") {
			continue
		}
		allowedFile := map[*token.File]bool{}
		if rel == "internal/kite/orderwrite" {
			for _, f := range p.Syntax {
				if requiresOrdersTag(f) {
					allowedFile[p.Fset.File(f.Pos())] = true
				}
			}
		}
		var hits []hit
		for e, tv := range p.TypesInfo.Types {
			if tv.Value == nil || tv.Value.Kind() != constant.String || allowedFile[p.Fset.File(e.Pos())] {
				continue
			}
			v := constant.StringVal(tv.Value)
			if pre := writePathPrefix(v); pre != "" {
				hits = append(hits, hit{e.Pos(), e.End(), v, pre})
			}
		}
		sort.Slice(hits, func(i, j int) bool {
			if hits[i].pos != hits[j].pos {
				return hits[i].pos < hits[j].pos
			}
			return hits[i].end > hits[j].end
		})
		var outerEnd token.Pos
		for _, h := range hits {
			if h.pos < outerEnd { // nested in the last reported expression
				continue
			}
			outerEnd = h.end
			val := h.val
			if len(val) > 60 {
				val = val[:60] + "…"
			}
			out = append(out, Violation{"g0-path", position(p.Fset, h.pos),
				fmt.Sprintf("string constant %q is a Kite write path (%s…); allowed only in internal/kite/mock and orders-tagged internal/kite/orderwrite files", val, h.pre)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}
