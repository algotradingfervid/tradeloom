package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// fixtureEnv loads the fixture module offline, with its fake gokiteconnect.
func fixtureEnv(t *testing.T) (string, []string) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	return dir, append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
}

// runFixture loads the fixture with tags and returns "rule dir/file.go:line" for every violation the
// checks report, sorted (Ruling 79 M-1: assertions keep the line, not only the file).
func runFixture(t *testing.T, tags string, checks ...func(string, []*packages.Package) []Violation) []string {
	t.Helper()
	dir, env := fixtureEnv(t)
	pkgs, err := Load(dir, env, tags, "./...")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, check := range checks {
		for _, v := range check("fixture", pkgs) {
			got = append(got, v.Rule+" "+v.Pkg)
		}
	}
	sort.Strings(got)
	return got
}

func loadFixture(t *testing.T, tags string) []string {
	t.Helper()
	return runFixture(t, tags, CheckOrderWrite, CheckClock, CheckKiteIndirect, CheckKiteRawWrite)
}

func assertList(t *testing.T, got, want []string) {
	t.Helper()
	want = append([]string(nil), want...)
	sort.Strings(want)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("\n got %q\nwant %q", got, want)
	}
}

func TestASTChecks_Fixture(t *testing.T) {
	common := []string{
		// The reviewer's generic probe (Ruling 39) and its method-value / interface variants.
		"g0-generic bad/generic.go:18", "g0-generic bad/genericvalue.go:12",
		"g0-iface bad/generic.go:14",
		"g0-kitecall bad/generic.go:22", "g0-kitecall bad/genericvalue.go:18", "g0-kitecall bad/genericvalue.go:22",
		"g0-typearg bad/generic.go:22", "g0-typearg bad/genericvalue.go:19", "g0-typearg bad/genericvalue.go:22",
		"g0-typearg bad/ifacecall.go:17", "g0-typearg bad/ifacecall.go:17",
		// Order writes on *kiteconnect.Client: direct, embedded, method value, untagged orderwrite file, and
		// sneaky.go, whose "|| go1.1" links it without the orders tag (Ruling 79 I-1).
		"g0-call bad/bad.go:6", "g0-call bad/embed.go:8", "g0-call bad/value.go:7",
		"g0-call orderwrite/untagged.go:6", "g0-call orderwrite/sneaky.go:11",
		// rule 8: wall-clock reads outside internal/clock.
		"rule8 timeuser/dotuser.go:3", "rule8 timeuser/timeuser.go:10", "rule8 timeuser/timeuser.go:11",
		// Ruling 69 raw HTTP writes: qualified, aliased, constant copy, unexported implementation,
		// dot-import, embedding, an untagged orderwrite file and sneaky.go. writer.go (orders-only) is allowed.
		"g0-kitehttp rawhttp/kitehttp.go:13", "g0-kitehttp rawhttp/kitehttp.go:14", "g0-kitehttp rawhttp/kitehttp.go:14",
		"g0-kitehttp rawhttp/kitehttp.go:20", "g0-kitehttp rawhttp/kitehttp.go:23", "g0-kitehttp rawhttp/kitehttp.go:28",
		"g0-kitehttp rawhttp/kitehttp.go:28", "g0-kitehttp rawhttp/kitehttp.go:29",
		"g0-kitehttp rawhttp/dotkite.go:7", "g0-kitehttp rawhttp/dotkite.go:7", "g0-kitehttp rawhttp/dotkite.go:7",
		"g0-kitehttp rawhttp/embed.go:7", "g0-kitehttp rawhttp/embed.go:10",
		"g0-kitehttp orderwrite/uri.go:7", "g0-kitehttp orderwrite/sneaky.go:12",
	}
	// Alert-triggered and mutual-fund order writes (Ruling 47) and InitiateHoldingsAuth (Ruling 48).
	for line := 7; line <= 15; line++ {
		common = append(common, fmt.Sprintf("g0-call bad/alertsmf.go:%d", line))
	}
	cases := []struct {
		tags  string
		extra []string
	}{
		{"noorders", nil},
		// With the orders tag writer.go is loaded and allowed; ordersonly.go ("orders" only) is flagged.
		{"orders", []string{"g0-call orderwrite/ordersonly.go:8"}},
		// Both tags: writer.go is excluded; a file tagged only "orders" still links and is flagged.
		{"orders,noorders", []string{"g0-call orderwrite/ordersonly.go:8"}},
	}
	for _, tc := range cases {
		t.Run(tc.tags, func(t *testing.T) {
			assertList(t, loadFixture(t, tc.tags), append(append([]string{}, common...), tc.extra...))
		})
	}
}

