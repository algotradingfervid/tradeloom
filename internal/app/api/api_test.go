// internal/app/api/api_test.go
package api

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/kite"
	"tradeloom/internal/service"
	"tradeloom/internal/service/servicetest"
)

func TestSessionLogin_OpensSystemBrowser(t *testing.T) {
	svc, fs, _ := servicetest.New(t)
	var opened []string
	s := NewSession(svc, context.Background, func(u string) { opened = append(opened, u) })
	ls, err := s.Login()
	if err != nil || len(opened) != 1 || opened[0] != fs.LoginURL || ls.LoginURL != fs.LoginURL {
		t.Fatalf("%+v %v %v", ls, err, opened)
	}
	fs.Err = &kite.LoginError{Code: kite.CodeNoCredentials, Message: "Set up first"}
	if _, err := s.Login(); err == nil || len(opened) != 1 {
		t.Fatalf("browser opened on error: %v %v", err, opened)
	}
}

var (
	errorType = reflect.TypeOf((*error)(nil)).Elem()
	timeType  = reflect.TypeOf(time.Time{})
)

func checkType(t *testing.T, where string, ty reflect.Type, seen map[reflect.Type]bool) {
	// No P0-2 pass-through exemption: FeedState/Offset reach the UI only as FeedStateDTO/OffsetDTO (Ruling 67).
	if seen[ty] {
		return
	}
	seen[ty] = true
	switch ty.Kind() {
	case reflect.Interface, reflect.Chan, reflect.Func, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
		t.Errorf("%s: %s is not JSON-friendly", where, ty)
	case reflect.Slice, reflect.Array, reflect.Pointer, reflect.Map:
		checkType(t, where, ty.Elem(), seen)
	case reflect.Struct:
		if ty == timeType {
			t.Errorf("%s: time.Time in a binding (use ms epoch)", where)
			return
		}
		for i := 0; i < ty.NumField(); i++ {
			f := ty.Field(i)
			if !f.IsExported() {
				continue
			}
			if tag := f.Tag.Get("json"); tag == "" || strings.HasPrefix(tag, ",") {
				t.Errorf("%s: %s.%s has no json name", where, ty, f.Name)
			}
			checkType(t, where+"."+f.Name, f.Type, seen)
		}
	}
}

func allBound(t *testing.T) []any {
	t.Helper()
	svc, _, _ := servicetest.New(t)
	noOpen := func(string) {}
	dialogs := Dialogs{Directory: func(string) (string, error) { return "", nil }, File: func(string, []string) (string, error) { return "", nil }}
	return []any{
		NewSession(svc, context.Background, noOpen), NewPortfolio(svc, context.Background), NewSettings(svc, context.Background),
		NewChart(svc, context.Background), NewInstruments(svc, context.Background), NewQuotes(svc, context.Background),
		NewWatchlists(svc, context.Background), NewData(svc, context.Background, dialogs), NewCalendar(svc, context.Background, dialogs),
		NewDiagnostics(svc, context.Background, noOpen), NewSystem(svc, context.Background),
	}
}

// ARCH §8.4: bound methods take/return JSON-friendly structs only (no time.Time outside the P0-2 payloads, no interfaces), error last.
func TestBindings_AreJSONFriendly(t *testing.T) {
	bound := allBound(t)
	if len(bound) != 11 {
		t.Fatalf("%d bound structs, want 11", len(bound))
	}
	for _, b := range bound {
		rt := reflect.TypeOf(b)
		if rt.NumMethod() == 0 {
			t.Fatalf("%s binds nothing", rt)
		}
		for i := 0; i < rt.NumMethod(); i++ {
			m := rt.Method(i)
			where := rt.Elem().Name() + "." + m.Name
			for j := 1; j < m.Type.NumIn(); j++ {
				checkType(t, where+" arg", m.Type.In(j), map[reflect.Type]bool{})
			}
			for j := 0; j < m.Type.NumOut(); j++ {
				out := m.Type.Out(j)
				if out == errorType {
					if j != m.Type.NumOut()-1 {
						t.Errorf("%s: error must be last", where)
					}
					continue
				}
				checkType(t, where+" result", out, map[reflect.Type]bool{})
			}
		}
	}
	_ = service.SessionStatus{}
}

