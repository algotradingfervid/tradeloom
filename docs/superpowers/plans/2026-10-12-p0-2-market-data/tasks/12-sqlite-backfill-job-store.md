> **P0-2 Time & Market Data Implementation Plan — Task 12.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 12: SQLite backfill job store

**Files:**
- Create: `internal/candles/sqlite/jobs.go`
- Test: `internal/candles/sqlite/jobs_test.go`

**Interfaces:**
- Consumes: Task 10: `candles.Job`, `Window`, `WindowStatus`, `JobProgress`, `JobStore`, `Split`. Task 11: `Store`, `openTest` (test helper).
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
func (s *Store) CreateJob(ctx context.Context, j candles.Job, ws []candles.Window) error
func (s *Store) ActiveWindows(ctx context.Context, token uint32, iv candles.Interval) ([]candles.Window, error)
func (s *Store) RaisePriority(ctx context.Context, token uint32, iv candles.Interval, r candles.Range, p int) error
func (s *Store) BestQueuedPriority(ctx context.Context) (int, bool, error)
func (s *Store) ClaimNext(ctx context.Context) (candles.Window, bool, error)
func (s *Store) FinishWindow(ctx context.Context, jobID string, seq int, st candles.WindowStatus, attempts int, errMsg string) error
func (s *Store) SplitWindow(ctx context.Context, w candles.Window, parts []candles.Range) error
func (s *Store) ResetRunning(ctx context.Context) (int, error)
func (s *Store) Progress(ctx context.Context, id string) (candles.JobProgress, error)
func (s *Store) FinishJob(ctx context.Context, id, status, errMsg string, at time.Time) error
func (s *Store) OpenJobs(ctx context.Context) ([]candles.Job, error)
```

Persisted jobs/windows (AD-23) make backfill resumable (FR-DATA-003 AC3): `ClaimNext` picks by window priority, then job age, then sequence; `ResetRunning` re-queues windows a crash left running; `Progress` counts own plus linked windows; `SplitWindow` supports the halve-on-window-error rule.

- [ ] **Step 1: Write the failing tests**

`internal/candles/sqlite/jobs_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/candles/sqlite/`
Expected: the build fails — output contains `s.CreateJob undefined (type *Store has no field or method CreateJob)` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/candles/sqlite/jobs.go`:

```go
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
)

var _ candles.JobStore = (*Store)(nil)

func ts(t time.Time) int64 { return t.Unix() }
func tm(u int64) time.Time { return time.Unix(u, 0).In(clock.IST) }

func (s *Store) CreateJob(ctx context.Context, j candles.Job, ws []candles.Window) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	oi := 0
	if j.OI {
		oi = 1
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO backfill_jobs(id, token, interval, from_ts, to_ts, requester, priority, oi, status, created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, j.ID, j.Token, string(j.Interval), ts(j.From), ts(j.To), j.Requester, j.Priority, oi, "queued", j.CreatedAt.UnixMilli()); err != nil {
		return err
	}
	for _, w := range ws {
		if w.JobID == j.ID {
			if _, err := tx.ExecContext(ctx, `INSERT INTO backfill_windows(job_id, seq, from_ts, to_ts, priority, status, attempts) VALUES(?,?,?,?,?,?,0)`,
				j.ID, w.Seq, ts(w.From), ts(w.To), w.Priority, string(candles.WindowQueued)); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO backfill_links(job_id, dep_job_id, dep_seq) VALUES(?,?,?)`, j.ID, w.JobID, w.Seq); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const windowCols = `w.job_id, w.seq, j.token, j.interval, w.from_ts, w.to_ts, w.priority, j.oi, w.status, w.attempts, COALESCE(w.error, '')`

func scanWindow(sc interface{ Scan(...any) error }) (candles.Window, error) {
	var w candles.Window
	var iv, st string
	var f, t int64
	var oi int
	err := sc.Scan(&w.JobID, &w.Seq, &w.Token, &iv, &f, &t, &w.Priority, &oi, &st, &w.Attempts, &w.Error)
	w.Interval, w.Status, w.From, w.To, w.OI = candles.Interval(iv), candles.WindowStatus(st), tm(f), tm(t), oi == 1
	return w, err
}

