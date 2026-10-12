> **P0-3 Session & Service Layer Implementation Plan — Task 26.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 26: Spike S0-8 — single instance end to end and the D-Bus notifications probe

**Files:**
- Create: `internal/notify/desktop/probe.go`, `docs/architecture/spikes/S0-8-single-instance.md`
- Modify: `.github/workflows/ci.yml` (P0-1) — run the Linux Go tests under `dbus-run-session`
- Test: `internal/notify/desktop/probe_test.go`

**Interfaces:**
- Consumes: godbus (Task 23), Task 23's forwarding on top of P0-1's `store.AcquireLock`. The Desktop Notifications Specification method `org.freedesktop.Notifications.GetServerInformation() → (name, vendor, version, spec_version: string)` comes from the freedesktop spec, not from a Go module (see "unverifiable" in the self-review).
- Produces:
  ```go
  package desktop
  type ServerInfo struct{ Name, Vendor, Version, SpecVersion string }
  func Probe(ctx context.Context) (ServerInfo, error) // FR-ACCT-003 AC5 groundwork (P3)
  ```

- [ ] **Step 1: Write the failing test**

```go
// internal/notify/desktop/probe_test.go
package desktop

import (
	"context"
	"os"
	"testing"

	"github.com/godbus/dbus/v5"
)

type fakeServer struct{}

func (fakeServer) GetServerInformation() (string, string, string, string, *dbus.Error) {
	return "fake-notifyd", "tradeloom-tests", "1.0", "1.2", nil
}

func TestProbe_FakeServer(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no session bus")
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Skip("session bus unreachable:", err)
	}
	defer conn.Close()
	if err := conn.Export(fakeServer{}, objPath, busName); err != nil {
		t.Fatal(err)
	}
	if reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Skipf("a real notification daemon owns %s (reply %v, err %v)", busName, reply, err)
	}
	info, err := Probe(context.Background())
	if err != nil || info != (ServerInfo{"fake-notifyd", "tradeloom-tests", "1.0", "1.2"}) {
		t.Fatalf("%+v %v", info, err)
	}
}

func TestProbe_NoBus(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent/tradeloom-test-bus")
	if _, err := Probe(context.Background()); err == nil {
		t.Fatal("expected an error without a session bus")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/notify/desktop/ -v`
Expected: FAIL to compile with `undefined: Probe`.

- [ ] **Step 3: Write the implementation**

```go
// internal/notify/desktop/probe.go
package desktop

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	busName = "org.freedesktop.Notifications"
	objPath = dbus.ObjectPath("/org/freedesktop/Notifications")
)

// ServerInfo is GetServerInformation's reply (Desktop Notifications Specification).
type ServerInfo struct {
	Name        string `json:"name"`
	Vendor      string `json:"vendor"`
	Version     string `json:"version"`
	SpecVersion string `json:"specVersion"`
}

// Probe reports whether a notification daemon answers on the session bus (AD-16: Wails' own check is stubbed true).
func Probe(ctx context.Context) (ServerInfo, error) {
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return ServerInfo{}, fmt.Errorf("session bus: %w", err)
	}
	defer conn.Close()
	var si ServerInfo
	call := conn.Object(busName, objPath).CallWithContext(ctx, busName+".GetServerInformation", 0)
	if call.Err != nil {
		return ServerInfo{}, fmt.Errorf("GetServerInformation: %w", call.Err)
	}
	if err := call.Store(&si.Name, &si.Vendor, &si.Version, &si.SpecVersion); err != nil {
		return ServerInfo{}, err
	}
	return si, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go test -tags webkit2_41,noorders ./internal/notify/desktop/ -v && dbus-run-session -- go test -tags webkit2_41,noorders ./internal/notify/desktop/ ./internal/app/ -run 'TestProbe|TestForwardToRunningInstance' -v`
