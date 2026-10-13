package archtest

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"net/url"
	"os"
	"path"
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

// requiresOrdersTag reports whether the file's //go:build expression links only in orders builds
// (Ruling 79 I-1): it must be false for EVERY assignment of the tags it names in which orders is false
// or noorders is true. Every other tag (go1.x, GOOS, cgo, custom) is a free variable, so a term such as
// "|| go1.1" or "|| linux" that is true without the orders tag disqualifies the file. A file without a
// //go:build line, an unparsable one, or one naming more than maxFreeTags tags is not orders-only.
func requiresOrdersTag(f *ast.File) bool {
	expr := goBuildExpr(f)
	return expr != nil && ordersOnly(expr)
}

// goBuildExpr returns the file's //go:build expression, or nil when it has none or it does not parse.
func goBuildExpr(f *ast.File) constraint.Expr {
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
				return nil
			}
			return expr
		}
	}
	return nil
}

const maxFreeTags = 16

func ordersOnly(expr constraint.Expr) bool {
	return linksOnlyWhen(expr, []string{"orders", "noorders"}, func(val map[string]bool) bool {
		return val["orders"] && !val["noorders"] // an orders build: the file may link here
	})
}

// onlyWithTag reports whether expr is false for every assignment of the tags it names in which tag is
// false (Ruling 99: an allow-listed cgo file links only in builds that set tag).
func onlyWithTag(expr constraint.Expr, tag string) bool {
	return expr != nil && linksOnlyWhen(expr, []string{tag}, func(val map[string]bool) bool { return val[tag] })
}

// linksOnlyWhen reports whether expr is false for every assignment of fixed and the tags expr names for
// which ok is false. Every other tag is a free variable; more than maxFreeTags tags is not decided (false).
func linksOnlyWhen(expr constraint.Expr, fixed []string, ok func(map[string]bool) bool) bool {
	seen := map[string]bool{}
	tags := []string{}
	for _, tag := range fixed {
		seen[tag] = true
		tags = append(tags, tag)
	}
	var walk func(constraint.Expr)
	walk = func(e constraint.Expr) {
		switch e := e.(type) {
		case *constraint.TagExpr:
			if !seen[e.Tag] {
				seen[e.Tag] = true
				tags = append(tags, e.Tag)
			}
		case *constraint.NotExpr:
			walk(e.X)
		case *constraint.AndExpr:
			walk(e.X)
			walk(e.Y)
		case *constraint.OrExpr:
			walk(e.X)
			walk(e.Y)
		}
	}
	walk(expr)
	if len(tags) > maxFreeTags {
		return false
	}
	for mask := 0; mask < 1<<len(tags); mask++ {
		val := map[string]bool{}
		for i, tag := range tags {
			val[tag] = mask&(1<<i) != 0
		}
		if ok(val) {
			continue
		}
		if expr.Eval(func(tag string) bool { return val[tag] }) {
			return false // links in a build where it must not
		}
	}
	return true
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
		for _, impl := range httpClientImpls(kp, tn, iface) {
			// The method set, not only the declared methods: a method promoted from an embedded type is
			// what a selector on the implementation resolves to (Ruling 83).
			ms := types.NewMethodSet(types.NewPointer(impl.Type()))
			for i := 0; i < ms.Len(); i++ {
				// impls are sorted by name and the first label is kept, so a method promoted into several
				// implementations is always labelled with the same one (Ruling 91 M6).
				if m := ms.At(i).Obj(); methods[m.Name()] {
					if _, seen := banned[m]; !seen {
						banned[m] = "(*" + impl.Name() + ")." + m.Name()
					}
				}
			}
		}
	}
	return banned
}

// httpClientImpls returns the non-interface types of kp whose pointer implements iface (the HTTPClient
// interface tn): gokiteconnect's unexported *httpClient, whatever it is called. Sorted by name
// (Scope.Names is sorted).
func httpClientImpls(kp *types.Package, tn *types.TypeName, iface *types.Interface) []*types.TypeName {
	var out []*types.TypeName
	scope := kp.Scope()
	for _, n := range scope.Names() {
		impl, ok := scope.Lookup(n).(*types.TypeName)
		if !ok || impl == tn || impl.IsAlias() {
			continue
		}
		named, ok := impl.Type().(*types.Named)
		if !ok || types.IsInterface(named) || !types.Implements(types.NewPointer(named), iface) {
			continue
		}
		out = append(out, impl)
	}
	return out
}

