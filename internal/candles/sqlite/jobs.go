package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
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
	r, err := s.db.ExecContext(ctx, `UPDATE backfill_windows SET status=?, attempts=?, error=NULLIF(?, '') WHERE job_id=? AND seq=?`,
		string(st), attempts, errMsg, jobID, seq)
	if err != nil {
		return err
	}
	return requireRow(r, "window "+jobID+"/"+strconv.Itoa(seq))
}

func (s *Store) SplitWindow(ctx context.Context, w candles.Window, parts []candles.Range) error {
	if len(parts) == 0 {
		return fmt.Errorf("candles: split of window %s/%d: no parts", w.JobID, w.Seq)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var prio, attempts int
	var status string
	err = tx.QueryRowContext(ctx, `SELECT priority, attempts, status FROM backfill_windows WHERE job_id=? AND seq=?`, w.JobID, w.Seq).Scan(&prio, &attempts, &status)
	if err == sql.ErrNoRows {
		return fmt.Errorf("candles: split of window %s/%d: window not found", w.JobID, w.Seq)
	}
	if err != nil {
		return err
	}
	if status != string(candles.WindowRunning) {
		return fmt.Errorf("candles: split of window %s/%d: status is %s, want running", w.JobID, w.Seq, status)
	}
	var max int
	if err := tx.QueryRowContext(ctx, `SELECT MAX(seq) FROM backfill_windows WHERE job_id=?`, w.JobID).Scan(&max); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM backfill_windows WHERE job_id=? AND seq=?`, w.JobID, w.Seq); err != nil {
		return err
	}
	for i, p := range parts {
		if _, err := tx.ExecContext(ctx, `INSERT INTO backfill_windows(job_id, seq, from_ts, to_ts, priority, status, attempts) VALUES(?,?,?,?,?,'queued',?)`,
			w.JobID, max+1+i, ts(p.From), ts(p.To), prio, attempts); err != nil {
			return err
		}
		// Every job that reused the old window now depends on every part.
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO backfill_links(job_id, dep_job_id, dep_seq)
			SELECT job_id, dep_job_id, ? FROM backfill_links WHERE dep_job_id=? AND dep_seq=?`, max+1+i, w.JobID, w.Seq); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM backfill_links WHERE dep_job_id=? AND dep_seq=?`, w.JobID, w.Seq); err != nil {
		return err
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
	r, err := s.db.ExecContext(ctx, `UPDATE backfill_jobs SET status=?, error=NULLIF(?, ''), finished_at=? WHERE id=?`, status, errMsg, at.UnixMilli(), id)
	if err != nil {
		return err
	}
	return requireRow(r, "job "+id)
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

func requireRow(r sql.Result, what string) error {
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("candles: %s not found", what)
	}
	return nil
}
