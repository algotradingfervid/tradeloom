// internal/app/app_test.go
package app

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/kite"
	"tradeloom/internal/service"
	"tradeloom/internal/service/servicetest"
)

type fakeRuntime struct {
	mu       sync.Mutex
	events   []string
	payloads map[string]any
	focuses  int
	quits    int
	opened   []string
	paths    []string
	pathErr  error
	geom     WindowGeometry
	setGeom  []WindowGeometry
}

func (f *fakeRuntime) rt() Runtime {
	return Runtime{
		Emit: func(n string, p any) {
			f.mu.Lock()
			f.events = append(f.events, n)
			f.payloads[n] = p
			f.mu.Unlock()
		},
		OpenURL: func(u string) { f.mu.Lock(); f.opened = append(f.opened, u); f.mu.Unlock() },
		OpenPath: func(p string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.paths = append(f.paths, p)
			return f.pathErr
		},
		Focus:               func() { f.mu.Lock(); f.focuses++; f.mu.Unlock() },
		Quit:                func() { f.mu.Lock(); f.quits++; f.mu.Unlock() },
		OpenDirectoryDialog: func(string) (string, error) { return "", nil },
		OpenFileDialog:      func(string, []string) (string, error) { return "", nil },
		WindowGeometry: func() (int, int, int, int, bool) {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.geom.W, f.geom.H, f.geom.X, f.geom.Y, f.geom.Maximised
		},
		SetWindowGeometry: func(w, h, x, y int, max bool) {
			f.mu.Lock()
			f.setGeom = append(f.setGeom, WindowGeometry{w, h, x, y, max})
			f.mu.Unlock()
		},
	}
}
func (f *fakeRuntime) focusCount() int { f.mu.Lock(); defer f.mu.Unlock(); return f.focuses }
func (f *fakeRuntime) quitCount() int  { f.mu.Lock(); defer f.mu.Unlock(); return f.quits }
func (f *fakeRuntime) sawEvent(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.payloads[name]
	return ok
}

func (f *fakeRuntime) sessionState() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	st, _ := f.payloads[service.EvSessionState].(service.SessionStatus)
	return st.State
}

func (f *fakeRuntime) jobProgress() service.JobProgress {
	f.mu.Lock()
	defer f.mu.Unlock()
	jp, _ := f.payloads[service.EvJobProgress].(service.JobProgress)
	return jp
}

func newFakeRuntime() *fakeRuntime { return &fakeRuntime{payloads: map[string]any{}} }

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// FR-ACCT-006: "focuses the app" after a successful login, not after a restore.
func TestApp_FocusesWindowOnLoginSuccess(t *testing.T) {
	svc, fs, b := servicetest.New(t)
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx, fr.rt())
	// The session.state source forwards the authoritative Session.State() (Rulings 59, 61), so the fake's
	// state is set before each wake-up and the test waits until the UI saw it.
	moveTo := func(st kite.State) {
		fs.Set(kite.SessionState{State: st})
		bus.Publish(b, kite.TopicSessionState, kite.SessionState{State: st})
		eventually(t, "session.state "+string(st), func() bool { return fr.sessionState() == string(st) })
	}
	moveTo(kite.StateReady)
	time.Sleep(30 * time.Millisecond)
	if fr.focusCount() != 0 {
		t.Fatal("focused on a restore")
	}
	moveTo(kite.StateLoggingIn)
	if fr.focusCount() != 0 {
		t.Fatal("focused when the login started")
	}
	moveTo(kite.StateReady)
	eventually(t, "focus", func() bool { return fr.focusCount() == 1 })
}

func TestErrorFormatter(t *testing.T) {
	var got map[string]string
	if err := json.Unmarshal([]byte(ErrorFormatter(&service.ErrRefused{Code: "c", Stage: "login", Reason: "r"}).(string)), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != "c" || got["stage"] != "login" || got["reason"] != "r" {
		t.Fatalf("%v", got)
	}
	_ = json.Unmarshal([]byte(ErrorFormatter(errors.New("boom")).(string)), &got)
	if got["code"] != "internal" || got["reason"] != "boom" {
		t.Fatalf("%v", got)
	}
}

func TestOptions(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	a := New(Deps{Service: svc})
	assets := fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}
	o := Options(a, assets, "tradeloom-0123456789ab", false)
	if o.SingleInstanceLock == nil || o.SingleInstanceLock.UniqueId != "tradeloom-0123456789ab" || o.SingleInstanceLock.OnSecondInstanceLaunch == nil {
		t.Fatalf("single instance %+v", o.SingleInstanceLock)
	}
	if len(o.Bind) != 11 || o.ErrorFormatter == nil || o.OnStartup == nil || o.OnShutdown == nil || o.OnBeforeClose == nil || o.EnableDefaultContextMenu {
		t.Fatalf("options %+v", o)
	}
	if o.Width != 1440 || o.Height != 900 || o.MinWidth != 1280 || o.MinHeight != 720 {
		t.Fatalf("window %dx%d min %dx%d", o.Width, o.Height, o.MinWidth, o.MinHeight)
	}
	if o.OnDomReady != nil {
		t.Fatal("OnDomReady must be unset without --smoke")
	}
	if Options(a, assets, "tradeloom-0123456789ab", true).OnDomReady == nil {
		t.Fatal("--smoke must quit on DOM ready")
	}
}

