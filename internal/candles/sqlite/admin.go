package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"time"

	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
)

func (s *Store) Stats(ctx context.Context) (candles.Stats, error) {
	var st candles.Stats
	sizes := map[string][2]int64{}
	// Candle tables are WITHOUT ROWID (index B-trees): every cell, interior or leaf, is one row.
	rows, err := s.db.QueryContext(ctx, `SELECT name, SUM(pgsize), SUM(ncell) FROM dbstat GROUP BY name`)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var name string
		var b, n int64
		if err := rows.Scan(&name, &b, &n); err != nil {
			rows.Close()
			return st, err
		}
		sizes[name] = [2]int64{b, n}
		st.TotalBytes += b
	}
	rows.Close()
	for _, iv := range candles.Native {
		v := sizes[iv.Table()]
		st.Intervals = append(st.Intervals, candles.IntervalStats{Interval: iv, Bytes: v[0], Rows: v[1]})
	}
	var b string
	if err := s.db.QueryRowContext(ctx, `SELECT v FROM meta WHERE k='budget_bytes'`).Scan(&b); err != nil {
		return st, err
	}
	st.BudgetBytes, _ = strconv.ParseInt(b, 10, 64)
	return st, nil
}

func (s *Store) SetBudget(ctx context.Context, bytes int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO meta(k, v) VALUES('budget_bytes', ?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, strconv.FormatInt(bytes, 10))
	return err
}

// AddRunRef records the stored interval the run's data comes from: a run on a resampled interval
// (week, month, N-minute) is recorded against its base so purging that table is guarded.
func (s *Store) AddRunRef(ctx context.Context, runID string, token uint32, iv candles.Interval, r candles.Range) error {
	if !iv.IsNative() {
		iv = iv.Base()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO run_refs(run_id, token, interval, from_ts, to_ts) VALUES(?,?,?,?,?)`,
		runID, token, string(iv), r.From.Unix(), r.To.Unix())
	return err
}

func blockedRuns(ctx context.Context, q querier, where string, args []any, confirm []string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT run_id FROM run_refs `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ok := map[string]bool{}
	for _, r := range confirm {
		ok[r] = true
	}
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if !ok[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out, rows.Err()
}

// purgeChunk bounds the rows one purge transaction deletes, so the write lock is released
// between chunks (a test lowers it).
var purgeChunk = 5000

// Purge deletes rule.Interval candles before the first day of rule.Before's month (purges work
// in whole months so monthly checksums stay exact).
//
// Steps: (1) one transaction checks the run guard and trims coverage and checksums, so from then
// on nothing below the cut is reported as fetched; (2) candles are deleted per token by primary
// key range in transactions of at most purgeChunk rows; (3) a final transaction deletes rows a
// concurrent writer added below the cut meanwhile and trims coverage and checksums again, so the
// end state is consistent.
func (s *Store) Purge(ctx context.Context, rule candles.PurgeRule, confirmRuns []string) error {
	t, err := table(rule.Interval)
	if err != nil {
		return err
	}
	y, m, _ := rule.Before.In(clock.IST).Date()
	cut := time.Date(y, m, 1, 0, 0, 0, 0, clock.IST)
	trim := func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM checksums WHERE interval=? AND month<?`, string(rule.Interval), y*100+int(m)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM coverage WHERE interval=? AND to_ts<=?`, string(rule.Interval), cut.Unix()); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE coverage SET from_ts=? WHERE interval=? AND from_ts<?`, cut.Unix(), string(rule.Interval), cut.Unix())
		return err
	}

	// (1) guard + trim.
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		blocked, err := blockedRuns(ctx, tx, `WHERE interval=? AND from_ts<?`, []any{string(rule.Interval), cut.Unix()}, confirmRuns)
		if err != nil {
			return err
		}
		if len(blocked) > 0 {
			return &candles.RunsReferencedError{Runs: blocked}
		}
		return trim(tx)
	})
	if err != nil {
		return err
	}

	// (2) per-token chunks. Tokens are walked through the primary key (one index seek each).
	del := `DELETE FROM ` + t + ` WHERE token=? AND ts IN (SELECT ts FROM ` + t + ` WHERE token=? AND ts<? ORDER BY ts LIMIT ?)`
	for tok, ok, err := s.nextToken(ctx, s.db, t, -1); ok || err != nil; tok, ok, err = s.nextToken(ctx, s.db, t, tok) {
		if err != nil {
			return err
		}
		for {
			var n int64
			err := s.inTx(ctx, func(tx *sql.Tx) error {
				res, err := tx.ExecContext(ctx, del, tok, tok, cut.Unix(), purgeChunk)
				if err != nil {
					return err
				}
				n, err = res.RowsAffected()
				return err
			})
			if err != nil {
				return err
			}
			if n < int64(purgeChunk) {
				break
			}
		}
	}

	// (3) final consistency pass: normally deletes nothing (one index seek per token).
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for tok, ok, err := s.nextToken(ctx, tx, t, -1); ok || err != nil; tok, ok, err = s.nextToken(ctx, tx, t, tok) {
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+t+` WHERE token=? AND ts<?`, tok, cut.Unix()); err != nil {
				return err
			}
		}
		return trim(tx)
	})
}

type rowQuerier interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// nextToken returns the smallest token in table t greater than after.
func (s *Store) nextToken(ctx context.Context, q rowQuerier, t string, after int64) (int64, bool, error) {
	var tok int64
	err := q.QueryRowContext(ctx, `SELECT token FROM `+t+` WHERE token>? ORDER BY token LIMIT 1`, after).Scan(&tok)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return tok, err == nil, err
}

func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Clear empties the candle store and backfill history (Q-A2 decision), with the run guard.
func (s *Store) Clear(ctx context.Context, confirmRuns []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	blocked, err := blockedRuns(ctx, tx, ``, nil, confirmRuns)
	if err != nil {
		return err
	}
	if len(blocked) > 0 {
		return &candles.RunsReferencedError{Runs: blocked}
	}
	for _, iv := range candles.Native {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+iv.Table()); err != nil {
			return err
		}
	}
	for _, q := range []string{`DELETE FROM coverage`, `DELETE FROM checksums`, `DELETE FROM backfill_links`, `DELETE FROM backfill_windows`, `DELETE FROM backfill_jobs`} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return tx.Commit()
}
