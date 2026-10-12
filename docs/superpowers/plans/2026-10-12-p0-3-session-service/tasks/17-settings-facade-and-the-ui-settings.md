> **P0-3 Session & Service Layer Implementation Plan — Task 17.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 17: Settings façade and the UI settings keys

**Files:**
- Create: `internal/service/settings.go`, `internal/settings/keys_ui.go`
- Test: `internal/service/settings_test.go`, `internal/settings/keys_ui_test.go`

**Interfaces:**
- Consumes: Task 2's registry `settings.Register`, `settings.Names()`, `settings.GetJSON(s, name)`, `settings.SetJSON(ctx, s, name, raw)`, `settings.ErrUnknownKey` (P0-1's `*settings.Settings` has no per-name access of its own); P0-1 `settings.Key[T]`, `settings.OneOf`, `settings.ValidationError`, `settings.Load`, `settings.Get` and the `regTestBackend` test backend of Task 2.
- Produces:
  ```go
  // package service
  type SettingValue struct{ Name, ValueJSON string }
  func (s *Service) Settings(ctx context.Context) ([]SettingValue, error)
  func (s *Service) SetSetting(ctx context.Context, name, valueJSON string) (SettingValue, error)
  // package settings (keys_ui.go) — contract §E; P0-4 reads them through Settings.All / Settings.Set and settings.changed
  var AppearanceTheme = Register(Key[string]{Name: "appearance.theme", Default: "light", Validate: OneOf("light", "dark")})
  var AppearanceFollowSystem = Register(Key[bool]{Name: "appearance.follow_system", Default: false})
  var AppearanceSidebarCollapsed = Register(Key[bool]{Name: "appearance.sidebar_collapsed", Default: false})
  var UILastRoute = Register(Key[string]{Name: "ui.last_route", Default: "/home", Validate: ValidateRoute})
  var ChartDefaultTimeframe = Register(Key[string]{Name: "chart.default_timeframe", Default: "60minute", Validate: OneOf(KiteIntervals...)})
  var CalendarStaleToastDay = Register(Key[string]{Name: "calendar.stale_toast_day", Default: "", Validate: ValidateDateOrEmpty})
  var KiteIntervals = []string{"minute", "3minute", "5minute", "10minute", "15minute", "30minute", "60minute", "day"}
  func ValidateRoute(v string) error; func ValidateDateOrEmpty(v string) error
  ```
  Keys under `kite.` are hidden from the list and refused with `use_credentials_flow`, because they change only through the credential methods.

`KiteIntervals` repeats the eight native Kite intervals rather than importing `candles.Native`: `settings` is a P0-1 platform package that may depend only on `bus` (`.go-arch-lint.yml`), and the list is fixed by Kite's API.

- [ ] **Step 1: Write the failing test**

```go
// internal/service/settings_test.go
package service

import (
	"context"
	"testing"
)

func TestSettingsFacade(t *testing.T) {
	e := newSvcEnv(t)
	ctx := context.Background()
	all, err := e.svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range all {
		if len(v.Name) >= 5 && v.Name[:5] == "kite." {
			t.Fatalf("credential key %s listed", v.Name)
		}
	}
	v, err := e.svc.SetSetting(ctx, "watchlist.active", `"wl-9"`)
	if err != nil || v.ValueJSON != `"wl-9"` {
		t.Fatalf("%+v %v", v, err)
	}
	if _, err := e.svc.SetSetting(ctx, "kite.redirect_port", `8000`); refusedCode(t, err) != "use_credentials_flow" {
		t.Fatal(err)
	}
	if _, err := e.svc.SetSetting(ctx, "no.such.key", `1`); refusedCode(t, err) != "unknown_setting" {
		t.Fatal(err)
	}
	if _, err := e.svc.SetSetting(ctx, "watchlist.active", `not json`); refusedCode(t, err) != "invalid_setting" {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/service/ -run TestSettingsFacade -v`
Expected: FAIL to compile with `undefined: (*Service).Settings`.

- [ ] **Step 3: Write the implementation**

