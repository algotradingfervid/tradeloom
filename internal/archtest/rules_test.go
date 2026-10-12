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