// requiresOrdersTag (Ruling 79 I-1): orders-only means false for every assignment of the expression's
// other tags in which orders is false or noorders is true.
func TestRequiresOrdersTag(t *testing.T) {
	for expr, want := range map[string]bool{
		"orders && !noorders":                      true,
		"orders":                                   false, // links under orders,noorders
		"!noorders":                                false,
		"(orders && !noorders) || go1.1":           false,
		"(orders && !noorders) || linux":           false,
		"orders && !noorders && linux":             true,
		"orders && !noorders && (linux || !linux)": true,
		"!orders || noorders":                      false,
		"orders || !orders":                        false,
	} {
		f, err := parser.ParseFile(token.NewFileSet(), "x.go", "//go:build "+expr+"\n\npackage x\n", parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		if got := requiresOrdersTag(f); got != want {
			t.Errorf("requiresOrdersTag(%q) = %v, want %v", expr, got, want)
		}
	}
}

func TestKitePkg_AnyMajorVersion(t *testing.T) {
	for path, want := range map[string]bool{
		"github.com/zerodha/gokiteconnect/v4":             true,
		"github.com/zerodha/gokiteconnect/v5":             true,
		"github.com/zerodha/gokiteconnect/v10":            true,
		"github.com/zerodha/gokiteconnect/v4/models":      false,
		"github.com/zerodha/gokiteconnect/v4/ticker":      false,
		"example.com/github.com/zerodha/gokiteconnect/v4": false,
	} {
		if got := kitePkg.MatchString(path); got != want {
			t.Errorf("kitePkg(%q) = %v, want %v", path, got, want)
		}
	}
}

// Import rules on the fixture module (Ruling 69, Ruling 79 I-2/M-3): rawhttp and netuser import
// net/http outside internal/kite (g0-http); netuser imports net, crypto/tls, os/exec and unsafe (g0-net)
// and internal/kite/mock (g0-mock). internal/kite may import net, net/http and crypto/tls. Test files
// (netuser_test.go, rawhttp/kitehttp_test.go) are not loaded, so they are exempt.
func TestImportRules_Fixture(t *testing.T) {
	dir, env := fixtureEnv(t)
	pkgs, err := Load(dir, env, "noorders", "./...")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, v := range CheckImports("fixture", Graph(pkgs)) {
		if strings.HasPrefix(v.Rule, "g0-") {
			imp, _, _ := strings.Cut(strings.TrimPrefix(v.Detail, "imports "), " ")
			got = append(got, v.Rule+" "+v.Pkg+" "+imp)
		}
	}
	sort.Strings(got)
	assertList(t, got, []string{
		"g0-http fixture/internal/rawhttp net/http",
		"g0-http fixture/internal/netuser net/http",
		"g0-net fixture/internal/netuser crypto/tls",
		"g0-net fixture/internal/netuser net",
		"g0-net fixture/internal/netuser os/exec",
		"g0-net fixture/internal/netuser unsafe",
		"g0-mock fixture/internal/netuser fixture/internal/kite/mock",
	})
}

// Every Ruling 69 identifier must resolve in the real gokiteconnect; a module bump that renames one
// would otherwise disable that part of the rule silently.
func TestKiteRawWrite_NamesResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("loads and type-checks the whole module")
	}
	pkgs, err := Load(moduleRoot(t), nil, "webkit2_41,noorders", "./internal/kite")
	if err != nil {
		t.Fatal(err)
	}
	if missing := KiteRawWriteMissing(pkgs); len(missing) != 0 {
		t.Fatalf("Ruling 69 identifiers not found in gokiteconnect: %v", missing)
	}
}

