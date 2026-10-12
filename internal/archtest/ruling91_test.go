package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/packages"
)

// Ruling 91 M1a: in the allow-listed importers only the listed syscall / x/sys objects may be used
// (internal/service: O_NONBLOCK, internal/app: Exec); raw Syscall/RawSyscall and anything else are
// flagged, as is every syscall / x/sys object used in any other package.
func TestSysObjects_Fixture(t *testing.T) {
	want := []string{
		"g0-sys service/raw.go:7", "g0-sys service/raw.go:8", // Syscall, RawSyscall
		"g0-sys app/reexec.go:9", // Getpid; Exec on line 6 is allowed
		// execuser: syscall.Exec/ForkExec/StartProcess, unix.Exec/ForkExec, syscall.Getpid, unix.Getpid.
		"g0-sys execuser/execuser.go:20", "g0-sys execuser/execuser.go:21", "g0-sys execuser/execuser.go:22",
		"g0-sys execuser/execuser.go:23", "g0-sys execuser/execuser.go:24",
		"g0-sys execuser/execuser.go:30", "g0-sys execuser/execuser.go:31",
	}
	for _, tags := range []string{"noorders", "orders", "orders,noorders"} {
		t.Run(tags, func(t *testing.T) { assertList(t, runFixture(t, tags, CheckSysObjects), want) })
	}
}

// Ruling 91 M3: kiteconnect.New and (*Client).SetHTTPClient only in internal/kite/auth.go and client.go,
// so no other code can build a gokiteconnect client that skips the write deny-list.
func TestKiteConstruct_Fixture(t *testing.T) {
	want := []string{
		"g0-kitenew kite/other.go:8", "g0-kitenew kite/other.go:9",
		"g0-kitenew rawhttp/newclient.go:10", "g0-kitenew rawhttp/newclient.go:12",
	}
	for _, tags := range []string{"noorders", "orders", "orders,noorders"} {
		t.Run(tags, func(t *testing.T) { assertList(t, runFixture(t, tags, CheckKiteConstruct), want) })
	}
}

// Ruling 91 M4: non-Go source files in a module package are flagged, including those another GOARCH
// would build (IgnoredFiles).
func TestOtherFiles_Fixture(t *testing.T) {
	want := []string{"g0-src asmuser/any.s", "g0-src asmuser/defs.h", "g0-src asmuser/stub_amd64.s", "g0-src asmuser/stub_arm64.s"}
	for _, tags := range []string{"noorders", "orders", "orders,noorders"} {
		t.Run(tags, func(t *testing.T) { assertList(t, runFixture(t, tags, CheckOtherFiles), want) })
	}
}

// Ruling 91 M6: when several types implement HTTPClient through the same promoted method, the method is
// banned under the first implementing type by name, every run, and no implementation is reported missing.
func TestKiteRawWrite_DeterministicLabel(t *testing.T) {
	const src = `package kiteconnect
type HTTPClient interface{ Do(string) error; GetClient() *httpClient }
func NewHTTPClient() HTTPClient { return nil }
const (URIPlaceOrder, URIModifyOrder, URICancelOrder, URIConvertPosition, URIInitHoldingsAuth = "", "", "", "", ""
	URIPlaceGTT, URIModifyGTT, URIDeleteGTT, URIAlerts, URIAlert = "", "", "", "", ""
	URIPlaceMFOrder, URICancelMFOrder, URIPlaceMFSIP, URIModifyMFSIP, URICancelMFSIP = "", "", "", "", "")
type httpClient struct{ *base }
func (*httpClient) GetClient() *httpClient { return nil }
type base struct{}
func (*base) Do(string) error { return nil }
type aClient struct{ *base }
func (*aClient) GetClient() *httpClient { return nil }
type zClient struct{ *base }
func (*zClient) GetClient() *httpClient { return nil }
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "k.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	tp, err := (&types.Config{}).Check("github.com/zerodha/gokiteconnect/v4", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pkgs := []*packages.Package{{PkgPath: tp.Path(), Types: tp}}
	do := tp.Scope().Lookup("base").Type().(*types.Named).Method(0)
	for i := 0; i < 20; i++ {
		if n := kiteRawWriteObjects(pkgs)[do]; n != "(*aClient).Do" {
			t.Fatalf("run %d: (*base).Do labelled %q, want (*aClient).Do", i, n)
		}
		if m := KiteRawWriteMissing(pkgs); len(m) != 0 {
			t.Fatalf("run %d: missing %v", i, m)
		}
	}
}
