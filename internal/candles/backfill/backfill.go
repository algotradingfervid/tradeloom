package backfill

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/logging"
)

// Job is a backfill request.
type Job struct {
	Token     uint32
	Interval  candles.Interval
	From, To  time.Time
	Requester string // "chart", "backtest:<run>", "calendar", …
	Priority  ratelimit.Priority
	OI        bool // futures: fetch open interest (oi=1)
}

// Progress is published on TopicProgress after every window (FR-DATA-003 "reports progress").
type Progress struct {
	JobID    string        `json:"jobId"`
	Token    uint32        `json:"token"`
	Interval string        `json:"interval"`
	Done     int           `json:"done"`
	Failed   int           `json:"failed"`
	Total    int           `json:"total"`
	ETA      time.Duration `json:"etaNs"`
	State    string        `json:"state"` // running | done | failed
}

// TopicProgress is "backfill.progress" (§3.4).
var TopicProgress = bus.NewTopic[Progress]("backfill.progress")

// ErrFailed is returned by Ensure when some windows failed after their retries.
var ErrFailed = errors.New("backfill: some windows failed")

// Options configures Backfill.
type Options struct {
	Store   candles.Store
	Jobs    candles.JobStore
	Fetcher kite.HistoricalPicker
	Clock   clock.Clock
	Bus     *bus.Bus
	Log     *slog.Logger
	Workers int // concurrent fetchers; default 3 (= historical rate)
}

// Backfill is the service.
type Backfill struct {
	o       Options
	submit  sync.Mutex // serialises Submit so overlapping requests see each other's windows
	mu      sync.Mutex
	waiters map[string][]chan Progress
	recent  []time.Time // completion times of the last 20 windows (ETA)
	wake    chan struct{}
	workers sync.WaitGroup
}

