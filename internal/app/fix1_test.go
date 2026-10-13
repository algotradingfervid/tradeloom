package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/app/api"
	"tradeloom/internal/bus"
	"tradeloom/internal/logging"
	"tradeloom/internal/service"
	"tradeloom/internal/service/servicetest"
)

// M1: the bound structs are exactly the eleven api types, in the canonical order.
func TestBind_ElevenAPITypes(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	want := []reflect.Type{
		reflect.TypeOf(&api.Session{}), reflect.TypeOf(&api.Portfolio{}), reflect.TypeOf(&api.Settings{}),
		reflect.TypeOf(&api.Chart{}), reflect.TypeOf(&api.Instruments{}), reflect.TypeOf(&api.Quotes{}),
		reflect.TypeOf(&api.Watchlists{}), reflect.TypeOf(&api.Data{}), reflect.TypeOf(&api.Calendar{}),
		reflect.TypeOf(&api.Diagnostics{}), reflect.TypeOf(&api.System{}),
	}
	got := New(Deps{Service: svc}).Bind()
	if len(got) != len(want) {
		t.Fatalf("%d bound, want %d", len(got), len(want))
	}
	for i, b := range got {
		if reflect.TypeOf(b) != want[i] || reflect.ValueOf(b).IsNil() {
			t.Errorf("Bind()[%d] = %T (nil=%v), want %s", i, b, reflect.ValueOf(b).IsNil(), want[i])
		}
	}
}

