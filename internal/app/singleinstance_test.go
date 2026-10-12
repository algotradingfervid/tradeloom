// internal/app/singleinstance_test.go
package app

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v2/pkg/options"

	"tradeloom/internal/service"
	"tradeloom/internal/service/servicetest"
)

// The lock itself (FR-ACCT-004 AC3, authoritative half) is P0-1's store.AcquireLock inside app.Start:
// TestStart_FirstRunCreatesEverything (boot_test.go) and TestRun_SecondInstanceExits (cmd/tradeloom).

func TestUniqueIDAndDBusNames(t *testing.T) {
	a, b := UniqueID("/home/u/.local/share/tradeloom"), UniqueID("/tmp/other")
	if !strings.HasPrefix(a, "tradeloom-") || len(a) != len("tradeloom-")+12 || a == b || a != UniqueID("/home/u/.local/share/tradeloom") {
		t.Fatalf("%s %s", a, b)
	}
	name, path := dbusNames("tradeloom-0123456789ab")
	if name != "org.wails_app_tradeloom_0123456789ab.SingleInstance" || path != "/org/wails_app_tradeloom_0123456789ab/SingleInstance" {
		t.Fatalf("%s %s", name, path)
	}
}

type recv struct{ ch chan string }

func (r recv) SendMessage(m string) *dbus.Error { r.ch <- m; return nil }

// Runs where a session bus exists (CI: `dbus-run-session -- go test …`, Task 26). It registers a name
// derived from a fresh temp dir, so it never touches a running Tradeloom's name.
func TestForwardToRunningInstance_DeliversArgs(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no session bus")
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Skip("session bus unreachable:", err)
	}
	defer conn.Close()
	uid := UniqueID(t.TempDir())
	name, path := dbusNames(uid)
	r := recv{ch: make(chan string, 1)}
	if err := conn.Export(r, path, name); err != nil {
		t.Fatal(err)
	}
	if reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("request name %v %v", reply, err)
	}
	args := []string{"http://127.0.0.1:7420/kite/callback?request_token=rtok12345678&status=success"}
	if err := ForwardToRunningInstance(uid, args, "/work"); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-r.ch:
		var d options.SecondInstanceData
		if err := json.Unmarshal([]byte(m), &d); err != nil || d.Args[0] != args[0] || d.WorkingDirectory != "/work" {
			t.Fatalf("%q %v", m, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("message not delivered")
	}
}

func TestForwardToRunningInstance_NoBusFails(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent/tradeloom-test-bus")
	if err := ForwardToRunningInstance("tradeloom-0123456789ab", nil, "/"); err == nil {
		t.Fatal("expected an error without a session bus")
	}
}

// Review Focus 5: a redirect URL passed to a second launch completes the login in the running instance.
// Ruling 57 (Task 23 deviation): it goes to the callback-rule path (CompleteForwardedLogin: the state nonce
// of the current attempt is required), never to the paste path.
func TestSecondInstance_RoutesRedirectToLogin(t *testing.T) {
	svc, fs, _ := servicetest.New(t)
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx, fr.rt())
	u := "http://127.0.0.1:7420/kite/callback?request_token=rtok12345678&status=success&state=nonce123"
	a.SecondInstance(options.SecondInstanceData{Args: []string{"--verbose", u}})
	eventually(t, "login routed", func() bool { f := fs.Forwarded(); return len(f) == 1 && f[0] == u })
	a.SecondInstance(options.SecondInstanceData{Args: []string{"--verbose"}})
	if fr.focusCount() != 2 || len(fs.Forwarded()) != 1 || len(fs.Completed()) != 0 {
		t.Fatalf("focus %d forwarded %v completed %v", fr.focusCount(), fs.Forwarded(), fs.Completed())
	}
}

// A second launch with "--view <route>" (desktop shortcuts, P0-4 router) navigates the running window.
func TestSecondInstance_NavigatesToView(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx, fr.rt())
	a.SecondInstance(options.SecondInstanceData{Args: []string{"--view", "/settings/data"}})
	eventually(t, "navigate event", func() bool { return fr.sawEvent(service.EvAppNavigate) })
	fr.mu.Lock()
	nav := fr.payloads[service.EvAppNavigate].(service.Navigate)
	fr.mu.Unlock()
	if nav.Route != "/settings/data" || fr.focusCount() != 1 {
		t.Fatalf("route %q focus %d", nav.Route, fr.focusCount())
	}
	a.SecondInstance(options.SecondInstanceData{Args: []string{"--view"}}) // no route: focus only
	if fr.focusCount() != 2 {
		t.Fatalf("focus %d", fr.focusCount())
	}
}
