package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
)

func TestClaimOrderSplitResetProgress(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	r := func(a, b int) candles.Range {
		return candles.Range{From: ist(2026, 1, a, 0, 0), To: ist(2026, 1, b, 0, 0)}
	}
	jb := candles.Job{ID: "bt", Token: 1, Interval: candles.Minute, From: r(1, 30).From, To: r(1, 30).To, Requester: "backtest", Priority: 3, CreatedAt: ist(2026, 10, 12, 10, 0)}
	ws := []candles.Window{
		{JobID: "bt", Seq: 0, From: r(1, 10).From, To: r(1, 10).To, Priority: 3},
		{JobID: "bt", Seq: 1, From: r(10, 20).From, To: r(10, 20).To, Priority: 3},
		{JobID: "bt", Seq: 2, From: r(20, 30).From, To: r(20, 30).To, Priority: 3},
	}
	if err := s.CreateJob(ctx, jb, ws); err != nil {
		t.Fatal(err)
	}
	ji := candles.Job{ID: "chart", Token: 1, Interval: candles.Minute, From: r(15, 30).From, To: r(15, 30).To, Requester: "chart", Priority: 0, CreatedAt: ist(2026, 10, 12, 10, 1)}
	if err := s.CreateJob(ctx, ji, []candles.Window{ws[1], ws[2]}); err != nil { // reuses bt's windows
		t.Fatal(err)
	}
	if err := s.RaisePriority(ctx, 1, candles.Minute, r(15, 30), 0); err != nil {
		t.Fatal(err)
	}
	if p, ok, err := s.BestQueuedPriority(ctx); err != nil || !ok || p != 0 {
		t.Fatalf("best priority = %d %v", p, ok)
	}
	w, ok, err := s.ClaimNext(ctx)
	if err != nil || !ok || w.Seq != 1 || w.Status != candles.WindowRunning || w.Token != 1 {
		t.Fatalf("claim = %+v %v %v, want bt/1 raised to interactive", w, ok, err)
	}
	if pr, err := s.Progress(ctx, "chart"); err != nil || pr.Total != 2 || pr.Done != 0 {
		t.Fatalf("chart progress %+v", pr)
	}
	if err := s.SplitWindow(ctx, w, candles.Split(candles.Range{From: w.From, To: w.To}, 5*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if pr, err := s.Progress(ctx, "bt"); err != nil || pr.Total != 4 {
		t.Fatalf("after split bt total = %d, want 4", pr.Total)
	}
	w2, ok2, err := s.ClaimNext(ctx)
	if err != nil || !ok2 || w2.JobID != "bt" {
		t.Fatalf("second claim = %+v %v %v", w2, ok2, err)
	}
	if n, err := s.ResetRunning(ctx); err != nil || n != 1 {
		t.Fatalf("reset = %d", n)
	}
	if err := s.FinishWindow(ctx, w2.JobID, w2.Seq, candles.WindowDone, 1, ""); err != nil {
		t.Fatal(err)
	}
	if pr, err := s.Progress(ctx, "bt"); err != nil || pr.Done != 1 {
		t.Fatalf("done = %d", pr.Done)
	}
	jobs, err := s.OpenJobs(ctx)
	if err != nil || len(jobs) != 2 || jobs[0].ID != "bt" {
		t.Fatalf("open jobs %+v", jobs)
	}
	if err := s.FinishJob(ctx, "chart", "done", "", ist(2026, 10, 12, 10, 5)); err != nil {
		t.Fatal(err)
	}
	if jobs, err := s.OpenJobs(ctx); err != nil || len(jobs) != 1 {
		t.Fatal("finished job still open")
	}
}

func seedLinked(t *testing.T, s *Store) (candles.Window, []candles.Window) {
	t.Helper()
	ctx := context.Background()
	d := func(a int) time.Time { return ist(2026, 1, a, 0, 0) }
	jb := candles.Job{ID: "own", Token: 1, Interval: candles.Minute, From: d(1), To: d(21), Requester: "backtest", Priority: 3, CreatedAt: ist(2026, 10, 12, 10, 0)}
	ws := []candles.Window{{JobID: "own", Seq: 0, From: d(1), To: d(11), Priority: 3}, {JobID: "own", Seq: 1, From: d(11), To: d(21), Priority: 3}}
	if err := s.CreateJob(ctx, jb, ws); err != nil {
		t.Fatal(err)
	}
	jl := candles.Job{ID: "lnk", Token: 1, Interval: candles.Minute, From: d(1), To: d(11), Requester: "chart", Priority: 0, CreatedAt: ist(2026, 10, 12, 10, 1)}
	if err := s.CreateJob(ctx, jl, ws[:1]); err != nil {
		t.Fatal(err)
	}
	w, ok, err := s.ClaimNext(ctx)
	if err != nil || !ok || w.Seq != 0 {
		t.Fatalf("claim %+v %v %v", w, ok, err)
	}
	return w, ws
}

func TestSplitKeepsLinkedJobsInSync(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	w, _ := seedLinked(t, s)
	parts := candles.Split(candles.Range{From: w.From, To: w.To}, 3*24*time.Hour)
	if len(parts) < 3 {
		t.Fatalf("parts %d", len(parts))
	}
	if err := s.SplitWindow(ctx, w, parts); err != nil {
		t.Fatal(err)
	}
	own, _ := s.Progress(ctx, "own")
	lnk, err := s.Progress(ctx, "lnk")
	if err != nil || lnk.Total != len(parts) || own.Total != len(parts)+1 {
		t.Fatalf("own %+v lnk %+v parts %d", own, lnk, len(parts))
	}
	for i := range parts {
		if err := s.FinishWindow(ctx, "own", 2+i, candles.WindowDone, 1, ""); err != nil {
			t.Fatal(err)
		}
	}
	own, _ = s.Progress(ctx, "own")
	lnk, _ = s.Progress(ctx, "lnk")
	if lnk.Done != len(parts) || own.Done != lnk.Done {
		t.Fatalf("done own %+v lnk %+v", own, lnk)
	}
}

func TestSplitWindowValidation(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	w, _ := seedLinked(t, s)
	r := candles.Range{From: w.From, To: w.To}
	if err := s.SplitWindow(ctx, w, nil); err == nil || !strings.Contains(err.Error(), "no parts") {
		t.Fatalf("empty parts err = %v", err)
	}
	if p, _ := s.Progress(ctx, "own"); p.Total != 2 {
		t.Fatalf("empty split changed windows: %+v", p)
	}
	missing := w
	missing.JobID = "ghost"
	if err := s.SplitWindow(ctx, missing, []candles.Range{r}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing err = %v", err)
	}
	queued := candles.Window{JobID: "own", Seq: 1}
	if err := s.SplitWindow(ctx, queued, []candles.Range{r}); err == nil || !strings.Contains(err.Error(), "want running") {
		t.Fatalf("non-running err = %v", err)
	}
	// A priority raised after the claim survives the split; attempts carry over.
	if err := s.RaisePriority(ctx, 1, candles.Minute, r, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishWindow(ctx, "own", 0, candles.WindowRunning, 2, ""); err != nil {
		t.Fatal(err)
	}
	stale := w // still carries priority 3
	if err := s.SplitWindow(ctx, stale, candles.Split(r, 5*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	aw, err := s.ActiveWindows(ctx, 1, candles.Minute)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, x := range aw {
		if x.Seq >= 2 {
			n++
			if x.Priority != 0 || x.Attempts != 2 {
				t.Fatalf("part %+v, want priority 0 attempts 2", x)
			}
		}
	}
	if n == 0 {
		t.Fatal("no parts found")
	}
}

func TestFinishMissingRowsError(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	if err := s.FinishWindow(ctx, "nope", 0, candles.WindowDone, 1, ""); err == nil {
		t.Fatal("FinishWindow on missing window returned nil")
	}
	if err := s.FinishJob(ctx, "nope", "done", "", ist(2026, 10, 12, 16, 0)); err == nil {
		t.Fatal("FinishJob on missing job returned nil")
	}
}

func TestJobStatePersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "candles.db")
	cal, err := calendar.NewDefault()
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Clock: clock.NewFake(ist(2026, 10, 12, 16, 0)), Calendar: cal}
	s, err := Open(ctx, path, opts)
	if err != nil {
		t.Fatal(err)
	}
	seedLinked(t, s) // claims own/0 -> running
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	jobs, err := s.OpenJobs(ctx)
	if err != nil || len(jobs) != 2 {
		t.Fatalf("open jobs after reopen: %+v %v", jobs, err)
	}
	aw, err := s.ActiveWindows(ctx, 1, candles.Minute)
	if err != nil || len(aw) != 2 || aw[0].Status != candles.WindowRunning || aw[1].Status != candles.WindowQueued {
		t.Fatalf("active after reopen: %+v %v", aw, err)
	}
	if n, err := s.ResetRunning(ctx); err != nil || n != 1 {
		t.Fatalf("reset = %d %v", n, err)
	}
	w, ok, err := s.ClaimNext(ctx)
	if err != nil || !ok || w.JobID != "own" || w.Seq != 0 {
		t.Fatalf("reclaim %+v %v %v", w, ok, err)
	}
}
