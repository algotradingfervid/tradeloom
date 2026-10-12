package app

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/service/servicetest"
)

// waitGroupDone reports whether a.wg reaches zero within d.
func waitGroupDone(a *App, d time.Duration) bool {
	done := make(chan struct{})
	go func() { a.wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// Task 22 minor (Ruling 83 addendum): a start that runs after Shutdown starts nothing: no platform, no
// service goroutines (RunBlock would keep one running forever).
func TestShutdown_BeforeStartStartsNothing(t *testing.T) {
	svc, fs, _ := servicetest.New(t)
	fs.RunBlock = make(chan struct{})
	defer close(fs.RunBlock)
	platform := false
	a := New(Deps{Service: svc, StartPlatform: func(context.Context) error { platform = true; return nil }})
	a.Shutdown(context.Background())
	a.start(context.Background(), newFakeRuntime().rt())
	if platform {
		t.Error("StartPlatform ran after Shutdown")
	}
	if !waitGroupDone(a, 200*time.Millisecond) {
		t.Error("service goroutines started after Shutdown")
	}
}

// Shutdown while StartPlatform is running: StopPlatform waits for StartPlatform to return, and start
// launches no service goroutine afterwards.
func TestShutdown_DuringStartPlatform(t *testing.T) {
	svc, fs, _ := servicetest.New(t)
	fs.RunBlock = make(chan struct{})
	defer close(fs.RunBlock)
	entered, release := make(chan struct{}), make(chan struct{})
	var mu sync.Mutex
	var order []string
	note := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }
	a := New(Deps{Service: svc,
		StartPlatform: func(context.Context) error { close(entered); <-release; note("platform started"); return nil },
		StopPlatform:  func(context.Context) { note("platform stopped") },
	})
	started := make(chan struct{})
	go func() { a.start(context.Background(), newFakeRuntime().rt()); close(started) }()
	<-entered
	stopped := make(chan struct{})
	go func() { a.Shutdown(context.Background()); close(stopped) }()
	select {
	case <-stopped:
		t.Error("Shutdown returned while StartPlatform was still running")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	<-started
	<-stopped
	mu.Lock()
	got := strings.Join(order, ",")
	mu.Unlock()
	if got != "platform started,platform stopped" {
		t.Errorf("order %q", got)
	}
	if !waitGroupDone(a, 200*time.Millisecond) {
		t.Error("service goroutines started after Shutdown")
	}
}

// Shutdown racing start (run with -race): wg.Add never runs concurrently with Shutdown's Wait, and every
// goroutine start launched has exited once Shutdown returned.
func TestShutdown_RacesStart(t *testing.T) {
	old := shutdownWait
	shutdownWait = 2 * time.Second
	defer func() { shutdownWait = old }()
	for i := 0; i < 20; i++ {
		svc, _, _ := servicetest.New(t)
		a := New(Deps{Service: svc})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); a.start(context.Background(), newFakeRuntime().rt()) }()
		go func() { defer wg.Done(); a.Shutdown(context.Background()) }()
		wg.Wait()
		if !waitGroupDone(a, time.Second) {
			t.Fatalf("iteration %d: service goroutines still running after Shutdown", i)
		}
	}
}

// Task 22 minor: the opener's process gets a WaitDelay, so an opener that leaves a child holding its
// output pipe cannot block CombinedOutput past the context deadline.
func TestOpenerCommand_WaitDelay(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		cmd, err := openerCommand(context.Background(), goos, "/data/logs")
		if err != nil {
			t.Fatal(err)
		}
		if cmd.WaitDelay <= 0 {
			t.Errorf("%s: WaitDelay = %v", goos, cmd.WaitDelay)
		}
	}
}

// Task 22 minor: only the logs folder itself or a file directly inside it opens; a file in a
// sub-directory of the logs folder is refused.
func TestOpenPath_OnlyDirectlyInsideLogsDir(t *testing.T) {
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
	sub := filepath.Join(logs, "sub")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(sub, "crash-20261012T090000.json")
	direct := filepath.Join(logs, "crash-20261012T090000.json")
	for _, p := range []string{nested, direct} {
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.openPath(nested); err == nil {
		t.Errorf("%s opened", nested)
	}
	if err := a.openPath(sub); err == nil {
		t.Errorf("%s opened", sub)
	}
	if err := a.openPath(direct); err != nil {
		t.Errorf("%s refused: %v", direct, err)
	}
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if len(fr.paths) != 1 || fr.paths[0] != direct {
		t.Fatalf("opened %v", fr.paths)
	}
}

// Ruling 91 I1: an opener that exits 0 but leaves a child holding the output pipe (xdg-open handing off
// to a file manager) is a success, not open_failed; a non-zero exit is still a failure, with its output.
func TestRunOpener_ChildHoldsPipe(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	var buf syncBuf
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cmd := exec.Command("sh", "-c", "sleep 2 & exit 0")
	cmd.WaitDelay = 100 * time.Millisecond
	start := time.Now()
	if err := runOpener(cmd, log); err != nil {
		t.Fatalf("zero exit with a child holding the pipe: %v", err)
	}
	if d := time.Since(start); d > 1500*time.Millisecond {
		t.Fatalf("waited %v for the child", d)
	}
	if !strings.Contains(buf.String(), "level=DEBUG") {
		t.Errorf("no debug log: %q", buf.String())
	}
	for _, script := range []string{"echo boom >&2; exit 3", "sleep 2 & echo boom >&2; exit 3"} {
		cmd := exec.Command("sh", "-c", script)
		cmd.WaitDelay = 100 * time.Millisecond
		if err := runOpener(cmd, log); err == nil || !strings.Contains(err.Error(), "exit status 3") {
			t.Errorf("%q: err %v", script, err)
		}
	}
	cmd = exec.Command("sh", "-c", "echo boom >&2; exit 3")
	cmd.WaitDelay = 100 * time.Millisecond
	if err := runOpener(cmd, log); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("output not reported: %v", err)
	}
}

// Ruling 91 M5: restoreGeometry (a DB read and a window call) is inside the tracked unit, so Shutdown,
// and the DB close after it, waits for a restore in progress before StopPlatform.
func TestShutdown_DuringRestoreGeometry(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	if err := svc.SaveWindowGeometry(context.Background(), `{"w":1500,"h":950,"x":10,"y":20,"maximised":false}`); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var mu sync.Mutex
	var order []string
	note := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }
	a := New(Deps{Service: svc, StopPlatform: func(context.Context) { note("platform stopped") }})
	rt := newFakeRuntime().rt()
	rt.SetWindowGeometry = func(int, int, int, int, bool) { close(entered); <-release; note("geometry restored") }
	started := make(chan struct{})
	go func() { a.start(context.Background(), rt); close(started) }()
	<-entered
	stopped := make(chan struct{})
	go func() { a.Shutdown(context.Background()); close(stopped) }()
	select {
	case <-stopped:
		t.Error("Shutdown returned while restoreGeometry was still running")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	<-started
	<-stopped
	mu.Lock()
	defer mu.Unlock()
	if got := strings.Join(order, ","); got != "geometry restored,platform stopped" {
		t.Errorf("order %q", got)
	}
}
