package store

import (
	"context"
	"database/sql"
)

// InstrumentRow is one row of the instruments table (§4.2). Expiry is yyyymmdd (0 = none);
// ImportedAt is Unix milliseconds.
type InstrumentRow struct {
	Token          int64
	ExchangeToken  int64
	Tradingsymbol  string
	Name           string
	Exchange       string
	Segment        string
	InstrumentType string
	Expiry         int64
	TickSizePaise  int64
	LotSize        int64
	DisplayName    string
	Underlying     string
	IsIndex        bool
	IsWeekly       bool
	ImportedAt     int64
}

// InstrumentImport is one row of instrument_imports.
type InstrumentImport struct {
	DownloadedAt int64 // Unix ms
	RowCount     int
	KeptCount    int
	Source       string // "kite" | "file"
}

// ReplaceInstruments swaps the whole instrument master in one transaction and rebuilds the FTS
// index (external-content FTS5 must be rebuilt after its content table changes).
func (db *DB) ReplaceInstruments(ctx context.Context, rows []InstrumentRow, imp InstrumentImport) error {
	return db.writeTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM instruments`); err != nil {
			return err
		}
		st, err := tx.PrepareContext(ctx, `INSERT INTO instruments(token, exchange_token, tradingsymbol, name, exchange, segment,
			instrument_type, expiry, tick_size_paise, lot_size, display_name, underlying, is_index, is_weekly, imported_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer st.Close()
		for _, r := range rows {
			if _, err := st.ExecContext(ctx, r.Token, r.ExchangeToken, r.Tradingsymbol, r.Name, r.Exchange, r.Segment,
				r.InstrumentType, r.Expiry, r.TickSizePaise, r.LotSize, r.DisplayName, r.Underlying, r.IsIndex, r.IsWeekly, r.ImportedAt); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO instrument_fts(instrument_fts) VALUES('rebuild')`); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO instrument_imports(downloaded_at, row_count, kept_count, source) VALUES(?,?,?,?)`,
			imp.DownloadedAt, imp.RowCount, imp.KeptCount, imp.Source)
		return err
	})
}

// LoadInstruments returns the whole master ordered by token.
func (db *DB) LoadInstruments(ctx context.Context) ([]InstrumentRow, error) {
	rows, err := db.R.QueryContext(ctx, `SELECT token, COALESCE(exchange_token,0), tradingsymbol, COALESCE(name,''), exchange, segment,
		instrument_type, COALESCE(expiry,0), tick_size_paise, lot_size, display_name, COALESCE(underlying,''), is_index,
		COALESCE(is_weekly,0), imported_at FROM instruments ORDER BY token`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InstrumentRow
	for rows.Next() {
		var r InstrumentRow
		if err := rows.Scan(&r.Token, &r.ExchangeToken, &r.Tradingsymbol, &r.Name, &r.Exchange, &r.Segment, &r.InstrumentType,
			&r.Expiry, &r.TickSizePaise, &r.LotSize, &r.DisplayName, &r.Underlying, &r.IsIndex, &r.IsWeekly, &r.ImportedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LastInstrumentImport returns the newest import, ok=false when none.
func (db *DB) LastInstrumentImport(ctx context.Context) (InstrumentImport, bool, error) {
	var imp InstrumentImport
	err := db.R.QueryRowContext(ctx, `SELECT COALESCE(downloaded_at,0), COALESCE(row_count,0), COALESCE(kept_count,0), COALESCE(source,'') FROM instrument_imports ORDER BY id DESC LIMIT 1`).
		Scan(&imp.DownloadedAt, &imp.RowCount, &imp.KeptCount, &imp.Source)
	if err == sql.ErrNoRows {
		return imp, false, nil
	}
	return imp, err == nil, err
}
