// internal/settings/keys_ui_test.go
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// Contract §E / P0-4 A7: names, defaults and validators of the UI keys.
func TestUIKeys_DefaultsAndNames(t *testing.T) {
	s, err := Load(context.Background(), &regTestBackend{m: map[string]string{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"appearance.theme":             `"light"`,
		"appearance.follow_system":     `false`,
		"appearance.sidebar_collapsed": `false`,
		"ui.last_route":                `"/home"`,
		"chart.default_timeframe":      `"60minute"`,
		"calendar.stale_toast_day":     `""`,
	} {
		raw, err := GetJSON(s, name)
		if err != nil || string(raw) != want {
			t.Fatalf("%s = %s (%v), want %s", name, raw, err, want)
		}
	}
	if AppearanceTheme.Name != "appearance.theme" || UILastRoute.Name != "ui.last_route" || ChartDefaultTimeframe.Name != "chart.default_timeframe" {
		t.Fatal("key variables do not carry the contract names")
	}
	if len(KiteIntervals) != 8 || KiteIntervals[0] != "minute" || KiteIntervals[7] != "day" {
		t.Fatalf("intervals %v", KiteIntervals)
	}
}

func TestUIKeys_Validators(t *testing.T) {
	ctx := context.Background()
	s, err := Load(ctx, &regTestBackend{m: map[string]string{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ok := map[string]string{
		"appearance.theme":         `"dark"`,
		"ui.last_route":            `"/settings/data"`,
		"chart.default_timeframe":  `"5minute"`,
		"calendar.stale_toast_day": `"2026-10-12"`,
	}
	for name, raw := range ok {
		if err := SetJSON(ctx, s, name, json.RawMessage(raw)); err != nil {
			t.Fatalf("%s = %s refused: %v", name, raw, err)
		}
	}
	bad := map[string]string{
		"appearance.theme":         `"blue"`,
		"ui.last_route":            `"home"`,
		"chart.default_timeframe":  `"7minute"`,
		"calendar.stale_toast_day": `"12-10-2026"`,
		"appearance.follow_system": `"yes"`,
	}
	var ve *ValidationError
	for name, raw := range bad {
		if err := SetJSON(ctx, s, name, json.RawMessage(raw)); !errors.As(err, &ve) {
			t.Fatalf("%s = %s accepted (%v)", name, raw, err)
		}
	}
	if Get(s, AppearanceTheme) != "dark" || Get(s, UILastRoute) != "/settings/data" {
		t.Fatal("accepted values were not stored")
	}
}

// Task 2 minor: no registered UI key may look like a secret (NFR-04).
func TestUIKeys_NoSecretLikeNames(t *testing.T) {
	for _, n := range Names() {
		if isSecretKey(n) {
			t.Fatalf("registered key %q is secret-like", n)
		}
	}
}

func TestValidateRoute_RejectsEscapes(t *testing.T) {
	for _, bad := range []string{"//evil.com", "/a/../b", "/..", "/a/..", "/../x"} {
		if ValidateRoute(bad) == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
	for _, ok := range []string{"/home", "/settings/data", "/a..b", "/"} {
		if err := ValidateRoute(ok); err != nil {
			t.Fatalf("%q refused: %v", ok, err)
		}
	}
}

// Final review I4: a route taken from a launch's argv is a bare path — a query or fragment is where a
// pasted URL would carry a token, so it is refused before anything is logged or navigated.
func TestValidateLaunchRoute_RejectsQueryAndFragment(t *testing.T) {
	for _, bad := range []string{"/home?request_token=x", "/home#token", "/?", "/a/b?c", "//evil.com", "/a/../b", "home", ""} {
		if ValidateLaunchRoute(bad) == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
	for _, ok := range []string{"/home", "/settings/data", "/a..b", "/"} {
		if err := ValidateLaunchRoute(ok); err != nil {
			t.Fatalf("%q refused: %v", ok, err)
		}
	}
}