// New builds the service; call Start to run workers.
func New(o Options) *Backfill {
	if o.Workers == 0 {
		o.Workers = 3
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	return &Backfill{o: o, waiters: map[string][]chan Progress{}, wake: make(chan struct{}, o.Workers)}
}

// Start re-queues windows a crash left running (done windows are never refetched, FR-DATA-003
// AC3) and starts the workers.
func (b *Backfill) Start(ctx context.Context) error {
	n, err := b.o.Jobs.ResetRunning(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		b.o.Log.Info("backfill resumed", "requeued_windows", n)
	}
	for i := 0; i < b.o.Workers; i++ {
		b.workers.Add(1)
		logging.Go(fmt.Sprintf("backfill.worker.%d", i), func() {
			defer b.workers.Done()
			b.worker(ctx)
		})
	}
	b.pokeAll()
	return nil
}

// WaitStopped blocks until every worker has returned after Start's ctx ended (shutdown: call it
// before closing the candle DB). It returns at once if Start was never called.
func (b *Backfill) WaitStopped() { b.workers.Wait() }

// poke wakes one idle worker.
func (b *Backfill) poke() {
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

func (b *Backfill) pokeAll() {
	for i := 0; i < b.o.Workers; i++ {
		b.poke()
	}
}

// Submit records a job for the parts of [From, To) not yet covered and not already queued by
// another job, reusing (and raising the priority of) overlapping queued windows.
func (b *Backfill) Submit(ctx context.Context, j Job) (string, error) {
	if !j.Interval.IsNative() {
		return "", fmt.Errorf("backfill: %q is not a Kite interval", j.Interval)
	}
	now := b.o.Clock.Now()
	if j.To.After(now) {
		j.To = now
	}
	b.submit.Lock()
	defer b.submit.Unlock()
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	missing, err := b.o.Store.Missing(ctx, j.Token, j.Interval, j.From, j.To)
	if err != nil {
		return "", err
	}
	active, err := b.o.Jobs.ActiveWindows(ctx, j.Token, j.Interval)
	if err != nil {
		return "", err
	}
	var ws []candles.Window
	var activeRanges []candles.Range
	for _, a := range active {
		activeRanges = append(activeRanges, candles.Range{From: a.From, To: a.To})
	}
	for _, m := range missing {
		for _, a := range active {
			if a.From.Before(m.To) && a.To.After(m.From) {
				ws = append(ws, a)
			}
		}
	}
	seq := 0
	for _, m := range missing {
		for _, r := range plan(candles.Subtract(m, activeRanges), j.Interval) {
			ws = append(ws, candles.Window{JobID: id.String(), Seq: seq, From: r.From, To: r.To, Priority: int(j.Priority)})
			seq++
		}
	}
	job := candles.Job{ID: id.String(), Token: j.Token, Interval: j.Interval, From: j.From, To: j.To,
		Requester: j.Requester, Priority: int(j.Priority), OI: j.OI, CreatedAt: now}
	if err := b.o.Jobs.CreateJob(ctx, job, ws); err != nil {
		return "", err
	}
	for _, m := range missing {
		if err := b.o.Jobs.RaisePriority(ctx, j.Token, j.Interval, m, int(j.Priority)); err != nil {
			return "", err
		}
	}
	b.o.Log.Info("backfill job", "id", job.ID, "token", j.Token, "interval", string(j.Interval),
		"requester", j.Requester, "priority", j.Priority.String(), "windows", seq, "reused", len(ws)-seq)
	b.update(ctx, job.ID)
	b.pokeAll()
	return job.ID, nil
}

// Wait blocks until job id is done or failed, or ctx ends.
func (b *Backfill) Wait(ctx context.Context, id string) (Progress, error) {
	ch := make(chan Progress, 1)
	b.mu.Lock()
	b.waiters[id] = append(b.waiters[id], ch)
	b.mu.Unlock()
	b.update(ctx, id) // may already be complete
	select {
	case p := <-ch:
		return p, nil
	case <-ctx.Done():
		return Progress{}, ctx.Err()
	}
}

// Ensure submits j and waits for it; ErrFailed when any window failed.
func (b *Backfill) Ensure(ctx context.Context, j Job) error {
	id, err := b.Submit(ctx, j)
	if err != nil {
		return err
	}
	p, err := b.Wait(ctx, id)
	if err != nil {
		return err
	}
	if p.State == "failed" {
		return ErrFailed
	}
	return nil
}

// DailyCheck returns the calendar inference check (FR-DATA-001): fetch token's day candle for d
// at interactive priority and report whether one exists.
func (b *Backfill) DailyCheck(token uint32) calendar.DailyCandleCheck {
	return func(ctx context.Context, d calendar.Date) (bool, error) {
		from := d.Midnight()
		if err := b.Ensure(ctx, Job{Token: token, Interval: candles.Day, From: from, To: from.AddDate(0, 0, 1), Requester: "calendar", Priority: ratelimit.Interactive}); err != nil {
			return false, err
		}
		cs, err := b.o.Store.Read(ctx, token, candles.Day, from, from.AddDate(0, 0, 1))
		return len(cs) > 0, err
	}
}

func (b *Backfill) worker(ctx context.Context) {
	for ctx.Err() == nil {
		p, ok, err := b.o.Jobs.BestQueuedPriority(ctx)
		if err != nil && ctx.Err() == nil {
			b.o.Log.Error("backfill: best queued priority", "err", err)
		}
		if err != nil || !ok {
			select {
			case <-ctx.Done():
				return
			case <-b.wake:
				continue
			}
		}
		var w candles.Window
		_, data, picked, ferr := b.o.Fetcher.HistoricalPick(ctx, ratelimit.Priority(p), func() (kite.HistoricalRequest, bool) {
			cw, ok, err := b.o.Jobs.ClaimNext(ctx)
			if err != nil && ctx.Err() == nil {
				b.o.Log.Error("backfill: claim window", "err", err)
			}
			if err != nil || !ok {
				return kite.HistoricalRequest{}, false
			}
			w = cw
			b.poke() // let an idle worker queue for the next slot
			// Kite's to is inclusive; windows are half-open.
			return kite.HistoricalRequest{Token: int(cw.Token), Interval: string(cw.Interval), From: cw.From, To: cw.To.Add(-time.Second), OI: cw.OI}, true
		})
		if !picked {
			continue
		}
		b.finish(context.WithoutCancel(ctx), w, data, ferr)
		b.poke() // other workers may be idle while windows remain
	}
}

func (b *Backfill) finish(ctx context.Context, w candles.Window, data []kite.HistoricalData, ferr error) {
	if ferr == nil {
		ferr = b.store(ctx, w, data)
	}
	if ferr == nil {
		b.finishWindow(ctx, w, candles.WindowDone, w.Attempts+1, "")
		b.mu.Lock()
		b.recent = append(b.recent, b.o.Clock.Now())
		if len(b.recent) > 20 {
			b.recent = b.recent[len(b.recent)-20:]
		}
		b.mu.Unlock()
	} else {
		switch kind := kite.Classify(ferr); {
		case kind == kite.KindCanceled || kind == kite.KindSession:
			// shutdown or expired session: back to the queue untouched; the limiter is paused
			// until login, so nothing is fetched meanwhile.
			b.finishWindow(ctx, w, candles.WindowQueued, w.Attempts, "")
		case kind == kite.KindPermission:
			b.finishWindow(ctx, w, candles.WindowFailed, w.Attempts+1, "historical data is not available on this Kite plan")
		case kind == kite.KindInput:
			if parts, ok := halve(candles.Range{From: w.From, To: w.To}); ok {
				b.o.Log.Info("backfill window too large; halving", "job", w.JobID, "seq", w.Seq, "err", ferr.Error())
				if err := b.o.Jobs.SplitWindow(ctx, w, parts); err != nil {
					b.o.Log.Error("backfill: split window", "job", w.JobID, "seq", w.Seq, "err", err)
				}
				break
			}
			b.finishWindow(ctx, w, candles.WindowFailed, w.Attempts+1, ferr.Error())
		default:
			st := candles.WindowQueued
			if w.Attempts+1 >= 3 {
				st = candles.WindowFailed
			}
			b.finishWindow(ctx, w, st, w.Attempts+1, ferr.Error())
		}
	}
	b.updateAll(ctx, w.Token, w.Interval)
}

// store writes a fetched window. Candles that have not ended on the app clock (Kite may return the
// forming candle, OQ-35) are stored incomplete, and coverage stops before the first of them so
// it is fetched again later.
func (b *Backfill) store(ctx context.Context, w candles.Window, data []kite.HistoricalData) error {
	now := b.o.Clock.Now()
	dur := w.Interval.Duration()
	var firstOpen *time.Time
	cs := make([]candles.Candle, 0, len(data))
	for _, d := range data {
		start := d.Date.Time.In(clock.IST)
		complete := !start.Add(dur).After(now)
		if !complete && firstOpen == nil {
			firstOpen = &start
		}
		cs = append(cs, candles.Candle{Start: start, Open: d.Open, High: d.High, Low: d.Low, Close: d.Close,
			Volume: int64(d.Volume), OI: int64(d.OI), HasOI: w.OI, Source: candles.SourceHistorical, Complete: complete})
	}
	coverTo := w.To
	if firstOpen != nil { // Kite returned the forming candle: cover up to its start
		if firstOpen.Before(coverTo) {
			coverTo = *firstOpen
		}
	} else if lim := now.Add(-dur).Add(time.Second); lim.Before(coverTo) {
		coverTo = lim // a candle starting at or after lim cannot have ended yet
	}
	if coverTo.Before(w.From) {
		coverTo = w.From
	}
	return b.o.Store.UpsertCovered(ctx, w.Token, w.Interval, candles.Range{From: w.From, To: coverTo}, cs, candles.SourceHistorical)
}

// finishWindow records a window's new state; a failure is logged, never discarded (the window
// stays running until the next Start re-queues it).
func (b *Backfill) finishWindow(ctx context.Context, w candles.Window, st candles.WindowStatus, attempts int, msg string) {
	if err := b.o.Jobs.FinishWindow(ctx, w.JobID, w.Seq, st, attempts, msg); err != nil {
		b.o.Log.Error("backfill: finish window", "job", w.JobID, "seq", w.Seq, "status", string(st), "err", err)
	}
}

func (b *Backfill) updateAll(ctx context.Context, token uint32, iv candles.Interval) {
	jobs, err := b.o.Jobs.OpenJobs(ctx)
	if err != nil {
		b.o.Log.Error("backfill: list jobs", "err", err)
		return
	}
	for _, j := range jobs {
		if j.Token == token && j.Interval == iv {
			b.update(ctx, j.ID)
		}
	}
}

// update recomputes and publishes job id's progress, closing the job and waking waiters when
// every window it depends on is done or failed.
func (b *Backfill) update(ctx context.Context, id string) {
	pr, err := b.o.Jobs.Progress(ctx, id)
	if err != nil {
		b.o.Log.Error("backfill: job progress", "job", id, "err", err)
		return
	}
	p := Progress{JobID: id, Done: pr.Done, Failed: pr.Failed, Total: pr.Total, State: "running"}
	jobs, err := b.o.Jobs.OpenJobs(ctx)
	if err != nil {
		b.o.Log.Error("backfill: list jobs", "err", err)
	}
	open := false
	for _, j := range jobs {
		if j.ID == id {
			p.Token, p.Interval, open = j.Token, string(j.Interval), true
		}
	}
	if pr.Done+pr.Failed == pr.Total {
		p.State = "done"
		if pr.Failed > 0 {
			p.State = "failed"
		}
		if open {
			if err := b.o.Jobs.FinishJob(ctx, id, p.State, "", b.o.Clock.Now()); err != nil {
				b.o.Log.Error("backfill: finish job", "job", id, "state", p.State, "err", err)
			}
		}
	}
	b.mu.Lock()
	if n := len(b.recent); n >= 2 {
		if span := b.recent[n-1].Sub(b.recent[0]); span > 0 {
			rate := float64(n-1) / span.Seconds()
			p.ETA = time.Duration(float64(pr.Total-pr.Done-pr.Failed) / rate * float64(time.Second))
		}
	}
	var ws []chan Progress
	if p.State != "running" {
		ws = b.waiters[id]
		delete(b.waiters, id)
	}
	b.mu.Unlock()
	for _, ch := range ws {
		ch <- p
	}
	if open || p.State != "running" {
		publish(b.o.Bus, TopicProgress, p)
	}
}

// publish sends v on t when a bus is configured (tests and tools may run without one).
func publish[T any](b *bus.Bus, t bus.Topic[T], v T) {
	if b != nil {
		bus.Publish(b, t, v)
	}
}
