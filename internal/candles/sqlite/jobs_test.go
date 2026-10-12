package sqlite

import (
	"context"
	"testing"
	"time"

	"tradeloom/internal/candles"
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
	if p, ok, _ := s.BestQueuedPriority(ctx); !ok || p != 0 {
		t.Fatalf("best priority = %d %v", p, ok)
	}
	w, ok, err := s.ClaimNext(ctx)
	if err != nil || !ok || w.Seq != 1 || w.Status != candles.WindowRunning || w.Token != 1 {
		t.Fatalf("claim = %+v %v %v, want bt/1 raised to interactive", w, ok, err)
	}
	if pr, _ := s.Progress(ctx, "chart"); pr.Total != 2 || pr.Done != 0 {
		t.Fatalf("chart progress %+v", pr)
	}
	if err := s.SplitWindow(ctx, w, candles.Split(candles.Range{From: w.From, To: w.To}, 5*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if pr, _ := s.Progress(ctx, "bt"); pr.Total != 4 {
		t.Fatalf("after split bt total = %d, want 4", pr.Total)
	}
	w2, _, _ := s.ClaimNext(ctx)
	if n, _ := s.ResetRunning(ctx); n != 1 {
		t.Fatalf("reset = %d", n)
	}
	if err := s.FinishWindow(ctx, w2.JobID, w2.Seq, candles.WindowDone, 1, ""); err != nil {
		t.Fatal(err)
	}
	if pr, _ := s.Progress(ctx, "bt"); pr.Done != 1 {
		t.Fatalf("done = %d", pr.Done)
	}
	jobs, _ := s.OpenJobs(ctx)
	if len(jobs) != 2 || jobs[0].ID != "bt" {
		t.Fatalf("open jobs %+v", jobs)
	}
	if err := s.FinishJob(ctx, "chart", "done", "", ist(2026, 10, 12, 10, 5)); err != nil {
		t.Fatal(err)
	}
	if jobs, _ := s.OpenJobs(ctx); len(jobs) != 1 {
		t.Fatal("finished job still open")
	}
}
