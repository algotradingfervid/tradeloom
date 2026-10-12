package store

import (
	"context"
	"database/sql"
)

// CalendarOverrideRow is one row of calendar_overrides (§4.2). Fields mirror calendar.Record.
type CalendarOverrideRow struct {
	Exchange string
	Date     int // yyyymmdd
	Type     string
	Phases   string // JSON
	Source   string
}

// CalendarOverrides returns every override ordered by date, exchange.
func (db *DB) CalendarOverrides(ctx context.Context) ([]CalendarOverrideRow, error) {
	rows, err := db.R.QueryContext(ctx,
		`SELECT exchange, date, type, phases, source FROM calendar_overrides ORDER BY date, exchange`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CalendarOverrideRow
	for rows.Next() {
		var r CalendarOverrideRow
		if err := rows.Scan(&r.Exchange, &r.Date, &r.Type, &r.Phases, &r.Source); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpsertCalendarOverrides writes rows in one transaction (an import is all-or-nothing).
func (db *DB) UpsertCalendarOverrides(ctx context.Context, rs []CalendarOverrideRow) error {
	return db.writeTx(ctx, func(tx *sql.Tx) error {
		st, err := tx.PrepareContext(ctx, `INSERT INTO calendar_overrides(exchange, date, type, phases, source)
			VALUES(?,?,?,?,?) ON CONFLICT(exchange, date) DO UPDATE SET type=excluded.type, phases=excluded.phases, source=excluded.source`)
		if err != nil {
			return err
		}
		defer st.Close()
		for _, r := range rs {
			if _, err := st.ExecContext(ctx, r.Exchange, r.Date, r.Type, r.Phases, r.Source); err != nil {
				return err
			}
		}
		return nil
	})
}

// writeTx runs fn in one transaction on the writer connection; rollback on error.
func (db *DB) writeTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// DeleteCalendarOverride removes one override.
func (db *DB) DeleteCalendarOverride(ctx context.Context, exchange string, date int) error {
	_, err := db.W.ExecContext(ctx, `DELETE FROM calendar_overrides WHERE exchange=? AND date=?`, exchange, date)
	return err
}