func (s *Store) ActiveWindows(ctx context.Context, token uint32, iv candles.Interval) ([]candles.Window, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+windowCols+` FROM backfill_windows w JOIN backfill_jobs j ON j.id=w.job_id
		WHERE j.token=? AND j.interval=? AND w.status IN ('queued','running') ORDER BY w.from_ts`, token, string(iv))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []candles.Window
	for rows.Next() {
		w, err := scanWindow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) RaisePriority(ctx context.Context, token uint32, iv candles.Interval, r candles.Range, p int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE backfill_windows SET priority=? WHERE priority>? AND status IN ('queued','running')
		AND from_ts<? AND to_ts>? AND job_id IN (SELECT id FROM backfill_jobs WHERE token=? AND interval=?)`,
		p, p, ts(r.To), ts(r.From), token, string(iv))
	return err
}

func (s *Store) BestQueuedPriority(ctx context.Context) (int, bool, error) {
	var p sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(priority) FROM backfill_windows WHERE status='queued'`).Scan(&p); err != nil {
		return 0, false, err
	}
	return int(p.Int64), p.Valid, nil
}

func (s *Store) ClaimNext(ctx context.Context) (candles.Window, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return candles.Window{}, false, err
	}
	defer tx.Rollback()
	w, err := scanWindow(tx.QueryRowContext(ctx, `SELECT `+windowCols+` FROM backfill_windows w JOIN backfill_jobs j ON j.id=w.job_id
		WHERE w.status='queued' ORDER BY w.priority, j.created_at, j.id, w.seq LIMIT 1`))
	if err == sql.ErrNoRows {
		return candles.Window{}, false, nil
	}
	if err != nil {
		return candles.Window{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE backfill_windows SET status='running' WHERE job_id=? AND seq=?`, w.JobID, w.Seq); err != nil {
		return candles.Window{}, false, err
	}
	w.Status = candles.WindowRunning
	return w, true, tx.Commit()
}

func (s *Store) FinishWindow(ctx context.Context, jobID string, seq int, st candles.WindowStatus, attempts int, errMsg string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE backfill_windows SET status=?, attempts=?, error=NULLIF(?, '') WHERE job_id=? AND seq=?`,
		string(st), attempts, errMsg, jobID, seq)
	return err
}

func (s *Store) SplitWindow(ctx context.Context, w candles.Window, parts []candles.Range) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var max int
	if err := tx.QueryRowContext(ctx, `SELECT MAX(seq) FROM backfill_windows WHERE job_id=?`, w.JobID).Scan(&max); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM backfill_windows WHERE job_id=? AND seq=?`, w.JobID, w.Seq); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE backfill_links SET dep_seq=? WHERE dep_job_id=? AND dep_seq=?`, max+1, w.JobID, w.Seq); err != nil {
		return err
	}
	for i, p := range parts {
		if _, err := tx.ExecContext(ctx, `INSERT INTO backfill_windows(job_id, seq, from_ts, to_ts, priority, status, attempts) VALUES(?,?,?,?,?,'queued',?)`,
			w.JobID, max+1+i, ts(p.From), ts(p.To), w.Priority, w.Attempts); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ResetRunning(ctx context.Context) (int, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE backfill_windows SET status='queued' WHERE status='running'`)
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	return int(n), err
}

func (s *Store) Progress(ctx context.Context, id string) (candles.JobProgress, error) {
	var p candles.JobProgress
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(status='done'),0), COALESCE(SUM(status='failed'),0) FROM backfill_windows w
		WHERE w.job_id=? OR EXISTS (SELECT 1 FROM backfill_links l WHERE l.job_id=? AND l.dep_job_id=w.job_id AND l.dep_seq=w.seq)`,
		id, id).Scan(&p.Total, &p.Done, &p.Failed)
	return p, err
}

func (s *Store) FinishJob(ctx context.Context, id, status, errMsg string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE backfill_jobs SET status=?, error=NULLIF(?, ''), finished_at=? WHERE id=?`, status, errMsg, at.UnixMilli(), id)
	return err
}

func (s *Store) OpenJobs(ctx context.Context) ([]candles.Job, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, token, interval, from_ts, to_ts, requester, priority, oi, status, created_at FROM backfill_jobs WHERE status='queued' ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []candles.Job
	for rows.Next() {
		var j candles.Job
		var iv string
		var f, t, c int64
		var oi int
		if err := rows.Scan(&j.ID, &j.Token, &iv, &f, &t, &j.Requester, &j.Priority, &oi, &j.Status, &c); err != nil {
			return nil, err
		}
		j.Interval, j.From, j.To, j.OI, j.CreatedAt = candles.Interval(iv), tm(f), tm(t), oi == 1, time.UnixMilli(c).In(clock.IST)
		out = append(out, j)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/candles/sqlite/`
Expected: `ok  	tradeloom/internal/candles/sqlite`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/candles/sqlite/jobs.go \
  internal/candles/sqlite/jobs_test.go
git commit -m "$(cat <<'EOF'
feat(candles/sqlite): persisted backfill jobs, windows and links

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