Expected: the first run shows `TestProbe_FakeServer` SKIP and `TestProbe_NoBus` PASS. The second run (inside the P0-1 Linux container, which has `dbus`) shows `TestProbe_FakeServer`, `TestProbe_NoBus`, `TestForwardToRunningInstance_DeliversArgs` and `TestForwardToRunningInstance_NoBusFails` all PASS.

- [ ] **Step 5: Run the Linux Go test job under a session bus in CI**

In `.github/workflows/ci.yml`, change the Go test step of the Linux container job (P0-1) from `go test …` to:

```yaml
      - name: Go tests (with a private D-Bus session)
        run: dbus-run-session -- go test -tags webkit2_41,noorders -race ./...
```

- [ ] **Step 6: Write `docs/architecture/spikes/S0-8-single-instance.md` and run the manual procedure**

````markdown
# S0-8 — single instance per data directory and D-Bus notifications probe

Decides: the flock (P0-1 `store.AcquireLock` inside `app.Start`) + Wails `SingleInstanceLock` interplay (ARCH §3.7, FR-ACCT-004 AC3) and the notification probe for FR-ACCT-003 (P3).

## Design verified in source (wails v2.16.0)
- `internal/frontend/desktop/linux/frontend.go:295-303`: `OnStartup` starts on a goroutine before `SetupSingleInstance`, so a second process must not call `wails.Run`.
- `internal/frontend/desktop/linux/single_instance.go`: name `org.wails_app_<id>.SingleInstance`, method `SendMessage(string JSON)`; with no session bus it silently continues.
- Therefore: `app.Start` (flock) first, before logging and `app.db`; `store.ErrLocked` → `app.ForwardToRunningInstance` (same D-Bus protocol) → exit 0; no bus or no listener → stderr + exit 1.

## Procedure (Ubuntu 24.04 VM with the noorders .deb; and the P0-1 container with xvfb)
```bash
export TRADELOOM_DATA_DIR=$(mktemp -d)
tradeloom &                       # first instance; wait for the window
tradeloom; echo "second exit=$?"  # expect: exit 0, the first window comes to front
tradeloom 'http://127.0.0.1:7420/kite/callback?request_token=abcd1234efgh&status=success'; echo "exit=$?"
grep -c 'second launch carried a Kite redirect' "$TRADELOOM_DATA_DIR/logs/tradeloom.log"   # expect 1
env -u DBUS_SESSION_BUS_ADDRESS tradeloom; echo "headless exit=$?"   # expect: stderr "Tradeloom is already running (data dir …)", exit 1
TRADELOOM_DATA_DIR=$(mktemp -d) tradeloom & echo "other data dir starts its own instance"  # expect a second window
kill -9 %1; tradeloom &   # expect FR-SHELL-018 AC2 "The app crashed last time" (P0-1 lock marker = running → Boot.CrashedLastTime)
```

## Results
| Step | Expected | Observed | Date |
|---|---|---|---|
| second launch | exit 0, window focused | | |
| deep link | exit 0, login attempted in first instance | | |
| no session bus | stderr message, exit 1 | | |
| other data dir | independent instance | | |
| after kill -9 | crash notice on next start | | |
| `desktop.Probe` on GNOME / KDE / headless | ServerInfo / ServerInfo / error | | |
````

Run the procedure on the VM and fill the Results table. For the last row use `go test -tags webkit2_41,noorders ./internal/notify/desktop/ -run TestProbe -v` in a GNOME session, a KDE session and `env -u DBUS_SESSION_BUS_ADDRESS`. Note: with a real daemon owning the name, `TestProbe_FakeServer` skips by design. To see the real reply there, add a one-off `t.Log(Probe(ctx))`, record it, and remove it again.

- [ ] **Step 7: Commit**

```bash
git add internal/notify/desktop docs/architecture/spikes/S0-8-single-instance.md .github/workflows/ci.yml
git commit -m "spike(S0-8): single-instance verification and notifications probe

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---