package backfill

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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

// ErrUnknownJob is returned by Wait for an id that is neither open nor finished in this process.
var ErrUnknownJob = errors.New("backfill: unknown job")

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
	upd     sync.Mutex // serialises update so a job's events are ordered and its terminal event is unique
	mu      sync.Mutex
	started bool
	waiters map[string][]chan Progress
	final   map[string]Progress // terminal progress of jobs finished in this process (published once)
	failed  map[string]memFail  // jobs failed in memory because their window state could not be written
	recent  []time.Time         // completion times of the last 20 windows (ETA)
	wake    chan struct{}
	workers sync.WaitGroup
}

// memFail records a job failed in memory (Ruling 51 (3)).
type memFail struct {
	token  uint32
	iv     candles.Interval
	reason string
}

// Retry of a failed window-state write: persistTries attempts, back-off persistBackoff doubling,
// on the app clock.
const (
	persistTries   = 3
	persistBackoff = 100 * time.Millisecond
)

// New builds the service; call Start to run workers.
func New(o Options) *Backfill {
	if o.Workers == 0 {
		o.Workers = 3
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	return &Backfill{o: o, waiters: map[string][]chan Progress{}, final: map[string]Progress{},
		failed: map[string]memFail{}, wake: make(chan struct{}, o.Workers)}
}

// Start re-queues windows a crash left running (done windows are never refetched, FR-DATA-003
// AC3) and starts the workers. It is single-use: a second call returns an error.
func (b *Backfill) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.started {
		b.mu.Unlock()
		return errors.New("backfill: already started")
	}
	b.started = true
	b.mu.Unlock()
	n, err := b.o.Jobs.ResetRunning(ctx)
	if err != nil {
		b.mu.Lock()
		b.started = false // no worker runs yet; the caller may retry
		b.mu.Unlock()
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
	// Active windows first: a window that completes between the two reads is then either still
	// listed as active or already covered, never neither (which would refetch it).
	active, err := b.o.Jobs.ActiveWindows(ctx, j.Token, j.Interval)
	if err != nil {
		return "", err
	}
	missing, err := b.o.Store.Missing(ctx, j.Token, j.Interval, j.From, j.To)
	if err != nil {
		return "", err
	}
	if j.OI { // an OI job reuses only OI windows: a non-OI window's candles carry no open interest (Ruling 66)
		oiOnly := active[:0:0]
		for _, a := range active {
			if a.OI {
				oiOnly = append(oiOnly, a)
			}
		}
		active = oiOnly
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

// Wait blocks until job id is done or failed, or ctx ends. An id that is neither open nor
// finished in this process returns ErrUnknownJob.
func (b *Backfill) Wait(ctx context.Context, id string) (Progress, error) {
	ch := make(chan Progress, 1)
	b.mu.Lock()
	if p, ok := b.final[id]; ok {
		b.mu.Unlock()
		return p, nil
	}
	b.waiters[id] = append(b.waiters[id], ch)
	_, memFailed := b.failed[id]
	b.mu.Unlock()
	known := memFailed
	if !known {
		jobs, err := b.o.Jobs.OpenJobs(ctx)
		if err != nil {
			b.dropWaiter(id, ch)
			return Progress{}, err
		}
		for _, j := range jobs {
			known = known || j.ID == id
		}
	}
	if !known {
		b.dropWaiter(id, ch)
		b.mu.Lock()
		p, ok := b.final[id] // finished between the two checks
		b.mu.Unlock()
		if ok {
			return p, nil
		}
		return Progress{}, fmt.Errorf("%w: %s", ErrUnknownJob, id)
	}
	b.update(ctx, id) // may already be complete
	select {
	case p := <-ch:
		return p, nil
	case <-ctx.Done():
		b.dropWaiter(id, ch)
		select {
		case p := <-ch: // delivered while ctx ended
			return p, nil
		default:
		}
		return Progress{}, ctx.Err()
	}
}

func (b *Backfill) dropWaiter(id string, ch chan Progress) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ws := b.waiters[id]
	for i, c := range ws {
		if c == ch {
			ws = append(ws[:i:i], ws[i+1:]...)
			break
		}
	}
	if len(ws) == 0 {
		delete(b.waiters, id)
	} else {
		b.waiters[id] = ws
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
// at interactive priority and report whether one exists. A failed fetch returns an error, never
// "no candle", so Infer does not mark the day closed.
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
	stalled := false // an error is logged once per stall, not per retry
	stall := func(msg string, err error) {
		if !stalled && ctx.Err() == nil {
			b.o.Log.Error(msg+"; waiting for new work", "err", err)
			stalled = true
		}
	}
	idle := func() bool {
		select {
		case <-ctx.Done():
			return false
		case <-b.wake:
			return true
		}
	}
	for ctx.Err() == nil {
		p, ok, err := b.o.Jobs.BestQueuedPriority(ctx)
		if err != nil {
			stall("backfill: best queued priority", err)
		}
		if err != nil || !ok {
			if !idle() {
				return
			}
			continue
		}
		var w candles.Window
		var claimErr error
		_, data, picked, ferr := b.o.Fetcher.HistoricalPick(ctx, ratelimit.Priority(p), func() (kite.HistoricalRequest, bool) {
			cw, ok, err := b.o.Jobs.ClaimNext(ctx)
			if err != nil || !ok {
				claimErr = err
				return kite.HistoricalRequest{}, false
			}
			w = cw
			b.poke() // let an idle worker queue for the next slot
			// Kite's to is inclusive; windows are half-open.
			return kite.HistoricalRequest{Token: int(cw.Token), Interval: string(cw.Interval), From: cw.From, To: cw.To.Add(-time.Second), OI: cw.OI}, true
		})
		if !picked {
			// The limiter refused entry (e.g. ratelimit.ErrStopped) or the claim failed: block
			// until new work or shutdown instead of spinning.
			if err := errors.Join(ferr, claimErr); err != nil && ctx.Err() == nil {
				stall("backfill: cannot fetch", err)
				if !idle() {
					return
				}
			}
			continue
		}
		stalled = false
		b.finish(ctx, w, data, ferr)
		b.poke() // other workers may be idle while windows remain
	}
}

// finish records the outcome of window w. wctx is the worker's context: writes run without its
// cancellation, retries stop when it ends.
func (b *Backfill) finish(wctx context.Context, w candles.Window, data []kite.HistoricalData, ferr error) {
	ctx := context.WithoutCancel(wctx)
	if ferr == nil {
		ferr = b.store(ctx, w, data)
	}
	if ferr == nil {
		b.setWindow(wctx, w, candles.WindowDone, w.Attempts+1, "")
		b.mu.Lock()
		b.recent = append(b.recent, b.o.Clock.Now())
		if len(b.recent) > 20 {
			b.recent = b.recent[len(b.recent)-20:]
		}
		b.mu.Unlock()
		b.updateAll(ctx, w.Token, w.Interval)
		return
	}
	switch kind := kite.Classify(ferr); {
	case kind == kite.KindSession || (kind == kite.KindCanceled && wctx.Err() != nil):
		// expired session, or shutdown: back to the queue untouched; the limiter is paused until
		// login, so nothing is fetched meanwhile. A cancellation while the worker is alive (a
		// per-call deadline) counts as an attempt below.
		b.setWindow(wctx, w, candles.WindowQueued, w.Attempts, "")
	case kind == kite.KindPermission:
		b.setWindow(wctx, w, candles.WindowFailed, w.Attempts+1, "historical data is not available on this Kite plan")
	case kind == kite.KindInput && tooLarge(w, ferr):
		if parts, ok := halve(candles.Range{From: w.From, To: w.To}); ok {
			b.o.Log.Info("backfill window too large; halving", "job", w.JobID, "seq", w.Seq, "err", ferr.Error())
			b.persist(wctx, w, "split window", func() error { return b.o.Jobs.SplitWindow(ctx, w, parts) })
			break
		}
		b.setWindow(wctx, w, candles.WindowFailed, w.Attempts+1, ferr.Error())
	case kind == kite.KindInput:
		// any other input error (bad token, bad interval) will not succeed on retry
		b.setWindow(wctx, w, candles.WindowFailed, w.Attempts+1, ferr.Error())
	default:
		st := candles.WindowQueued
		if w.Attempts+1 >= 3 {
			st = candles.WindowFailed
		}
		b.setWindow(wctx, w, st, w.Attempts+1, ferr.Error())
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

// setWindow records a window's new state through persist.
func (b *Backfill) setWindow(wctx context.Context, w candles.Window, st candles.WindowStatus, attempts int, msg string) {
	b.persist(wctx, w, "finish window ("+string(st)+")", func() error {
		return b.o.Jobs.FinishWindow(context.WithoutCancel(wctx), w.JobID, w.Seq, st, attempts, msg)
	})
}

// persist runs a window-state write, retrying it persistTries times with a doubling back-off on
// the app clock. If every try fails, the window stays "running" in the DB (the next Start
// re-queues it) and every open job depending on it is failed in memory, so its waiters and bus
// subscribers are not left hanging. Shutdown during the back-off stops retrying without failing
// anything: the window is resumed after restart.
func (b *Backfill) persist(wctx context.Context, w candles.Window, what string, write func() error) {
	var err error
	for i := 0; i < persistTries; i++ {
		if i > 0 {
			if clock.Sleep(wctx, b.o.Clock, persistBackoff<<(i-1)) != nil {
				b.o.Log.Error("backfill: "+what+" not recorded before shutdown; resumed at next start",
					"job", w.JobID, "seq", w.Seq, "err", err)
				return
			}
		}
		if err = write(); err == nil {
			return
		}
		b.o.Log.Warn("backfill: "+what+" failed", "job", w.JobID, "seq", w.Seq, "try", i+1, "err", err)
	}
	b.o.Log.Error("backfill: "+what+" failed after retries; failing dependent jobs", "job", w.JobID, "seq", w.Seq, "err", err)
	b.failInMemory(context.WithoutCancel(wctx), w, fmt.Sprintf("%s: %v", what, err))
}

// failInMemory fails w's job and every open job of the same series whose range overlaps w (the
// jobs that may have reused it), then publishes their terminal progress.
func (b *Backfill) failInMemory(ctx context.Context, w candles.Window, reason string) {
	ids := []string{w.JobID}
	jobs, err := b.o.Jobs.OpenJobs(ctx)
	if err != nil {
		b.o.Log.Error("backfill: list jobs", "err", err)
	}
	for _, j := range jobs {
		if j.ID != w.JobID && j.Token == w.Token && j.Interval == w.Interval && j.From.Before(w.To) && w.From.Before(j.To) {
			ids = append(ids, j.ID)
		}
	}
	b.mu.Lock()
	for _, id := range ids {
		if _, done := b.final[id]; !done {
			b.failed[id] = memFail{token: w.Token, iv: w.Interval, reason: reason}
		}
	}
	b.mu.Unlock()
	for _, id := range ids {
		b.update(ctx, id)
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
// every window it depends on is done or failed (or it was failed in memory). The terminal event
// is published exactly once per job; a job already final is not published again.
func (b *Backfill) update(ctx context.Context, id string) {
	b.upd.Lock()
	defer b.upd.Unlock()
	b.mu.Lock()
	fin, isFinal := b.final[id]
	mf, memFailed := b.failed[id]
	b.mu.Unlock()
	if isFinal {
		b.release(id, fin)
		return
	}
	p := Progress{JobID: id, State: "running"}
	if memFailed {
		p.Token, p.Interval = mf.token, string(mf.iv)
	}
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
	if !open && !memFailed {
		return // not a job this process can report on
	}
	pr, err := b.o.Jobs.Progress(ctx, id)
	if err != nil {
		b.o.Log.Error("backfill: job progress", "job", id, "err", err)
		if !memFailed {
			return
		}
	}
	p.Done, p.Failed, p.Total = pr.Done, pr.Failed, pr.Total
	errMsg := ""
	switch {
	case memFailed:
		p.State, errMsg = "failed", mf.reason
	case pr.Done+pr.Failed == pr.Total:
		p.State = "done"
		if pr.Failed > 0 {
			p.State = "failed"
		}
	}
	b.mu.Lock()
	if n := len(b.recent); n >= 2 && p.State == "running" {
		if span := b.recent[n-1].Sub(b.recent[0]); span > 0 {
			rate := float64(n-1) / span.Seconds()
			p.ETA = time.Duration(float64(pr.Total-pr.Done-pr.Failed) / rate * float64(time.Second))
		}
	}
	b.mu.Unlock()
	if p.State == "running" {
		publish(b.o.Bus, TopicProgress, p)
		return
	}
	if open {
		if err := b.o.Jobs.FinishJob(ctx, id, p.State, errMsg, b.o.Clock.Now()); err != nil {
			b.o.Log.Error("backfill: finish job", "job", id, "state", p.State, "err", err)
		}
	}
	b.mu.Lock()
	b.final[id] = p
	delete(b.failed, id)
	b.mu.Unlock()
	publish(b.o.Bus, TopicProgress, p)
	b.release(id, p)
}

// release hands p to every waiter of job id.
func (b *Backfill) release(id string, p Progress) {
	b.mu.Lock()
	ws := b.waiters[id]
	delete(b.waiters, id)
	b.mu.Unlock()
	for _, ch := range ws {
		ch <- p
	}
}

// publish sends v on t when a bus is configured (tests and tools may run without one).
func publish[T any](b *bus.Bus, t bus.Topic[T], v T) {
	if b != nil {
		bus.Publish(b, t, v)
	}
}

// tooLarge reports whether a KindInput error on w means the window must shrink: w exceeds Kite's
// window for its interval, or Kite said so ("interval exceeds max limit: N days"; older replies
// read "interval exceeds limit: N days").
func tooLarge(w candles.Window, err error) bool {
	if max, ok := MaxWindow[w.Interval]; ok && w.To.Sub(w.From) > max {
		return true
	}
	msg := err.Error()
	var ke *kite.Err
	if errors.As(err, &ke) {
		msg = ke.Message
	}
	msg = strings.ToLower(msg)
	return strings.Contains(msg, "interval exceeds") && strings.Contains(msg, "limit")
}
