package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestReplaceInstrumentsIsAtomicAndRebuildsFTS(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, filepath.Join(dir, "app.db"), Options{BackupDir: filepath.Join(dir, "backups"), Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := []InstrumentRow{
		{Token: 738561, Tradingsymbol: "RELIANCE", Name: "RELIANCE INDUSTRIES", Exchange: "NSE", Segment: "NSE", InstrumentType: "EQ", TickSizePaise: 10, LotSize: 1, DisplayName: "RELIANCE", ImportedAt: 1},
		{Token: 256265, Tradingsymbol: "NIFTY 50", Exchange: "NSE", Segment: "INDICES", InstrumentType: "EQ", DisplayName: "NIFTY 50", IsIndex: true, ImportedAt: 1},
	}
	if err := db.ReplaceInstruments(ctx, rows, InstrumentImport{DownloadedAt: 1000, RowCount: 90000, KeptCount: 2, Source: "kite"}); err != nil {
		t.Fatal(err)
	}
	got, err := db.LoadInstruments(ctx)
	if err != nil || len(got) != 2 || got[0].Token != 256265 || !got[0].IsIndex {
		t.Fatalf("load = %+v %v", got, err)
	}
	var n int
	if err := db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM instrument_fts WHERE instrument_fts MATCH 'reliance'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("fts match = %d %v", n, err)
	}
	dup := append(rows, rows[0]) // primary-key violation: nothing may change
	if err := db.ReplaceInstruments(ctx, dup, InstrumentImport{DownloadedAt: 2000}); err == nil {
		t.Fatal("duplicate token accepted")
	}
	if got, _ := db.LoadInstruments(ctx); len(got) != 2 {
		t.Fatalf("failed replace changed the table: %d rows", len(got))
	}
	imp, ok, err := db.LastInstrumentImport(ctx)
	if err != nil || !ok || imp.DownloadedAt != 1000 || imp.KeptCount != 2 {
		t.Fatalf("last import = %+v %v %v", imp, ok, err)
	}
}
