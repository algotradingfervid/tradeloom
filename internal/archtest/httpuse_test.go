package archtest

import (
	"testing"

	"golang.org/x/tools/go/packages"
)

// fixtureHTTPAllow stands in for httpUseAllowed on the fixture module: kite/http.go may use
// http.DefaultTransport and kite/allowed.go http.Client, nothing else.
var fixtureHTTPAllow = map[string]httpAllowance{
	"internal/kite/http.go":    {objects: []string{"net/http.DefaultTransport"}},
	"internal/kite/allowed.go": {objects: []string{"net/http.Client"}},
}

func runHTTPUse(t *testing.T, tags string, allow map[string]httpAllowance) []string {
	t.Helper()
	return runFixture(t, tags, func(module string, pkgs []*packages.Package) []Violation {
		return checkHTTPUse(module, pkgs, allow)
	})
}

// g0-httpuse (I3): the net/http identities that build or send a request are flagged in every file not on
// the allow-list, under internal/kite too: a struct literal, constructors, package-level senders, the
// default client and transport, renamed and dot-imports, function and method values, a method expression,
// and a method on a *http.Client another package returns (httpvia/via.go imports no net/http). kite.Client,
// http.StatusOK and http.Handler are not net/http senders and stay clean; allowed.go and http.go pass.
func TestHTTPUse_Fixture(t *testing.T) {
	want := []string{
		"g0-httpuse kite/handrolled.go:13",                                     // &http.Client{}
		"g0-httpuse kite/handrolled.go:14",                                     // http.NewRequest
		"g0-httpuse kite/handrolled.go:15",                                     // c.Do
		"g0-httpuse kite/handrolled.go:16", "g0-httpuse kite/handrolled.go:16", // http.DefaultClient.Get
		"g0-httpuse kite/handrolled.go:17",                                     // http.Post
		"g0-httpuse kite/handrolled.go:18",                                     // http.PostForm
		"g0-httpuse kite/handrolled.go:19",                                     // h.Get (renamed import)
		"g0-httpuse kite/handrolled.go:20",                                     // http.Head (function value)
		"g0-httpuse kite/handrolled.go:22",                                     // &http.Transport{}
		"g0-httpuse kite/handrolled.go:23",                                     // http.DefaultTransport
		"g0-httpuse kite/handrolled.go:24",                                     // http.NewRequestWithContext
		"g0-httpuse kite/handrolled.go:25",                                     // c.Do (method value)
		"g0-httpuse kite/handrolled.go:27", "g0-httpuse kite/handrolled.go:27", // (*http.Client).PostForm
		"g0-httpuse httpvia/via.go:7",                                // kite.Shared().Post, no net/http import
		"g0-httpuse httpvia/dot.go:5", "g0-httpuse httpvia/dot.go:5", // DefaultClient.Head (dot-import)
		// Outside internal/kite (g0-http flags the imports as well).
		"g0-httpuse rawhttp/kitehttp.go:13",  // http.DefaultClient
		"g0-httpuse rawhttp/newclient.go:12", // *http.Client
	}
	for _, tags := range []string{"noorders", "orders", "orders,noorders"} {
		t.Run(tags, func(t *testing.T) { assertList(t, runHTTPUse(t, tags, fixtureHTTPAllow), want) })
	}
}

// The allow-list is per file and per object (identity): without an entry, kite/http.go's
// http.DefaultTransport is flagged; allowing allowed.go only DefaultTransport leaves its two http.Client
// uses flagged.
func TestHTTPUse_PerFileObject(t *testing.T) {
	allow := map[string]httpAllowance{"internal/kite/allowed.go": {objects: []string{"net/http.DefaultTransport"}}}
	var got []string
	for _, s := range runHTTPUse(t, "noorders", allow) {
		if s == "g0-httpuse kite/http.go:10" || s == "g0-httpuse kite/allowed.go:6" {
			got = append(got, s)
		}
	}
	assertList(t, got, []string{"g0-httpuse kite/allowed.go:6", "g0-httpuse kite/allowed.go:6", "g0-httpuse kite/http.go:10"})
}

// Every banned name must resolve in net/http, and every allow entry to a banned object, so a toolchain
// change or a typo cannot silently drop part of the rule or leave an allowance that matches nothing.
func TestHTTPUseMissing(t *testing.T) {
	dir, env := fixtureEnv(t)
	pkgs, err := Load(dir, env, "noorders", "./...")
	if err != nil {
		t.Fatal(err)
	}
	if m := HTTPUseMissing(pkgs, httpUseAllowed); len(m) != 0 {
		t.Fatalf("banned names or real allow-list entries not found: %v", m)
	}
	allow := map[string]httpAllowance{"internal/kite/x.go": {objects: []string{"net/http.NoSuchThing", "net/http.StatusOK", "net/http.Client", "nodot"}}}
	assertList(t, HTTPUseMissing(pkgs, allow), []string{
		"internal/kite/x.go: net/http.NoSuchThing", "internal/kite/x.go: net/http.StatusOK (not a banned net/http identity)",
		"internal/kite/x.go: nodot"})
}
