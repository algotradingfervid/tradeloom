package sqlite

import (
	"context"
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

func (s *Store) AddRunRef(ctx context.Context, runID string, token uint32, iv candles.Interval, r candles.Range) error {
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

// Purge deletes rule.Interval candles before the first day of rule.Before's month (purges work
// in whole months so monthly checksums stay exact).
func (s *Store) Purge(ctx context.Context, rule candles.PurgeRule, confirmRuns []string) error {
	t, err := table(rule.Interval)
	if err != nil {
		return err
	}
	y, m, _ := rule.Before.In(clock.IST).Date()
	cut := time.Date(y, m, 1, 0, 0, 0, 0, clock.IST)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	blocked, err := blockedRuns(ctx, tx, `WHERE interval=? AND from_ts<?`, []any{string(rule.Interval), cut.Unix()}, confirmRuns)
	if err != nil {
		return err
	}
	if len(blocked) > 0 {
		return &candles.RunsReferencedError{Runs: blocked}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+t+` WHERE ts<?`, cut.Unix()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM checksums WHERE interval=? AND month<?`, string(rule.Interval), y*100+int(m)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM coverage WHERE interval=? AND to_ts<=?`, string(rule.Interval), cut.Unix()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE coverage SET from_ts=? WHERE interval=? AND from_ts<?`, cut.Unix(), string(rule.Interval), cut.Unix()); err != nil {
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
