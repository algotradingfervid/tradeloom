package sqlite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
	"tradeloom/internal/store"
)

func TestNewFileUses8KPagesAndWAL(t *testing.T) {
	s, _ := openTest(t, ist(2026, 10, 12, 10, 0))
	if n, err := s.PageSize(context.Background()); err != nil || n != 8192 {
		t.Fatalf("page_size = %d %v", n, err)
	}
	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q %v", mode, err)
	}
}

func TestUpsertReadRoundTripInPaise(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	day := ist(2026, 10, 12, 9, 15)
	in := []candles.Candle{
		{Start: day, Open: 2450.05, High: 2451.1, Low: 2449.95, Close: 2450.5, Volume: 1200, Complete: true},
		{Start: day.Add(time.Minute), Open: 2450.5, High: 2452, Low: 2450, Close: 2451.35, Volume: 900, OI: 15000, HasOI: true, Complete: true},
	}
	if err := s.Upsert(ctx, 738561, candles.Minute, in, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(ctx, 738561, candles.Minute, in, candles.SourceHistorical); err != nil { // idempotent
		t.Fatal(err)
	}
	got, err := s.Read(ctx, 738561, candles.Minute, day, day.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != in[0] || got[1] != in[1] {
		t.Fatalf("read %+v", got)
	}
	if _, err := s.Read(ctx, 1, candles.Week, day, day); err == nil {
		t.Fatal("Read of a non-native interval must fail")
	}
}

func TestHistoricalReplacesTickBuiltButNotViceVersa(t *testing.T) { // FR-DATA-002 AC2 (store side)
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	at := ist(2026, 10, 12, 10, 0)
	tick := candles.Candle{Start: at, Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 10}
	if err := s.Upsert(ctx, 1, candles.Minute5, []candles.Candle{tick}, candles.SourceTick); err != nil {
		t.Fatal(err)
	}
	hist := candles.Candle{Start: at, Open: 100.05, High: 101.5, Low: 98.9, Close: 100.4, Volume: 12, Complete: true}
	if err := s.Upsert(ctx, 1, candles.Minute5, []candles.Candle{hist}, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(ctx, 1, candles.Minute5, []candles.Candle{tick}, candles.SourceTick); err != nil { // late tick write
		t.Fatal(err)
	}
	got, _ := s.Read(ctx, 1, candles.Minute5, at, at.Add(5*time.Minute))
	if len(got) != 1 || got[0].Close != 100.4 || !got[0].Complete || got[0].Source != candles.SourceHistorical {
		t.Fatalf("stored %+v, want the historical candle", got)
	}
}

func TestCoverageAndMissing(t *testing.T) {
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	r := func(a, b int) candles.Range {
		return candles.Range{From: ist(2026, 1, a, 0, 0), To: ist(2026, 1, b, 0, 0)}
	}
	for _, w := range []candles.Range{r(1, 5), r(10, 15), r(5, 8)} {
		if err := s.UpsertCovered(ctx, 7, candles.Day, w, nil, candles.SourceHistorical); err != nil {
			t.Fatal(err)
		}
	}
	cov, _ := s.Coverage(ctx, 7, candles.Day)
	if len(cov.Ranges) != 2 || cov.Ranges[0] != r(1, 8) || cov.Ranges[1] != r(10, 15) {
		t.Fatalf("coverage %+v", cov.Ranges)
	}
	miss, _ := s.Missing(ctx, 7, candles.Day, ist(2026, 1, 3, 0, 0), ist(2026, 1, 20, 0, 0))
	if len(miss) != 2 || miss[0] != r(8, 10) || miss[1] != r(15, 20) {
		t.Fatalf("missing %+v", miss)
	}
	if err := s.Upsert(ctx, 7, candles.Day, []candles.Candle{{Start: ist(2026, 1, 9, 0, 0), Complete: true}}, candles.SourceTick); err != nil {
		t.Fatal(err)
	}
	if miss2, _ := s.Missing(ctx, 7, candles.Day, ist(2026, 1, 3, 0, 0), ist(2026, 1, 20, 0, 0)); len(miss2) != 2 {
		t.Fatal("plain Upsert must not change coverage")
	}
}

func TestChecksumChangesOnlyForTouchedMonth(t *testing.T) { // FR-DATA-002 AC6
	ctx := context.Background()
	s, _ := openTest(t, ist(2026, 10, 12, 16, 0))
	var sep, oct []candles.Candle
	for i := 0; i < 30; i++ {
		sep = append(sep, minute(ist(2026, 9, 30, 9, 15), i, 100, true))
		oct = append(oct, minute(ist(2026, 10, 1, 9, 15), i, 200, true))
	}
	for _, tok := range []uint32{1, 2} {
		if err := s.Upsert(ctx, tok, candles.Minute, append(sep, oct...), candles.SourceHistorical); err != nil {
			t.Fatal(err)
		}
	}
	before := map[string]string{}
	for _, tok := range []uint32{1, 2} {
		for _, m := range []int{202609, 202610} {
			sum, _ := s.Checksum(ctx, tok, candles.Minute, m)
			if sum == "" {
				t.Fatalf("no checksum for %d/%d", tok, m)
			}
			before[fmt.Sprint(tok, "/", m)] = sum
		}
	}
	if err := s.Upsert(ctx, 1, candles.Minute, []candles.Candle{minute(ist(2026, 10, 1, 9, 15), 30, 201, true)}, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	for _, tok := range []uint32{1, 2} {
		for _, m := range []int{202609, 202610} {
			sum, _ := s.Checksum(ctx, tok, candles.Minute, m)
			changed := sum != before[fmt.Sprint(tok, "/", m)]
			if want := tok == 1 && m == 202610; changed != want {
				t.Fatalf("token %d month %d changed=%v, want %v", tok, m, changed, want)
			}
		}
	}
	// Incomplete (tick-built) rows are not part of the checksum.
	sum0, _ := s.Checksum(ctx, 2, candles.Minute, 202610)
	if err := s.Upsert(ctx, 2, candles.Minute, []candles.Candle{minute(ist(2026, 10, 1, 9, 15), 40, 1, false)}, candles.SourceTick); err != nil {
		t.Fatal(err)
	}
	if sum1, _ := s.Checksum(ctx, 2, candles.Minute, 202610); sum1 != sum0 {
		t.Fatal("tick-built candle changed the checksum")
	}
}

// Controller ruling 6: candles.db and its -wal/-shm sidecars are owner-only, also under a
// data directory whose path contains a space (macOS "Application Support").
func TestFilesAreOwnerOnly(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "Application Support")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "candles.db")
	for round := 0; round < 2; round++ { // new file, then reopen of the existing file
		s, err := Open(ctx, path, Options{Clock: clock.NewFake(ist(2026, 10, 12, 16, 0)), Sessions: noSessions})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Upsert(ctx, 1, candles.Minute, []candles.Candle{minute(ist(2026, 10, 12, 9, 15), 0, 100, true)}, candles.SourceHistorical); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{path, path + "-wal", path + "-shm"} {
			fi, err := os.Stat(p)
			if err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
			if m := fi.Mode().Perm(); m != 0o600 {
				t.Fatalf("round %d: %s mode %o, want 600", round, filepath.Base(p), m)
			}
		}
		if n, _ := s.PageSize(ctx); n != 8192 {
			t.Fatalf("round %d: page_size %d", round, n)
		}
		s.Close()
	}
}

// Controller ruling 16: a damaged existing candles.db is refused before any connection opens
// (a leftover -wal must not be replayed into it) and the file is left untouched.
func TestDamagedFileIsRefusedUntouched(t *testing.T) {
	ctx := context.Background()
	for name, content := range map[string][]byte{
		"not sqlite": bytes.Repeat([]byte("garbage!"), 1024),
		"truncated":  truncatedDB(t),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "candles.db")
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Open(ctx, path, Options{Clock: clock.NewFake(ist(2026, 10, 12, 16, 0)), Sessions: noSessions})
			if !errors.Is(err, store.ErrCorrupt) {
				t.Fatalf("err = %v, want store.ErrCorrupt", err)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(after, content) {
				t.Fatal("refused file was modified")
			}
			for _, p := range []string{path + "-wal", path + "-shm"} {
				if _, err := os.Stat(p); err == nil {
					t.Fatalf("%s created for a refused file", filepath.Base(p))
				}
			}
		})
	}
}

func noSessions(uint32, calendar.Date) (time.Time, time.Time, bool) {
	return time.Time{}, time.Time{}, false
}

// truncatedDB returns a valid candles.db cut to its first page (header declares more pages).
func truncatedDB(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "full.db")
	s, err := Open(context.Background(), path, Options{Clock: clock.NewFake(ist(2026, 10, 12, 16, 0)), Sessions: noSessions})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	b, err := os.ReadFile(path)
	if err != nil || len(b) <= 8192 {
		t.Fatalf("full db %d bytes %v", len(b), err)
	}
	return b[:8192]
}

func TestOpenRequiresSessionsOrCalendar(t *testing.T) {
	if _, err := Open(context.Background(), filepath.Join(t.TempDir(), "candles.db"), Options{}); err == nil {
		t.Fatal("Open without Sessions or Calendar must fail")
	}
}