// I1: the file opener only opens the logs folder or a file inside it; failures reach the caller.
func TestOpenPath_OnlyInsideLogsDir(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	a.start(ctx, fr.rt())
	logs, err := svc.OpenLogsFolder(ctx)
	if err != nil {
		t.Fatal(err)
	}
	crash := filepath.Join(logs, "crash-20261012T090000.json")
	if err := os.WriteFile(crash, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(logs), "app.db")
	_ = os.WriteFile(outside, nil, 0o600)
	if err := os.Symlink(outside, filepath.Join(logs, "crash-link.json")); err != nil {
		t.Fatal(err)
	}
	for _, ok := range []string{logs, crash} {
		if err := a.openPath(ok); err != nil {
			t.Errorf("%s refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "logs/crash.json", logs + "/../app.db", outside, logs + "x", "/etc/passwd",
		filepath.Join(logs, "missing.json"), filepath.Join(logs, "crash-link.json"), logs + "/./crash-20261012T090000.json"} {
		if err := a.openPath(bad); err == nil {
			t.Errorf("%q opened", bad)
		}
	}
	fr.mu.Lock()
	paths := append([]string(nil), fr.paths...)
	fr.pathErr = errors.New("xdg-open: no handler")
	fr.mu.Unlock()
	if len(paths) != 2 || paths[0] != logs || paths[1] != crash {
		t.Fatalf("opened %v", paths)
	}
	if err := a.openPath(logs); err == nil {
		t.Fatal("opener failure was swallowed")
	}
	// The bound Diagnostics method goes through the same opener.
	fr.mu.Lock()
	fr.pathErr = nil
	fr.mu.Unlock()
	if err := a.diagnostics.OpenLogsFolder(); err != nil {
		t.Fatal(err)
	}
}

// I1: BrowserOpenURL is for https URLs (the Kite login page) only.
func TestOpenURL_HTTPSOnly(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	a.start(ctx, fr.rt())
	a.openURL("file:///etc/passwd")
	a.openURL("http://kite.zerodha.com/")
	a.openURL("https://kite.zerodha.com/connect/login?api_key=k&v=3")
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if len(fr.opened) != 1 || !strings.HasPrefix(fr.opened[0], "https://") {
		t.Fatalf("opened %v", fr.opened)
	}
}

func TestOpenerCommand(t *testing.T) {
	for goos, name := range map[string]string{"darwin": "open", "linux": "xdg-open"} {
		cmd, err := openerCommand(context.Background(), goos, "/data/logs")
		if err != nil || len(cmd.Args) != 2 || cmd.Args[0] != name || cmd.Args[1] != "/data/logs" {
			t.Fatalf("%s: %v %v", goos, cmd, err)
		}
	}
	if _, err := openerCommand(context.Background(), "windows", "/data/logs"); err == nil {
		t.Fatal("unsupported OS must refuse")
	}
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

// I2: Shutdown cancels, waits for the service goroutines, then stops the platform.
func TestShutdown_WaitsForGoroutines(t *testing.T) {
	svc, fs, _ := servicetest.New(t)
	fs.RunBlock = make(chan struct{})
	var stopped sync.WaitGroup
	stopped.Add(1)
	var order []string
	var mu sync.Mutex
	note := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }
	a := New(Deps{Service: svc, StopPlatform: func(context.Context) { note("stop"); stopped.Done() }})
	a.start(context.Background(), newFakeRuntime().rt())
	done := make(chan struct{})
	go func() { a.Shutdown(context.Background()); close(done) }()
	select {
	case <-done:
		t.Fatal("Shutdown returned while RunSession was still running")
	case <-time.After(100 * time.Millisecond):
	}
	note("released")
	close(fs.RunBlock)
	<-done
	stopped.Wait()
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(order, ",") != "released,stop" {
		t.Fatalf("order %v", order)
	}
}

// I2: the wait is bounded; a goroutine that never exits is logged and shutdown goes on.
func TestShutdown_WaitIsBounded(t *testing.T) {
	old := shutdownWait
	shutdownWait = 50 * time.Millisecond
	defer func() { shutdownWait = old }()
	svc, fs, _ := servicetest.New(t)
	fs.RunBlock = make(chan struct{})
	defer close(fs.RunBlock)
	var buf syncBuf
	stopped := false
	a := New(Deps{Service: svc, Log: slog.New(slog.NewTextHandler(&buf, nil)), StopPlatform: func(context.Context) { stopped = true }})
	a.start(context.Background(), newFakeRuntime().rt())
	a.Shutdown(context.Background())
	if !stopped || !strings.Contains(buf.String(), "level=WARN") || !strings.Contains(buf.String(), "shutdown") {
		t.Fatalf("stopped=%v log=%q", stopped, buf.String())
	}
}

// Shutdown before Startup (Wails never started the window) is a no-op apart from StopPlatform.
func TestShutdown_WithoutStart(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	stopped := false
	a := New(Deps{Service: svc, StopPlatform: func(context.Context) { stopped = true }})
	a.Shutdown(context.Background())
	if !stopped {
		t.Fatal("StopPlatform not called")
	}
}

// M2: the restored geometry is clamped to the minimum window size and a sane maximum.
func TestRestoreGeometry_Clamped(t *testing.T) {
	for _, tc := range []struct {
		saved string
		want  WindowGeometry
	}{
		{`{"w":100,"h":50,"x":-99999,"y":5,"maximised":false}`, WindowGeometry{W: 1280, H: 720, X: -maxWindowCoord, Y: 5}},
		{`{"w":50000,"h":90000,"x":99999,"y":-3,"maximised":true}`, WindowGeometry{W: maxWindowSize, H: maxWindowSize, X: maxWindowCoord, Y: -3, Maximised: true}},
	} {
		svc, _, _ := servicetest.New(t)
		ctx := context.Background()
		if err := svc.SaveWindowGeometry(ctx, tc.saved); err != nil {
			t.Fatal(err)
		}
		a := New(Deps{Service: svc})
		fr := newFakeRuntime()
		c, cancel := context.WithCancel(ctx)
		a.start(c, fr.rt())
		cancel()
		fr.mu.Lock()
		got := append([]WindowGeometry(nil), fr.setGeom...)
		fr.mu.Unlock()
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("%s → %+v, want %+v", tc.saved, got, tc.want)
		}
	}
}

// M2: while maximised, the saved size and position stay the normal (restored) ones.
func TestBeforeClose_MaximisedKeepsNormalSize(t *testing.T) {
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
	fr.geom = WindowGeometry{W: 2560, H: 1440, X: 0, Y: 0, Maximised: true}
	fr.mu.Unlock()
	a.BeforeClose(ctx)
	if saved, _ := svc.GetUiState(ctx, UiStateWindowGeometry); saved != `{"w":1500,"h":950,"x":10,"y":20,"maximised":true}` {
		t.Fatalf("saved %s", saved)
	}
}

// M3: after a restore asked for a restart, BeforeClose writes nothing (app.db may be closed or replaced).
func TestBeforeClose_SkipsAfterRestartRequest(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	a.start(ctx, fr.rt())
	fr.mu.Lock()
	fr.geom = WindowGeometry{W: 1600, H: 1000, X: 5, Y: 6}
	fr.mu.Unlock()
	a.RequestRestart()
	if a.BeforeClose(ctx) {
		t.Fatal("P0 never prevents closing")
	}
	if saved, _ := svc.GetUiState(ctx, UiStateWindowGeometry); saved != "" {
		t.Fatalf("geometry saved after a restart request: %s", saved)
	}
}

// M4: every reason is redacted, including plain (non-ErrRefused) errors.
func TestErrorFormatter_Redacts(t *testing.T) {
	const secret = "s3cr3t-api-secret-value-0001"
	logging.RegisterSecret(secret)
	for _, err := range []error{errors.New("boom " + secret), &service.ErrRefused{Code: "c", Stage: "s", Reason: "bad " + secret}} {
		if out := ErrorFormatter(err).(string); strings.Contains(out, secret) {
			t.Fatalf("secret leaked: %s", out)
		}
	}
}

// M7: a finished restore that did not reach StageRestart (checks failed, nothing closed) does not quit.
func TestApp_FinishedNonRestartRestoreDoesNotQuit(t *testing.T) {
	svc, _, b := servicetest.New(t)
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx, fr.rt())
	bus.Publish(b, service.TopicJobProgress, service.JobProgress{JobID: "r1", Kind: service.JobRestore, Stage: "verify", Finished: true, Error: "not a Tradeloom backup"})
	eventually(t, "restore event", func() bool { return fr.jobProgress().JobID == "r1" })
	time.Sleep(30 * time.Millisecond)
	if fr.quitCount() != 0 || a.RestartRequested() {
		t.Fatal("a restore that failed its checks must not restart the app")
	}
}