// KiteRawWriteMissing lists the kiteRawWriteNames that no loaded gokiteconnect package declares.
func KiteRawWriteMissing(pkgs []*packages.Package) []string {
	banned := kiteRawWriteObjects(pkgs)
	found := map[string]bool{}
	for _, name := range banned {
		found[name] = true
	}
	var out []string
	for _, n := range kiteRawWriteNames {
		if !found[n] {
			out = append(out, n)
		}
	}
	// Ruling 79 M-5: every HTTPClient method must also have resolved on an implementing type
	// (the unexported *httpClient), or a method value taken from GetClient() would go unreported.
	// Ruling 83: the entry names the type that really implements HTTPClient, or says that none does.
	for _, kp := range kitePackages(pkgs) {
		tn, ok := kp.Scope().Lookup("HTTPClient").(*types.TypeName)
		if !ok {
			continue
		}
		iface, ok := tn.Type().Underlying().(*types.Interface)
		if !ok {
			continue
		}
		impls := httpClientImpls(kp, tn, iface)
		for i := 0; i < iface.NumMethods(); i++ {
			name := iface.Method(i).Name()
			if len(impls) == 0 {
				out = append(out, "HTTPClient."+name+" (no type implements HTTPClient)")
				continue
			}
			for _, impl := range impls {
				// By object, not label: a promoted method shared by several implementations has one label.
				obj, _, _ := types.LookupFieldOrMethod(types.NewPointer(impl.Type()), true, kp, name)
				if _, ok := banned[obj]; obj == nil || !ok {
					out = append(out, "(*"+impl.Name()+")."+name)
				}
			}
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
// "/" (writePathPrefix adds it) and are joined from segments at run time, so this list does not trip
// g0-path itself (a relative path with a "/" is matched too, M1) without exempting archtest.
var writePathPrefixes = joinSegments([][]string{{"orders"}, {"gtt"}, {"alerts"}, {"mf", ""},
	{"portfolio", "positions"}, {"portfolio", "holdings", "authorise"}})

func joinSegments(segs [][]string) []string {
	out := make([]string, 0, len(segs))
	for _, s := range segs {
		out = append(out, strings.Join(s, "/"))
	}
	return out
}

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

// normKitePath returns the path s names as the server would route it (M1), or "" when s is not a path:
// percent-decoded until stable (as netguard's requestPaths), host and leading whitespace trimmed
// (kitePath), given a leading "/" when it is relative but contains a "/" ("orders/regular"; a bare word
// such as "orders" or "GetOrders" is not a path), cleaned of dot segments and repeated slashes
// (path.Clean), and lower-cased. A string that does not decode is matched undecoded.
func normKitePath(s string) string {
	for i := 0; i < 8; i++ {
		u, err := url.PathUnescape(s)
		if err != nil || u == s {
			break
		}
		s = u
	}
	s = kitePath(s)
	if !strings.HasPrefix(s, "/") {
		if !strings.Contains(s, "/") {
			return ""
		}
		s = "/" + s
	}
	return strings.ToLower(path.Clean(s))
}

// writePathPrefix returns the write-path prefix the normalised s starts with (normKitePath), or "".
func writePathPrefix(s string) string {
	p := normKitePath(s)
	if p == "" {
		return ""
	}
	for _, w := range writePathPrefixes {
		// A prefix ending in "/" also matches its bare form: "/mf" is "/mf/" without the slash (Ruling 79 M-1).
		if strings.HasPrefix(p[1:], w) || (strings.HasSuffix(w, "/") && p[1:] == strings.TrimSuffix(w, "/")) {
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

// execFuncs are the process-starting functions outside os/exec that g0-exec bans (Ruling 83), by package
// path. golang.org/x/sys/unix has Exec (v0.47.0 declares no ForkExec; it is listed in case a version adds
// it). Every name in os and syscall must resolve (ExecNamesMissing); the x/sys names may be absent.
var execFuncs = map[string][]string{
	"os":                    {"StartProcess"},
	"syscall":               {"Exec", "ForkExec", "StartProcess"},
	"golang.org/x/sys/unix": {"Exec", "ForkExec"},
}

// isExecObject reports whether obj is part of os/exec, or one of the execFuncs package-level functions.
func isExecObject(obj types.Object) (string, bool) {
	if obj == nil || obj.Pkg() == nil {
		return "", false
	}
	path := obj.Pkg().Path()
	if path == "os/exec" {
		if _, isPkg := obj.(*types.PkgName); !isPkg {
			return "os/exec." + obj.Name(), true
		}
		return "", false
	}
	fn, ok := obj.(*types.Func)
	if !ok || fn.Type().(*types.Signature).Recv() != nil || !containsStr(execFuncs[path], fn.Name()) {
		return "", false
	}
	return path + "." + fn.Name(), true
}

// CheckExec reports every reference outside internal/app to os/exec (functions, types, fields and
// methods, also when reached through a value another package returns, without importing os/exec) and to
// the execFuncs (g0-exec, Ruling 83): a package that can start a process can run curl against the Kite
// order API, which neither the transport deny-list nor the nm check would see. It matches
// types.Object identity through TypesInfo.Uses, so renamed imports, dot-imports, aliases and function
// values are caught. Test files are not loaded and are exempt. Raw syscalls (syscall.Syscall with
// SYS_EXECVE) are stopped by g0-sys (CheckSysObjects). internal/app must not export os/exec values: their
// use elsewhere is flagged here, but a wrapper that hides os/exec behind internal/app's own types is not
// (Ruling 91 M2).
func CheckExec(module string, pkgs []*packages.Package) []Violation {
	var out []Violation
	for _, p := range pkgs {
		if under(relPkg(module, p.PkgPath), "internal/app") {
			continue
		}
		for id, obj := range p.TypesInfo.Uses {
			if name, ok := isExecObject(obj); ok {
				out = append(out, Violation{"g0-exec", position(p.Fset, id.Pos()),
					name + " starts a process; allowed only in internal/app (the desktop opener)"})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// ExecNamesMissing lists the os and syscall execFuncs, and os/exec itself, that the loaded packages do not
// declare, so a toolchain change cannot silently drop part of g0-exec. pkgs must include os, os/exec and
// syscall (loaded directly or as dependencies).
func ExecNamesMissing(pkgs []*packages.Package) []string {
	byPath := map[string]*types.Package{}
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.Types != nil {
			byPath[p.PkgPath] = p.Types
		}
	})
	var out []string
	if byPath["os/exec"] == nil || byPath["os/exec"].Scope().Lookup("Command") == nil {
		out = append(out, "os/exec.Command")
	}
	for _, path := range []string{"os", "syscall"} {
		for _, name := range execFuncs[path] {
			tp := byPath[path]
			if tp == nil {
				out = append(out, path+"."+name)
				continue
			}
			if fn, ok := tp.Scope().Lookup(name).(*types.Func); !ok {
				out = append(out, path+"."+name)
			} else if _, ok := isExecObject(fn); !ok {
				out = append(out, path+"."+name+" (not matched)")
			}
		}
	}
	return out
}

// isSysPath reports whether path is syscall or a golang.org/x/sys package.
func isSysPath(path string) bool { return under(path, "syscall") || under(path, "golang.org/x/sys") }

// CheckSysObjects reports every use of a syscall or golang.org/x/sys object (function, constant, type,
// field, method) in a module package, unless the package is in sysImporters and the object is on its
// list (g0-sys, Ruling 91 M1a). It blocks raw Syscall/RawSyscall (and SYS_EXECVE) in the allow-listed
// importers, and syscall objects reached without importing syscall anywhere else. The listed names are
// resolved to package-level objects and matched by identity (Ruling 99), so a field or method that
// shares a listed name (a struct field Exec, a method Exec) is not allowed with it.
func CheckSysObjects(module string, pkgs []*packages.Package) []Violation {
	return checkSysObjects(module, pkgs, sysImporters)
}

func checkSysObjects(module string, pkgs []*packages.Package, allow map[string]sysAllowance) []Violation {
	var out []Violation
	for _, p := range pkgs {
		allowed := map[types.Object]bool{}
		names := allow[relPkg(module, p.PkgPath)].objects
		var byPath map[string]*types.Package
		if len(names) != 0 {
			byPath = typesByPath([]*packages.Package{p}) // resolved in p's own type universe
		}
		for _, name := range names {
			if obj := lookupPkgObject(byPath, name); obj != nil {
				allowed[obj] = true
			}
		}
		for id, obj := range p.TypesInfo.Uses {
			if _, isPkg := obj.(*types.PkgName); isPkg || obj.Pkg() == nil || !isSysPath(obj.Pkg().Path()) {
				continue
			}
			if !allowed[obj] {
				out = append(out, Violation{"g0-sys", position(p.Fset, id.Pos()),
					obj.Pkg().Path() + "." + obj.Name() + " is not on this package's syscall / golang.org/x/sys allow-list (sysImporters)"})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// typesByPath indexes every types.Package reachable from pkgs (loaded packages and, through
// types.Package.Imports, their dependencies) by path. One load shares one types.Package per path, so an
// object looked up here is identical to the one TypesInfo.Uses records.
func typesByPath(pkgs []*packages.Package) map[string]*types.Package {
	out := map[string]*types.Package{}
	var add func(tp *types.Package)
	add = func(tp *types.Package) {
		if tp == nil || out[tp.Path()] != nil {
			return
		}
		out[tp.Path()] = tp
		for _, imp := range tp.Imports() {
			add(imp)
		}
	}
	packages.Visit(pkgs, nil, func(p *packages.Package) { add(p.Types) })
	return out
}

// lookupPkgObject resolves "pkg/path.Name" to the package-level object Name of pkg/path, or nil.
func lookupPkgObject(byPath map[string]*types.Package, name string) types.Object {
	i := strings.LastIndex(name, ".")
	if i <= 0 {
		return nil
	}
	tp := byPath[name[:i]]
	if tp == nil {
		return nil
	}
	return tp.Scope().Lookup(name[i+1:])
}

// SysAllowMissing lists the allow entries ("pkg: path.Name") that do not resolve to a package-level object
// in the loaded packages, so a typo or a toolchain change cannot leave an allowance that matches nothing.
// pkgs (one load) must include, directly or as dependencies, every package the entries name.
func SysAllowMissing(pkgs []*packages.Package, allow map[string]sysAllowance) []string {
	byPath := typesByPath(pkgs)
	var out []string
	for pkg, a := range allow {
		for _, name := range a.objects {
			if lookupPkgObject(byPath, name) == nil {
				out = append(out, pkg+": "+name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// kiteConstructFiles are the only files that may build a gokiteconnect client (Ruling 91 M3): both put
// the write deny-list under it (guardWrites).
var kiteConstructFiles = []string{"auth.go", "client.go"}

// kiteConstructObjects resolves kiteconnect.New and (*kiteconnect.Client).SetHTTPClient in every loaded
// gokiteconnect package.
func kiteConstructObjects(pkgs []*packages.Package) map[types.Object]string {
	out := map[types.Object]string{}
	for _, kp := range kitePackages(pkgs) {
		if fn, ok := kp.Scope().Lookup("New").(*types.Func); ok {
			out[fn] = "New"
		}
		if tn, ok := kp.Scope().Lookup("Client").(*types.TypeName); ok {
			if m, _, _ := types.LookupFieldOrMethod(types.NewPointer(tn.Type()), true, kp, "SetHTTPClient"); m != nil {
				out[m] = "(*Client).SetHTTPClient"
			}
		}
	}
	return out
}

// KiteConstructMissing lists the g0-kitenew names that no loaded gokiteconnect package declares.
func KiteConstructMissing(pkgs []*packages.Package) []string {
	found := map[string]bool{}
	for _, n := range kiteConstructObjects(pkgs) {
		found[n] = true
	}
	var out []string
	for _, n := range []string{"New", "(*Client).SetHTTPClient"} {
		if !found[n] {
			out = append(out, n)
		}
	}
	return out
}

// CheckKiteConstruct reports every reference to kiteconnect.New or (*Client).SetHTTPClient outside
// internal/kite/auth.go and internal/kite/client.go (g0-kitenew, Ruling 91 M3): a client built anywhere
// else could carry a transport without the write deny-list. Calls, function and method values,
// renamed and dot-imports are caught (TypesInfo.Uses). Test files are not loaded and are exempt.
func CheckKiteConstruct(module string, pkgs []*packages.Package) []Violation {
	banned := kiteConstructObjects(pkgs)
	var out []Violation
	for _, p := range pkgs {
		inKite := relPkg(module, p.PkgPath) == "internal/kite"
		for id, obj := range p.TypesInfo.Uses {
			name, ok := banned[obj]
			if !ok {
				continue
			}
			if inKite && containsStr(kiteConstructFiles, filepath.Base(p.Fset.Position(id.Pos()).Filename)) {
				continue
			}
			out = append(out, Violation{"g0-kitenew", position(p.Fset, id.Pos()),
				"kiteconnect." + name + " is only allowed in internal/kite/auth.go and internal/kite/client.go"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// httpUseFuncs are the package-level net/http identities that build or send a request outside the
// gokiteconnect client kite.New / NewAuth construct (g0-httpuse); httpUseMethods are the *http.Client
// methods that send one. A client built from them carries no write deny-list (netguard.go guardWrites).
var (
	httpUseFuncs   = []string{"Client", "Transport", "DefaultClient", "DefaultTransport", "NewRequest", "NewRequestWithContext", "Get", "Post", "PostForm", "Head"}
	httpUseMethods = []string{"Do", "Get", "Post", "PostForm", "Head"}
)

// httpAllowance is what one file may use of the g0-httpuse identities ("net/http.Client",
// "net/http.(*Client).Do") and why.
type httpAllowance struct {
	objects []string
	reason  string
}

// httpUseAllowed is the explicit allow-list of g0-httpuse, keyed by module-relative file (I3). Found with
// grep and CheckHTTPUse on the real module (tags webkit2_41,noorders / orders) on 2026-10-13: these are
// exactly the files that use one today. Adding a file or an object is a G-0 review item.
var httpUseAllowed = map[string]httpAllowance{
	"internal/kite/client.go": {[]string{"net/http.Client"},
		"kite.(*Client).attempt hands gokiteconnect a per-attempt *http.Client whose transport is ctxTransport over the recorder over guardWrites (New)"},
	"internal/kite/auth.go": {[]string{"net/http.Client", "net/http.DefaultTransport"},
		"NewAuth takes and copies a *http.Client and puts it behind guardWrites; nil gets NewAllowListTransport(http.DefaultTransport, AllowedHosts)"},
	"internal/kite/netguard.go": {[]string{"net/http.DefaultTransport"},
		"NewAllowListTransport and guardWrites default a nil next to http.DefaultTransport, under the deny-list they add"},
	"internal/kite/mock/recorder.go": {[]string{"net/http.Client"},
		"(*Recorder).Client returns the httptest server's client for tests; g0-mock bars non-test code from importing the mock"},
}

// httpUseObjects resolves httpUseFuncs and httpUseMethods in the loaded net/http to their objects, labelled
// "net/http.Name" and "net/http.(*Client).Name".
func httpUseObjects(pkgs []*packages.Package) map[types.Object]string {
	out := map[types.Object]string{}
	hp := typesByPath(pkgs)["net/http"]
	if hp == nil {
		return out
	}
	for _, n := range httpUseFuncs {
		if obj := hp.Scope().Lookup(n); obj != nil {
			out[obj] = "net/http." + n
		}
	}
	if tn, ok := hp.Scope().Lookup("Client").(*types.TypeName); ok {
		for _, n := range httpUseMethods {
			if m, _, _ := types.LookupFieldOrMethod(types.NewPointer(tn.Type()), true, hp, n); m != nil {
				if _, isFunc := m.(*types.Func); isFunc {
					out[m] = "net/http.(*Client)." + n
				}
			}
		}
	}
	return out
}

// HTTPUseMissing lists the g0-httpuse identities the loaded net/http does not declare, and the allow
// entries ("file: name") that are not one of them. pkgs (one load) must include net/http.
func HTTPUseMissing(pkgs []*packages.Package, allow map[string]httpAllowance) []string {
	banned := httpUseObjects(pkgs)
	labels := map[string]bool{}
	for _, l := range banned {
		labels[l] = true
	}
	var out []string
	for _, n := range httpUseFuncs {
		if !labels["net/http."+n] {
			out = append(out, "net/http."+n)
		}
	}
	for _, n := range httpUseMethods {
		if !labels["net/http.(*Client)."+n] {
			out = append(out, "net/http.(*Client)."+n)
		}
	}
	byPath := typesByPath(pkgs)
	for file, a := range allow {
		for _, name := range a.objects {
			switch {
			case labels[name]:
			case lookupPkgObject(byPath, name) != nil:
				out = append(out, file+": "+name+" (not a banned net/http identity)")
			default:
				out = append(out, file+": "+name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// CheckHTTPUse reports every use, in any module package, of a net/http identity that builds or sends a
// request (httpUseFuncs, httpUseMethods) outside the file and object pairs of httpUseAllowed (g0-httpuse,
// I3). g0-http lets all of internal/kite import net/http, so without this a hand-rolled client in a new
// internal/kite file would send requests that pass neither through kite.New / NewAuth (and so the write
// deny-list) nor through gokiteconnect's write methods (the nm check). Matched by types.Object identity
// through TypesInfo.Uses: renamed and dot-imports, function and method values, method expressions and
// methods on a *http.Client returned by another package are caught; kite.Client and fields named Client
// are not. Test files are not loaded and are exempt. Residual: a request sent through a RoundTripper or
// *http.Client value that an allowed file hands out (the method of the RoundTripper interface is not
// banned), httptest, and golang.org/x/net or raw net sockets under internal/kite.
func CheckHTTPUse(module string, pkgs []*packages.Package) []Violation {
	return checkHTTPUse(module, pkgs, httpUseAllowed)
}

func checkHTTPUse(module string, pkgs []*packages.Package, allow map[string]httpAllowance) []Violation {
	banned := httpUseObjects(pkgs)
	var out []Violation
	for _, p := range pkgs {
		rel := relPkg(module, p.PkgPath)
		for id, obj := range p.TypesInfo.Uses {
			name, ok := banned[obj]
			if !ok {
				continue
			}
			file := rel + "/" + filepath.Base(p.Fset.Position(id.Pos()).Filename)
			if containsStr(allow[file].objects, name) {
				continue
			}
			out = append(out, Violation{"g0-httpuse", position(p.Fset, id.Pos()),
				name + " builds or sends an HTTP request; allowed only in the httpUseAllowed files (kite.New / NewAuth put the write deny-list under every Kite client)"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// CheckOtherFiles reports every non-Go source file (assembly, C/C++/Objective-C/Fortran sources and
// headers, SWIG files, .syso objects) in a module package, whether this build includes it (OtherFiles)
// or another GOOS/GOARCH would (IgnoredFiles) (g0-src, Ruling 91 M4): code linked from them is invisible to
// every type-based G-0 rule. Packages under a testdata directory are exempt.
func CheckOtherFiles(module string, pkgs []*packages.Package) []Violation {
	var out []Violation
	for _, p := range pkgs {
		if inTestdata(relPkg(module, p.PkgPath)) {
			continue
		}
		files := append(append([]string(nil), p.OtherFiles...), p.IgnoredFiles...)
		for _, f := range files {
			if strings.HasSuffix(f, ".go") {
				continue
			}
			out = append(out, Violation{"g0-src", filepath.ToSlash(filepath.Base(filepath.Dir(f))) + "/" + filepath.Base(f),
				"non-Go source file in package " + p.PkgPath + "; only Go files may be linked"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// inTestdata reports whether the module-relative package path is a testdata directory or below one.
func inTestdata(rel string) bool {
	return rel == "testdata" || strings.HasPrefix(rel, "testdata/") || strings.Contains(rel, "/testdata/") || strings.HasSuffix(rel, "/testdata")
}

// fileImport is one import of a parsed file and its line.
type fileImport struct {
	path string
	line int
}

// parseImports parses the imports and the //go:build expression (nil if none) of the Go file at path.
func parseImports(path string) ([]fileImport, constraint.Expr, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	var out []fileImport
	for _, imp := range f.Imports {
		ip, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, fileImport{ip, fset.Position(imp.Pos()).Line})
	}
	return out, goBuildExpr(f), nil
}

func importPaths(imps []fileImport) []string {
	out := make([]string, 0, len(imps))
	for _, imp := range imps {
		out = append(out, imp.path)
	}
	return out
}

// fileLabel is "dir/file" for a file path, the form position uses.
func fileLabel(path string) string {
	return filepath.ToSlash(filepath.Base(filepath.Dir(path))) + "/" + filepath.Base(path)
}

// cgoAllowance lets one file (module-relative path) import "C" only if its //go:build line keeps it out of
// every build that does not set tag.
type cgoAllowance struct {
	file, tag, reason string
}

// cgoAllowed are the only cgo files of the module and of tradeloom/ta (Ruling 99). Both are test-oracle
// only: no release, archtest or nm-check build sets libmc or oracle, and no non-test file imports
// ta/internal/oracle. Adding an entry is a G-0 review item.
var cgoAllowed = []cgoAllowance{
	{"ta/libm_c.go", "libmc",
		"test-oracle only: routes ta's libm calls to the C library so the oracle tests (go test -tags oracle,libmc) can demand bit-exact agreement with TA-Lib C; the default build uses libm.go (//go:build !libmc, pure Go)"},
	{"ta/internal/oracle/oracle.go", "oracle",
		"test-oracle only: calls the reference TA-Lib C library from the *_oracle_test.go differential tests; imported by no non-test file, and unsafe here only converts cgo pointers"},
}

func cgoAllowedFile(key string, expr constraint.Expr, allow []cgoAllowance) bool {
	for _, a := range allow {
		if a.file == key && onlyWithTag(expr, a.tag) {
			return true
		}
	}
	return false
}

// CheckCgo reports import "C" in every .go file of a module package (g0-cgo, Ruling 99): the files this
// build compiles (packages.Package.GoFiles, which holds go list's GoFiles and CgoFiles) and the ones it
// does not (IgnoredFiles: excluded by a build tag, GOOS/GOARCH or CGO_ENABLED=0). C code linked through cgo
// is invisible to every type-based G-0 rule. Packages under testdata are exempt; cgoAllowed files are
// allowed under their tag only. A file that does not parse is reported (fail closed).
func CheckCgo(module string, pkgs []*packages.Package) []Violation {
	return checkCgo(module, pkgs, cgoAllowed)
}

func checkCgo(module string, pkgs []*packages.Package, allow []cgoAllowance) []Violation {
	var out []Violation
	for _, p := range pkgs {
		rel := relPkg(module, p.PkgPath)
		if inTestdata(rel) {
			continue
		}
		seen := map[string]bool{}
		for _, f := range append(append([]string(nil), p.GoFiles...), p.IgnoredFiles...) {
			if !strings.HasSuffix(f, ".go") || seen[f] {
				continue
			}
			seen[f] = true
			imps, expr, err := parseImports(f)
			if err != nil {
				out = append(out, Violation{"g0-cgo", fileLabel(f), "cannot parse imports: " + err.Error()})
				continue
			}
			for _, imp := range imps {
				if imp.path == "C" && !cgoAllowedFile(rel+"/"+filepath.Base(f), expr, allow) {
					out = append(out, Violation{"g0-cgo", fileLabel(f) + ":" + strconv.Itoa(imp.line),
						"import \"C\" in package " + p.PkgPath + "; cgo is not allowed (C code is invisible to the G-0 rules)"})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// walkGoSources calls fn for every non-test .go file under dir (module path modPath) with its package
// path, under any build tags: unlike go list, it also sees directories whose files are all excluded by
// tags or GOOS (Ruling 99 fix I-1). It skips testdata, directories starting with "." or "_", the
// module-relative directories in skip, and nested modules (a directory with its own go.mod) other than the
// module-relative ones in descend.
func walkGoSources(dir, modPath string, descend, skip []string, fn func(path, pkgPath string)) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if path == dir {
				return nil
			}
			name := d.Name()
			if name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || containsStr(skip, rel) {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil && !containsStr(descend, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		pkgPath := modPath
		if relDir := filepath.ToSlash(filepath.Dir(rel)); relDir != "." {
			pkgPath += "/" + relDir
		}
		fn(path, pkgPath)
		return nil
	})
}

// cgoSourceSkip are module-relative directories CheckCgoSources does not walk: the main go.mod ignores
// frontend/node_modules, so nothing in it is part of the module.
var cgoSourceSkip = []string{"frontend/node_modules"}

// CheckCgoSources walks the module at root (module path module) and reports import "C" in every non-test
// .go file, under any build tags, unless cgoAllowed allows it (g0-cgo, Ruling 99 fix I-1). It complements
// the load-based CheckCgo: go list drops a directory whose Go files are all excluded by tags or GOOS
// (NoGoError), so a tag-hidden cgo file alone in its directory never reaches CheckCgo. Nested modules are
// walked only if listed in descend (the real module descends into ta). Files that do not parse are
// reported (fail closed).
func CheckCgoSources(root, module string, descend []string) []Violation {
	var out []Violation
	err := walkGoSources(root, module, descend, cgoSourceSkip, func(path, pkgPath string) {
		imps, expr, err := parseImports(path)
		if err != nil {
			out = append(out, Violation{"g0-cgo", fileLabel(path), "cannot parse imports: " + err.Error()})
			return
		}
		for _, imp := range imps {
			if imp.path == "C" && !cgoAllowedFile(relPkg(module, pkgPath)+"/"+filepath.Base(path), expr, cgoAllowed) {
				out = append(out, Violation{"g0-cgo", fileLabel(path) + ":" + strconv.Itoa(imp.line),
					"import \"C\" in package " + pkgPath + "; cgo is not allowed (C code is invisible to the G-0 rules)"})
			}
		}
	})
	if err != nil {
		out = append(out, Violation{"g0-cgo", root, "walk: " + err.Error()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// isStdlib reports whether path resolves to a package in GOROOT, looked up from srcDir (Ruling 99 fix
// I-2): a dot-less first element is not enough, since main-module paths (tradeloom/internal/kite) and
// replaced modules (evil/sock) have none.
func isStdlib(path, srcDir string) bool {
	if path == "C" {
		return false
	}
	bp, err := build.Default.Import(path, srcDir, build.FindOnly)
	return err == nil && bp.Goroot
}

// taBanned are the imports no tradeloom/ta file may have (Ruling 99), beyond the stdlib-only rule:
// network, process, raw-syscall, plugin and unsafe/cgo access. "C" and "unsafe" are allowed only in the
// cgoAllowed files under their tag.
func taBanned(ip string) bool {
	// Stricter than g0-net (which bans only net itself, as internal/kite needs net/http): an indicator
	// library has no use for any net/... package, net/url included.
	return under(ip, "net") || under(ip, "golang.org/x/sys") || containsStr([]string{"os/exec", "syscall", "plugin", "unsafe", "C"}, ip)
}

// CheckTASources walks the nested module tradeloom/ta (dir, module path modPath) and checks every non-test
// .go file, under any build tags, outside testdata (g0-ta, Ruling 99): it may import only the standard
// library (resolved in GOROOT, isStdlib) and modPath's own packages, and none of net/..., os/exec, syscall,
// golang.org/x/sys/..., unsafe, plugin or "C" — except "C" and "unsafe" in a cgoAllowed file whose
// //go:build line requires its tag. ta will be linked into the binary through internal/candles/builder; it
// is not yet imported by any main-module package and is not matched by ./... TestArchitecture also loads
// it for the type-based rules, and this file walk sees the tag-gated files (ta/internal/oracle is not
// loaded at all without the oracle tag). Files that do not parse are reported (fail closed).
func CheckTASources(dir, modPath string) []Violation {
	var out []Violation
	err := walkGoSources(dir, modPath, nil, nil, func(path, pkgPath string) {
		key := relPkg("tradeloom", pkgPath) + "/" + filepath.Base(path)
		imps, expr, err := parseImports(path)
		if err != nil {
			out = append(out, Violation{"g0-ta", fileLabel(path), "cannot parse imports: " + err.Error()})
			return
		}
		for _, imp := range imps {
			at := fileLabel(path) + ":" + strconv.Itoa(imp.line)
			switch {
			case (imp.path == "C" || imp.path == "unsafe") && cgoAllowedFile(key, expr, cgoAllowed):
			case taBanned(imp.path):
				out = append(out, Violation{"g0-ta", at, "imports " + imp.path + " (not allowed in tradeloom/ta; C and unsafe only in the cgoAllowed test-oracle files under their tag)"})
			case under(imp.path, modPath):
			case isStdlib(imp.path, filepath.Dir(path)):
			default:
				out = append(out, Violation{"g0-ta", at, "imports " + imp.path + " (tradeloom/ta may import only the standard library and its own packages)"})
			}
		}
	})
	if err != nil {
		out = append(out, Violation{"g0-ta", dir, "walk: " + err.Error()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}
