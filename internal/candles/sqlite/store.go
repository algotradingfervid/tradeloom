package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
)

func table(iv candles.Interval) (string, error) {
	t := iv.Table()
	if t == "" {
		return "", fmt.Errorf("candles: %q is not a stored interval", iv)
	}
	return t, nil
}

func (s *Store) Read(ctx context.Context, token uint32, iv candles.Interval, from, to time.Time) ([]candles.Candle, error) {
	t, err := table(iv)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT ts, o, h, l, c, v, oi, src, complete FROM `+t+` WHERE token=? AND ts>=? AND ts<? ORDER BY ts`,
		token, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []candles.Candle
	for rows.Next() {
		var ts, o, h, l, c, v int64
		var oi sql.NullInt64
		var src, complete int
		if err := rows.Scan(&ts, &o, &h, &l, &c, &v, &oi, &src, &complete); err != nil {
			return nil, err
		}
		out = append(out, candles.Candle{
			Start: time.Unix(ts, 0).In(clock.IST), Open: candles.FromPaise(o), High: candles.FromPaise(h),
			Low: candles.FromPaise(l), Close: candles.FromPaise(c), Volume: v, OI: oi.Int64, HasOI: oi.Valid,
			Source: candles.Source(src), Complete: complete == 1,
		})
	}
	return out, rows.Err()
}

func (s *Store) Upsert(ctx context.Context, token uint32, iv candles.Interval, cs []candles.Candle, src candles.Source) error {
	return s.write(ctx, token, iv, nil, cs, src)
}

func (s *Store) UpsertCovered(ctx context.Context, token uint32, iv candles.Interval, window candles.Range, cs []candles.Candle, src candles.Source) error {
	return s.write(ctx, token, iv, &window, cs, src)
}

func (s *Store) write(ctx context.Context, token uint32, iv candles.Interval, window *candles.Range, cs []candles.Candle, src candles.Source) error {
	t, err := table(iv)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := tx.PrepareContext(ctx, `INSERT INTO `+t+`(token, ts, o, h, l, c, v, oi, src, complete) VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(token, ts) DO UPDATE SET o=excluded.o, h=excluded.h, l=excluded.l, c=excluded.c, v=excluded.v,
		oi=excluded.oi, src=excluded.src, complete=excluded.complete
		WHERE NOT (`+t+`.complete = 1 AND (excluded.src = 1 OR excluded.complete = 0))`)
	if err != nil {
		return err
	}
	defer st.Close()
	months := map[int]bool{}
	for _, c := range cs {
		var oi any
		if c.HasOI {
			oi = c.OI
		}
		complete := 0
		if c.Complete {
			complete = 1
			months[monthOf(c.Start)] = true
		}
		if _, err := st.ExecContext(ctx, token, c.Start.Unix(), candles.ToPaise(c.Open), candles.ToPaise(c.High),
			candles.ToPaise(c.Low), candles.ToPaise(c.Close), c.Volume, oi, int(src), complete); err != nil {
			return err
		}
	}
	if window != nil {
		if err := addCoverage(ctx, tx, token, iv, *window); err != nil {
			return err
		}
	}
	ms := make([]int, 0, len(months))
	for m := range months {
		ms = append(ms, m)
	}
	sort.Ints(ms)
	for _, m := range ms {
		if err := recomputeChecksum(ctx, tx, t, token, iv, m, s.now()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) now() time.Time {
	if s.clk == nil {
		return time.Unix(0, 0)
	}
	return s.clk.Now()
}

func monthOf(t time.Time) int {
	y, m, _ := t.In(clock.IST).Date()
	return y*100 + int(m)
}

func monthBounds(month int) (time.Time, time.Time) {
	start := time.Date(month/100, time.Month(month%100), 1, 0, 0, 0, 0, clock.IST)
	return start, start.AddDate(0, 1, 0)
}

// recomputeChecksum hashes the month's complete rows in ts order: per row ts, o, h, l, c, v, oi
// (oi NULL as -1) as big-endian int64 (FR-DATA-002 AC6, FR-BT-044).
func recomputeChecksum(ctx context.Context, tx *sql.Tx, t string, token uint32, iv candles.Interval, month int, now time.Time) error {
	from, to := monthBounds(month)
	rows, err := tx.QueryContext(ctx, `SELECT ts, o, h, l, c, v, oi FROM `+t+` WHERE token=? AND ts>=? AND ts<? AND complete=1 ORDER BY ts`,
		token, from.Unix(), to.Unix())
	if err != nil {
		return err
	}
	h := sha256.New()
	var buf [8]byte
	n := 0
	for rows.Next() {
		var f [6]int64
		var oi sql.NullInt64
		if err := rows.Scan(&f[0], &f[1], &f[2], &f[3], &f[4], &f[5], &oi); err != nil {
			rows.Close()
			return err
		}
		o := int64(-1)
		if oi.Valid {
			o = oi.Int64
		}
		for _, v := range append(f[:], o) {
			binary.BigEndian.PutUint64(buf[:], uint64(v))
			h.Write(buf[:])
		}
		n++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if n == 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM checksums WHERE token=? AND interval=? AND month=?`, token, string(iv), month)
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO checksums(token, interval, month, sha256, rows, updated_at) VALUES(?,?,?,?,?,?)
		ON CONFLICT(token, interval, month) DO UPDATE SET sha256=excluded.sha256, rows=excluded.rows, updated_at=excluded.updated_at`,
		token, string(iv), month, hex.EncodeToString(h.Sum(nil)), n, now.UnixMilli())
	return err
}

func (s *Store) Checksum(ctx context.Context, token uint32, iv candles.Interval, month int) (string, error) {
	var sum string
	err := s.db.QueryRowContext(ctx, `SELECT sha256 FROM checksums WHERE token=? AND interval=? AND month=?`, token, string(iv), month).Scan(&sum)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return sum, err
}

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}

func coverageRanges(ctx context.Context, q querier, token uint32, iv candles.Interval) ([]candles.Range, error) {
	rows, err := q.QueryContext(ctx, `SELECT from_ts, to_ts FROM coverage WHERE token=? AND interval=? ORDER BY from_ts`, token, string(iv))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []candles.Range
	for rows.Next() {
		var f, t int64
		if err := rows.Scan(&f, &t); err != nil {
			return nil, err
		}
		out = append(out, candles.Range{From: time.Unix(f, 0).In(clock.IST), To: time.Unix(t, 0).In(clock.IST)})
	}
	return out, rows.Err()
}

func writeCoverage(ctx context.Context, tx *sql.Tx, token uint32, iv candles.Interval, rs []candles.Range) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM coverage WHERE token=? AND interval=?`, token, string(iv)); err != nil {
		return err
	}
	for _, r := range candles.Merge(rs) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO coverage(token, interval, from_ts, to_ts) VALUES(?,?,?,?)`,
			token, string(iv), r.From.Unix(), r.To.Unix()); err != nil {
			return err
		}
	}
	return nil
}

func addCoverage(ctx context.Context, tx *sql.Tx, token uint32, iv candles.Interval, r candles.Range) error {
	have, err := coverageRanges(ctx, tx, token, iv)
	if err != nil {
		return err
	}
	return writeCoverage(ctx, tx, token, iv, append(have, r))
}

func (s *Store) Coverage(ctx context.Context, token uint32, iv candles.Interval) (candles.Coverage, error) {
	rs, err := coverageRanges(ctx, s.db, token, iv)
	return candles.Coverage{Token: token, Interval: iv, Ranges: rs}, err
}

func (s *Store) Missing(ctx context.Context, token uint32, iv candles.Interval, from, to time.Time) ([]candles.Range, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT from_ts, to_ts FROM coverage WHERE token=? AND interval=? AND to_ts>? AND from_ts<? ORDER BY from_ts`,
		token, string(iv), from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var have []candles.Range
	for rows.Next() {
		var f, t int64
		if err := rows.Scan(&f, &t); err != nil {
			return nil, err
		}
		have = append(have, candles.Range{From: time.Unix(f, 0).In(clock.IST), To: time.Unix(t, 0).In(clock.IST)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candles.Subtract(candles.Range{From: from.In(clock.IST), To: to.In(clock.IST)}, have), nil
}