// The dialog bindings pass the runtime's answer through; "" (cancelled) is not an error.
func TestDialogs_PassThrough(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	var titles []string
	dialogs := Dialogs{
		Directory: func(title string) (string, error) { titles = append(titles, title); return "/backups", nil },
		File: func(title string, patterns []string) (string, error) {
			titles = append(titles, title+" "+strings.Join(patterns, ","))
			return "", nil
		},
	}
	d := NewData(svc, context.Background, dialogs)
	if folder, err := d.ChooseFolder(); err != nil || folder != "/backups" {
		t.Fatalf("%q %v", folder, err)
	}
	if file, err := d.ChooseBackupFile(); err != nil || file != "" {
		t.Fatalf("%q %v", file, err)
	}
	c := NewCalendar(svc, context.Background, dialogs)
	if file, err := c.ChooseImportFile(); err != nil || file != "" {
		t.Fatalf("%q %v", file, err)
	}
	if len(titles) != 3 || titles[0] != "Choose a backup folder" || titles[1] != "Choose a backup archive *.tar.zst" || titles[2] != "Choose a calendar CSV *.csv" {
		t.Fatalf("titles %v", titles)
	}
}

// Diagnostics opens the logs folder as a file:// URL through the runtime's browser opener.
func TestDiagnostics_OpensLogsFolder(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	var opened []string
	d := NewDiagnostics(svc, context.Background, func(u string) { opened = append(opened, u) })
	if err := d.OpenLogsFolder(); err != nil {
		t.Fatal(err)
	}
	dir, _ := svc.OpenLogsFolder(context.Background())
	if len(opened) != 1 || opened[0] != "file://"+dir {
		t.Fatalf("opened %v", opened)
	}
	if err := d.OpenCrashReport(); err == nil || len(opened) != 1 {
		t.Fatalf("no crash report: err %v opened %v", err, opened)
	}
}

// The bound surface is exactly the plan's 65 methods: nothing internal (LoadCalendar, RunQuotes, Restore,
// SaveWindowGeometry …) is ever exposed to the frontend.
func TestBindings_ExactMethodSets(t *testing.T) {
	want := map[string]string{
		"Session": "AckRiskDisclosure CancelLogin CompleteLogin Credentials Login Logout MigrateToKeyring Profile ReconnectFeed " +
			"RefuseSecretStorage RemoveCredentials SaveCredentials SecretStorage SetRedirectPort Status Unlock UseEncryptedFile",
		"Portfolio":   "Holdings OrderBook Positions Refresh",
		"Settings":    "All GetUiState Rebind ResetShortcuts Set SetUiState Shortcuts",
		"Chart":       "Candles CloseLive OpenLive Refresh",
		"Instruments": "ByIdentifier ByTokens Search",
		"Quotes":      "Watch",
		"Watchlists":  "Add Create Delete List Remove Rename Reorder ReorderLists SetActive",
		"Data":        "BackupNow ChooseBackupFile ChooseFolder ClearCandles InspectBackup RefreshInstruments Restore Stats",
		"Calendar":    "ChooseImportFile Import Info Remove Upsert",
		"Diagnostics": "CreateSupportBundle DismissCrash LastCrash OpenCrashReport OpenLogsFolder",
		"System":      "Info Status",
	}
	total := 0
	for _, b := range allBound(t) {
		rt := reflect.TypeOf(b)
		var names []string
		for i := 0; i < rt.NumMethod(); i++ {
			names = append(names, rt.Method(i).Name)
		}
		total += len(names)
		if got := strings.Join(names, " "); got != want[rt.Elem().Name()] {
			t.Errorf("%s binds %q, want %q", rt.Elem().Name(), got, want[rt.Elem().Name()])
		}
	}
	if total != 65 {
		t.Fatalf("%d bound methods, want 65", total)
	}
}
