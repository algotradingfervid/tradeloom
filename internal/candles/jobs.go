package candles

import (
	"context"
	"time"
)

// WindowStatus is a backfill window's state (§4.3 backfill_windows.status).
type WindowStatus string

const (
	WindowQueued  WindowStatus = "queued"
	WindowRunning WindowStatus = "running"
	WindowDone    WindowStatus = "done"
	WindowFailed  WindowStatus = "failed"
)

// Job is a backfill request persisted in backfill_jobs.
type Job struct {
	ID         string
	Token      uint32
	Interval   Interval
	From, To   time.Time
	Requester  string
	Priority   int // ratelimit.Priority value
	OI         bool
	Status     string // queued | done | failed
	CreatedAt  time.Time
	FinishedAt time.Time
	Error      string
}

// Window is one Kite-sized fetch persisted in backfill_windows.
type Window struct {
	JobID    string
	Seq      int
	Token    uint32
	Interval Interval
	From, To time.Time
	Priority int
	OI       bool
	Status   WindowStatus
	Attempts int
	Error    string
}

// JobProgress counts the windows a job depends on: its own plus other jobs' active windows it
// reused (FR-DATA-003 AC5).
type JobProgress struct {
	Done, Failed, Total int
}

// JobStore persists backfill state in the candle DB (AD-23). candles/sqlite implements it.
type JobStore interface {
	CreateJob(ctx context.Context, j Job, ws []Window) error
	// ActiveWindows returns queued/running windows of (token, iv) from any job.
	ActiveWindows(ctx context.Context, token uint32, iv Interval) ([]Window, error)
	// RaisePriority lowers the priority value of active windows of (token, iv) overlapping r to p.
	RaisePriority(ctx context.Context, token uint32, iv Interval, r Range, p int) error
	// BestQueuedPriority returns the smallest priority value among queued windows.
	BestQueuedPriority(ctx context.Context) (int, bool, error)
	// ClaimNext marks the best queued window (priority, then job created_at, then seq) running.
	ClaimNext(ctx context.Context) (Window, bool, error)
	// FinishWindow sets a window's terminal or re-queued status; attempts is the new count.
	FinishWindow(ctx context.Context, jobID string, seq int, st WindowStatus, attempts int, errMsg string) error
	// SplitWindow replaces a window by parts (new seqs appended), all queued.
	SplitWindow(ctx context.Context, w Window, parts []Range) error
	// ResetRunning re-queues windows left running by a crash; returns how many.
	ResetRunning(ctx context.Context) (int, error)
	// Progress counts windows of job id plus active windows of other jobs overlapping its range.
	Progress(ctx context.Context, id string) (JobProgress, error)
	// FinishJob marks a job done or failed.
	FinishJob(ctx context.Context, id, status, errMsg string, at time.Time) error
	// OpenJobs returns jobs whose status is queued (for progress and waiters after restart).
	OpenJobs(ctx context.Context) ([]Job, error)
}