func TestRestartRequested(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	a := New(Deps{Service: svc})
	if a.RestartRequested() {
		t.Fatal("fresh app must not request a restart")
	}
	a.RequestRestart()
	if !a.RestartRequested() {
		t.Fatal("RequestRestart not recorded")
	}
}

// FR-SHELL-019 / A6: the window geometry is restored at startup and saved before close.
func TestApp_RestoresAndSavesWindowGeometry(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.SaveWindowGeometry(ctx, `{"w":1500,"h":950,"x":10,"y":20,"maximised":false}`); err != nil {
		t.Fatal(err)
	}
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	a.start(ctx, fr.rt())
	fr.mu.Lock()
	restored := append([]WindowGeometry(nil), fr.setGeom...)
	fr.geom = WindowGeometry{W: 1600, H: 1000, X: 5, Y: 6, Maximised: false} // the maximised case keeps the normal size: TestBeforeClose_MaximisedKeepsNormalSize
	fr.mu.Unlock()
	if len(restored) != 1 || restored[0] != (WindowGeometry{W: 1500, H: 950, X: 10, Y: 20}) {
		t.Fatalf("restored %+v", restored)
	}
	if a.BeforeClose(ctx) {
		t.Fatal("P0 never prevents closing")
	}
	saved, err := svc.GetUiState(ctx, UiStateWindowGeometry)
	if err != nil || saved != `{"w":1600,"h":1000,"x":5,"y":6,"maximised":false}` {
		t.Fatalf("saved %q %v", saved, err)
	}
}

// A5: a finished restore is forwarded to the UI, then the app asks for a restart and quits.
func TestApp_QuitsAfterRestoreJob(t *testing.T) {
	svc, _, b := servicetest.New(t)
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx, fr.rt())
	bus.Publish(b, service.TopicJobProgress, service.JobProgress{JobID: "b1", Kind: "backup", Stage: "done", Finished: true, ResultPath: "/x.tar.zst"})
	eventually(t, "backup event", func() bool { return fr.sawEvent(service.EvJobProgress) })
	if fr.quitCount() != 0 || a.RestartRequested() {
		t.Fatal("a backup must not restart the app")
	}
	bus.Publish(b, service.TopicJobProgress, service.JobProgress{JobID: "r0", Kind: service.JobRestore, Stage: "verify", Done: 1, Total: 4})
	eventually(t, "restore progress", func() bool { return fr.jobProgress().JobID == "r0" })
	if fr.quitCount() != 0 || a.RestartRequested() {
		t.Fatal("an unfinished restore must not restart the app")
	}
	bus.Publish(b, service.TopicJobProgress, service.JobProgress{JobID: "r1", Kind: service.JobRestore, Stage: service.StageRestart, Finished: true})
	eventually(t, "quit", func() bool { return fr.quitCount() == 1 })
	if !a.RestartRequested() {
		t.Fatal("restart not requested")
	}
	fr.mu.Lock()
	jp := fr.payloads[service.EvJobProgress].(service.JobProgress)
	fr.mu.Unlock()
	if jp.Stage != "restart" {
		t.Fatalf("UI did not get the final restore event: %+v", jp)
	}
}

// Ruling 75: a failed restore that reached StageRestart (Error set) also quits for the relaunch,
// because app.db is already closed (or the data directory was swapped back on the next start).
func TestApp_QuitsAfterFailedRestoreJob(t *testing.T) {
	svc, _, b := servicetest.New(t)
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx, fr.rt())
	bus.Publish(b, service.TopicJobProgress, service.JobProgress{JobID: "r1", Kind: service.JobRestore, Stage: service.StageRestart,
		Finished: true, Error: "The restore failed; your previous data will be put back when Tradeloom restarts: boom"})
	eventually(t, "quit", func() bool { return fr.quitCount() == 1 })
	if !a.RestartRequested() {
		t.Fatal("restart not requested after a failed restore")
	}
	if jp := fr.jobProgress(); jp.Error == "" || jp.Stage != service.StageRestart {
		t.Fatalf("UI did not get the failed restore event: %+v", jp)
	}
}