```go
// internal/service/settings.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"tradeloom/internal/settings"
)

type SettingValue struct {
	Name      string `json:"name"`
	ValueJSON string `json:"valueJson"`
}

const settingsStage = "settings"

func credentialKey(name string) bool { return strings.HasPrefix(name, "kite.") }

// Settings lists every registered key (sorted by settings.Names) except the credential keys.
func (s *Service) Settings(_ context.Context) ([]SettingValue, error) {
	names := settings.Names()
	sort.Strings(names)
	out := make([]SettingValue, 0, len(names))
	for _, n := range names {
		if credentialKey(n) {
			continue
		}
		raw, err := settings.GetJSON(s.d.Settings, n)
		if err != nil {
			return nil, asRefused(settingsStage, err)
		}
		out = append(out, SettingValue{Name: n, ValueJSON: string(raw)})
	}
	return out, nil
}

func (s *Service) SetSetting(ctx context.Context, name, valueJSON string) (SettingValue, error) {
	if credentialKey(name) {
		return SettingValue{}, refused(settingsStage, "use_credentials_flow", "Change Kite settings in Settings → Kite connection.")
	}
	if !json.Valid([]byte(valueJSON)) {
		return SettingValue{}, refused(settingsStage, "invalid_setting", "The value is not valid JSON.")
	}
	if err := settings.SetJSON(ctx, s.d.Settings, name, json.RawMessage(valueJSON)); err != nil {
		if errors.Is(err, settings.ErrUnknownKey) {
			return SettingValue{}, refused(settingsStage, "unknown_setting", "Unknown setting "+name+".")
		}
		return SettingValue{}, refused(settingsStage, "invalid_setting", err.Error())
	}
	raw, err := settings.GetJSON(s.d.Settings, name)
	if err != nil {
		return SettingValue{}, asRefused(settingsStage, err)
	}
	return SettingValue{Name: name, ValueJSON: string(raw)}, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test -tags webkit2_41,noorders ./internal/service/ -run TestSettingsFacade -v`
Expected: PASS.

- [ ] **Step 5: Write the failing UI-keys test**

```go
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
```

- [ ] **Step 6: Run the UI-keys test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/settings/ -run TestUIKeys -v`
Expected: FAIL to compile with `undefined: AppearanceTheme`, `undefined: KiteIntervals`.

- [ ] **Step 7: Write the UI keys**

```go
// internal/settings/keys_ui.go
package settings

import (
	"errors"
	"strings"
	"time"
)

// KiteIntervals are the eight native Kite candle intervals (chart timeframes).
var KiteIntervals = []string{"minute", "3minute", "5minute", "10minute", "15minute", "30minute", "60minute", "day"}

// ValidateRoute accepts an in-app route: "/" followed by path characters, no whitespace.
func ValidateRoute(v string) error {
	if !strings.HasPrefix(v, "/") || strings.ContainsAny(v, " \t\r\n") {
		return errors.New("must be an in-app route starting with /")
	}
	return nil
}

// ValidateDateOrEmpty accepts "" or a YYYY-MM-DD date.
func ValidateDateOrEmpty(v string) error {
	if v == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", v); err != nil {
		return errors.New("must be a date formatted YYYY-MM-DD")
	}
	return nil
}

// UI settings keys of contract §E (P0-4 A7). Declared through Register so the generic façade lists them.
var (
	AppearanceTheme            = Register(Key[string]{Name: "appearance.theme", Default: "light", Validate: OneOf("light", "dark")})
	AppearanceFollowSystem     = Register(Key[bool]{Name: "appearance.follow_system", Default: false})
	AppearanceSidebarCollapsed = Register(Key[bool]{Name: "appearance.sidebar_collapsed", Default: false})
	UILastRoute                = Register(Key[string]{Name: "ui.last_route", Default: "/home", Validate: ValidateRoute})
	ChartDefaultTimeframe      = Register(Key[string]{Name: "chart.default_timeframe", Default: "60minute", Validate: OneOf(KiteIntervals...)})
	CalendarStaleToastDay      = Register(Key[string]{Name: "calendar.stale_toast_day", Default: "", Validate: ValidateDateOrEmpty})
)
```

- [ ] **Step 8: Run the settings tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/settings/ ./internal/service/ -run 'TestUIKeys|TestRegistry|TestSettingsFacade' -v`
Expected: PASS (`TestUIKeys_DefaultsAndNames`, `TestUIKeys_Validators`, the three `TestRegistry_*` tests, `TestSettingsFacade`); `Settings.All` now lists the six UI keys as well.

- [ ] **Step 9: Commit**

```bash
git add internal/service/settings.go internal/service/settings_test.go internal/settings/keys_ui.go internal/settings/keys_ui_test.go
git commit -m "feat(service,settings): generic settings get/set with credential deny-list and the UI settings keys

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
