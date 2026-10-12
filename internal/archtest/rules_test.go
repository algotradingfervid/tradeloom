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
		// g0-http (Ruling 69): net/http and its sub-packages only under internal/kite.
		{"kite imports net/http", Pkg{"tradeloom/internal/kite", []string{"net/http"}}, ""},
		{"kite/mock imports httptest", Pkg{"tradeloom/internal/kite/mock", []string{"net/http/httptest"}}, ""},
		{"app imports net/http", Pkg{"tradeloom/internal/app", []string{"net/http"}}, "g0-http"},
		{"service imports httputil", Pkg{"tradeloom/internal/service", []string{"net/http/httputil"}}, "g0-http"},
		{"kitex is not kite", Pkg{"tradeloom/internal/kitex", []string{"net/http"}}, "g0-http"},
		{"cmd imports net/http", Pkg{"tradeloom/cmd/tradeloom", []string{"net/http"}}, "g0-http"},
		{"orders imports net/url", Pkg{"tradeloom/internal/orders", []string{"net/url"}}, ""},
		{"service imports x/net/http2", Pkg{"tradeloom/internal/service", []string{"golang.org/x/net/http2"}}, "g0-http"},
		{"kite imports x/net/http2", Pkg{"tradeloom/internal/kite", []string{"golang.org/x/net/http2"}}, ""},
		// g0-net (Ruling 79 I-2): net and crypto/tls only under internal/kite (exact paths: net/url and
		// net/netip are fine), unsafe nowhere, os/exec only under internal/app.
		{"service imports net", Pkg{"tradeloom/internal/service", []string{"net"}}, "g0-net"},
		{"service imports net/netip", Pkg{"tradeloom/internal/service", []string{"net/netip"}}, ""},
		{"kite imports net", Pkg{"tradeloom/internal/kite", []string{"net"}}, ""},
		{"app imports crypto/tls", Pkg{"tradeloom/internal/app", []string{"crypto/tls"}}, "g0-net"},
		{"kite imports crypto/tls", Pkg{"tradeloom/internal/kite", []string{"crypto/tls"}}, ""},
		{"store imports crypto/tls/x", Pkg{"tradeloom/internal/store", []string{"crypto/tlsx"}}, ""},
		{"kite imports unsafe", Pkg{"tradeloom/internal/kite", []string{"unsafe"}}, "g0-net"},
		{"app imports unsafe", Pkg{"tradeloom/internal/app", []string{"unsafe"}}, "g0-net"},
		{"app imports os/exec", Pkg{"tradeloom/internal/app", []string{"os/exec"}}, ""},
		{"app/opener imports os/exec", Pkg{"tradeloom/internal/app/opener", []string{"os/exec"}}, ""},
		{"service imports os/exec", Pkg{"tradeloom/internal/service", []string{"os/exec"}}, "g0-net"},
		{"cmd imports os/exec", Pkg{"tradeloom/cmd/tradeloom", []string{"os/exec"}}, "g0-net"},
		// g0-mock (Ruling 79 M-3): no non-test import of internal/kite/mock.
		{"service imports kite/mock", Pkg{"tradeloom/internal/service", []string{"tradeloom/internal/kite/mock"}}, "g0-mock"},
		{"kite imports kite/mock", Pkg{"tradeloom/internal/kite", []string{"tradeloom/internal/kite/mock"}}, "g0-mock"},
		{"kite/mock/sub imports kite/mock", Pkg{"tradeloom/internal/kite/mock/sub", []string{"tradeloom/internal/kite/mock"}}, ""},
		{"service imports kite/mockx", Pkg{"tradeloom/internal/service", []string{"tradeloom/internal/kite/mockx"}}, ""},
		// g0-exec (Ruling 83): syscall and golang.org/x/sys/... only in the allow-listed packages, exactly.
		{"service imports syscall", Pkg{"tradeloom/internal/service", []string{"syscall"}}, ""},
		{"service/servicetest imports syscall", Pkg{"tradeloom/internal/service/servicetest", []string{"syscall"}}, "g0-exec"},
		{"store imports syscall", Pkg{"tradeloom/internal/store", []string{"syscall"}}, "g0-exec"},
		{"app imports syscall", Pkg{"tradeloom/internal/app", []string{"syscall"}}, "g0-exec"},
		{"kite imports x/sys/unix", Pkg{"tradeloom/internal/kite", []string{"golang.org/x/sys/unix"}}, "g0-exec"},
		{"service imports x/sys/unix", Pkg{"tradeloom/internal/service", []string{"golang.org/x/sys/unix"}}, ""},
		{"cmd imports x/sys/windows", Pkg{"tradeloom/cmd/tradeloom", []string{"golang.org/x/sys/windows"}}, "g0-exec"},
		{"store imports syscallx", Pkg{"tradeloom/internal/store", []string{"syscallx"}}, ""},
		// g0-plugin (Ruling 83): plugin nowhere.
		{"app imports plugin", Pkg{"tradeloom/internal/app", []string{"plugin"}}, "g0-plugin"},
		{"kite imports plugin", Pkg{"tradeloom/internal/kite", []string{"plugin"}}, "g0-plugin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := CheckImports("tradeloom", []Pkg{tc.pkg})
			switch {
			case tc.wantRule == "" && len(v) != 0:
				t.Fatalf("unexpected violations %v", v)
			case tc.wantRule != "" && !hasRule(v, tc.wantRule):
				t.Fatalf("want %s, got %v", tc.wantRule, v)
			}
		})
	}
}

// hasRule: a package may break more than one rule (internal/clock importing net/http breaks rule1 and g0-http).
func hasRule(vs []Violation, rule string) bool {
	for _, v := range vs {
		if v.Rule == rule {
			return true
		}
	}
	return false
}

func TestUnder(t *testing.T) {
	if !under("internal/kite/ticker", "internal/kite") || under("internal/kitex", "internal/kite") || !under("a", "a") {
		t.Fatal("under")
	}
	if !strings.HasPrefix(Violation{"r", "p", "d"}.String(), "r: p") {
		t.Fatal("String")
	}
}