// g0-path (Ruling 69 addendum, Ruling 79 M-1) on the fixture module: literal, folded-concatenation,
// whitespace, host-prefixed, upper-case and exact "/mf" write paths are flagged, as are the folded
// gokiteconnect URI constants of the rawhttp fixtures, the untagged uri.go, sneaky.go and the kite/mockx
// look-alike. Read paths (clean.go), the fake internal/kite/mock and the orders-only writer.go are not;
// test files are not loaded.
func TestPathRule_Fixture(t *testing.T) {
	want := []string{
		"g0-path orderwrite/uri.go:7", "g0-path orderwrite/sneaky.go:12", "g0-path mockx/mockx.go:5",
		"g0-path rawhttp/dotkite.go:7", "g0-path rawhttp/dotkite.go:11",
		"g0-path rawhttp/kitehttp.go:14", "g0-path rawhttp/kitehttp.go:23", "g0-path rawhttp/kitehttp.go:29", "g0-path rawhttp/kitehttp.go:35",
		"g0-path rawpath/path.go:4", "g0-path rawpath/path.go:13", "g0-path rawpath/path.go:14", "g0-path rawpath/path.go:19", "g0-path rawpath/path.go:20",
		"g0-path rawpath/upper.go:5", "g0-path rawpath/upper.go:6", "g0-path rawpath/upper.go:7",
	}
	for _, tags := range []string{"noorders", "orders", "orders,noorders"} {
		t.Run(tags, func(t *testing.T) { assertList(t, runFixture(t, tags, CheckKitePaths), want) })
	}
}

// Ruling 79 M-5: KiteRawWriteMissing also reports an HTTPClient method that resolved on no implementing
// type, so a gokiteconnect change that renames or hides *httpClient cannot silently drop those bans.
func TestKiteRawWriteMissing_HTTPClientMethods(t *testing.T) {
	const decls = `package kiteconnect
type HTTPClient interface{ Do(string) error; GetClient() *httpClient }
func NewHTTPClient() HTTPClient { return nil }
const (URIPlaceOrder, URIModifyOrder, URICancelOrder, URIConvertPosition, URIInitHoldingsAuth = "", "", "", "", ""
	URIPlaceGTT, URIModifyGTT, URIDeleteGTT, URIAlerts, URIAlert = "", "", "", "", ""
	URIPlaceMFOrder, URICancelMFOrder, URIPlaceMFSIP, URIModifyMFSIP, URICancelMFSIP = "", "", "", "", "")
`
	for name, tc := range map[string]struct {
		impl string
		want []string
	}{
		"with *httpClient":     {"type httpClient struct{}\nfunc (*httpClient) Do(string) error { return nil }\nfunc (h *httpClient) GetClient() *httpClient { return h }\n", nil},
		"Do renamed":           {"type httpClient struct{}\nfunc (*httpClient) Do2(string) error { return nil }\nfunc (h *httpClient) GetClient() *httpClient { return h }\n", []string{"(*httpClient).Do", "(*httpClient).GetClient"}}, // no longer implements HTTPClient
		"no implementing type": {"type httpClient struct{}\n", []string{"(*httpClient).Do", "(*httpClient).GetClient"}},
	} {
		t.Run(name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "k.go", decls+tc.impl, 0)
			if err != nil {
				t.Fatal(err)
			}
			tp, err := (&types.Config{}).Check("github.com/zerodha/gokiteconnect/v4", fset, []*ast.File{f}, nil)
			if err != nil {
				t.Fatal(err)
			}
			got := KiteRawWriteMissing([]*packages.Package{{PkgPath: tp.Path(), Types: tp}})
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("missing %q, want %q", got, tc.want)
			}
		})
	}
}
