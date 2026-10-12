> **P0-2 Time & Market Data Implementation Plan — Task 15.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 15: Instrument master: scope filter, display names, tags, lookups and search

**Files:**
- Create: `internal/store/instruments.go`
- Create: `internal/instruments/instrument.go`
- Create: `internal/instruments/filter.go`
- Create: `internal/instruments/aliases.go`
- Create: `internal/instruments/master.go`
- Create: `internal/instruments/search.go`
- Create: `internal/instruments/testdata/dump.csv`
- Test: `internal/store/instruments_test.go`
- Test: `internal/instruments/fixture_test.go`
- Test: `internal/instruments/filter_test.go`
- Test: `internal/instruments/search_test.go`

**Interfaces:**
- Consumes: Task 1: `clock.Clock`, `clock.Fake`, `clock.MidnightIST`. Task 3/4: `calendar.Date`, `DateOf`, `DateFromInt`, `Exchange`, `NSE`. Task 6: `ratelimit.New`. Task 7/9: `kite.Instrument`, `kite.Instruments`, `kite.New`, `(*kite.Client).Instruments`. Task 8: `mock.NewServer`, `SetInstrumentsCSV`. P0-1: `store.Open`, `store.Options`, `DB.W`, `DB.R`, the v1 `instruments`, `instrument_imports`, `instrument_fts` tables. Task 5: `(*store.DB).writeTx`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
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
type InstrumentImport struct {
	DownloadedAt int64 // Unix ms
	RowCount     int
	KeptCount    int
	Source       string // "kite" | "file"
}
func (db *DB) ReplaceInstruments(ctx context.Context, rows []InstrumentRow, imp InstrumentImport) error
func (db *DB) LoadInstruments(ctx context.Context) ([]InstrumentRow, error)
func (db *DB) LastInstrumentImport(ctx context.Context) (InstrumentImport, bool, error)
type Tag string
const (
	TagIndices Tag = "INDICES"
	TagNSE     Tag = "NSE"
	TagBSE     Tag = "BSE"
	TagNFOFut  Tag = "NFO-FUT"
	TagBFOFut  Tag = "BFO-FUT"
)
type Instrument struct {
	Token         uint32    `json:"token"`
	ExchangeToken uint32    `json:"exchangeToken"`
	Exchange      string    `json:"exchange"` // Kite exchange: NSE, BSE, NFO, BFO
	Segment       string    `json:"segment"`
	Tag           Tag       `json:"tag"`
	Tradingsymbol string    `json:"tradingsymbol"`
	Name          string    `json:"name"`
	Type          string    `json:"instrumentType"` // EQ or FUT
	Expiry        time.Time `json:"expiry"`         // zero unless a future; 00:00 IST of the expiry date
	TickSizePaise int64     `json:"tickSizePaise"`
	LotSize       int64     `json:"lotSize"`
	DisplayName   string    `json:"displayName"`
	Underlying    string    `json:"underlying"` // futures: the dump's name field
	IsIndex       bool      `json:"isIndex"`
	IsWeekly      bool      `json:"isWeekly"`
}
func (i Instrument) ID() string
func (i Instrument) IsFuture() bool
func (i Instrument) CalendarExchange() calendar.Exchange
func UnitsForLots(lots int64, in Instrument) int64
func Filter(rows kite.Instruments, today calendar.Date) []Instrument
type Master struct{ /* unexported fields */ }
func NewMaster(clk clock.Clock) *Master
func (m *Master) Replace(ins []Instrument, importedAt time.Time)
func (m *Master) ImportedAt() time.Time
func (m *Master) Len() int
func (m *Master) ByToken(token uint32) (Instrument, bool)
func (m *Master) ByID(id string) (Instrument, bool)
func (m *Master) ExchangeOf(token uint32) calendar.Exchange
func (m *Master) Futures(segment, underlying string) []Instrument
const (
	RankExactSymbol   = 0
	RankSymbolPrefix  = 1
	RankDisplayTokens = 2
	RankNameContains  = 3
)
type Result struct {
	Instrument Instrument `json:"instrument"`
	Rank       int        `json:"rank"`
	AlsoOnNSE  bool       `json:"alsoOnNse"` // BSE row whose company is also listed on NSE
}
func (m *Master) Search(q string, limit int) []Result
```

NFR-15 scope guard at import; FR-WATCH-013 one formatter (`UNDERLYING MON FUT`, `UNDERLYING DD MON YYYY FUT` on a month collision, indices by name) and `EXCHANGE:TRADINGSYMBOL` ids; FR-WATCH-014 tag derived from the instrument's own record; FR-WATCH-008 ranking (exact symbol > prefix > all display tokens > name contains; indices/equities before futures; NSE before BSE with the "also on NSE" flag; nearest expiry first), expiry filtering at search time, < 100 ms, lot size from the latest dump. The fixture is read through gokiteconnect's real CSV parser via the mock server. Search ranks over the in-memory master instead of FTS candidates (ARCHITECTURE §4.2 note): FTS5 prefix matching cannot express the "BANKNIFTY" → NIFTY BANK alias or "name contains"; the FTS index is still rebuilt on every import so it stays consistent for later consumers.

- [ ] **Step 1: Write the failing tests**

`internal/store/instruments_test.go`:

```go
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
```

`internal/instruments/fixture_test.go`:

```go
package instruments

import (
	"context"
	"os"
	"testing"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/mock"
	"tradeloom/internal/kite/ratelimit"
)

func ist(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, clock.IST)
}

// kiteFixture serves body (default testdata/dump.csv) from a mock Kite and returns a client
// whose app clock reads now. The dump goes through gokiteconnect's real CSV parser.
func kiteFixture(t *testing.T, now time.Time, body []byte) (*kite.Client, *mock.Server, *clock.Fake) {
	t.Helper()
	if body == nil {
		var err error
		if body, err = os.ReadFile("testdata/dump.csv"); err != nil {
			t.Fatal(err)
		}
	}
	srv := mock.NewServer()
	t.Cleanup(srv.Close)
	srv.SetInstrumentsCSV(body)
	fc := clock.NewFake(now)
	lim := ratelimit.New(fc, ratelimit.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	lim.Run(ctx)
	c := kite.New(kite.Options{APIKey: "k", BaseURI: srv.URL, Limiter: lim, Clock: fc})
	c.SetAccessToken("t")
	return c, srv, fc
}

// loadMaster imports the fixture dump into a fresh Master at now.
func loadMaster(t *testing.T, now time.Time, body []byte) (*Master, *clock.Fake) {
	t.Helper()
	c, _, fc := kiteFixture(t, now, body)
	rows, err := c.Instruments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m := NewMaster(fc)
	m.Replace(Filter(rows, calendar.DateOf(now)), now)
	return m, fc
}

func ids(rs []Result) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Instrument.ID())
	}
	return out
}
```

`internal/instruments/filter_test.go`:

```go
package instruments

import (
	"bytes"
	"os"
	"testing"
)

func TestImportKeepsOnlyScopeAndDropsExpired(t *testing.T) { // NFR-15, FR-WATCH-008 AC5 (import)
	m, _ := loadMaster(t, ist(2026, 10, 12, 9, 0), nil)
	if m.Len() != 17 {
		t.Fatalf("kept %d, want 17 (3 indices, 5 equities, 9 live futures)", m.Len())
	}
	for _, tok := range []uint32{12347138, 12347394, 209876549, 53505799, 1373186, 13368578} {
		if _, ok := m.ByToken(tok); ok {
			t.Fatalf("token %d (option/MCX/CDS/expired) survived import", tok)
		}
	}
}

func TestDisplayNamesAndTags(t *testing.T) { // FR-WATCH-013 AC1/AC2, FR-WATCH-014
	m, _ := loadMaster(t, ist(2026, 10, 12, 9, 0), nil)
	cases := []struct {
		id, display string
		tag         Tag
	}{
		{"NFO:RELIANCE26OCTFUT", "RELIANCE OCT FUT", TagNFOFut},
		{"NFO:RELIANCE26NOVFUT", "RELIANCE NOV FUT", TagNFOFut},
		{"NSE:NIFTY 50", "NIFTY 50", TagIndices},
		{"BSE:SENSEX", "SENSEX", TagIndices},
		{"NSE:RELIANCE", "RELIANCE", TagNSE},
		{"BSE:RELIANCE", "RELIANCE", TagBSE},
		{"BFO:SENSEX26OCTFUT", "SENSEX OCT FUT", TagBFOFut},
		{"NFO:TWINCO26OCT13FUT", "TWINCO 13 OCT 2026 FUT", TagNFOFut},
		{"NFO:TWINCO26OCTFUT", "TWINCO 27 OCT 2026 FUT", TagNFOFut},
	}
	for _, c := range cases {
		in, ok := m.ByID(c.id)
		if !ok || in.DisplayName != c.display || in.Tag != c.tag || in.ID() != c.id {
			t.Errorf("%s: %+v (ok=%v), want %q %s", c.id, in, ok, c.display, c.tag)
		}
	}
	tw, _ := m.ByID("NFO:TWINCO26OCT13FUT")
	ro, _ := m.ByID("NFO:RELIANCE26OCTFUT")
	if !tw.IsWeekly || ro.IsWeekly || !ro.IsFuture() || ro.LotSize != 500 || ro.TickSizePaise != 10 || ro.Underlying != "RELIANCE" {
		t.Fatalf("futures metadata: twinco weekly=%v reliance %+v", tw.IsWeekly, ro)
	}
	// FR-WATCH-014 AC1/AC2: one instrument carries one tag wherever it is found.
	seen := map[uint32]Tag{}
	for _, q := range []string{"reliance", "RELIANCE", "reliance industries", "rel", "nifty", "sensex"} {
		for _, r := range m.Search(q, 0) {
			if t0, ok := seen[r.Instrument.Token]; ok && t0 != r.Instrument.Tag {
				t.Fatalf("token %d tagged %s and %s", r.Instrument.Token, t0, r.Instrument.Tag)
			}
			seen[r.Instrument.Token] = r.Instrument.Tag
			if bt, _ := m.ByToken(r.Instrument.Token); bt.Tag != r.Instrument.Tag {
				t.Fatal("lookup and search disagree on tag")
			}
		}
	}
}

func TestLotSizeChangeFollowsNewDump(t *testing.T) { // FR-WATCH-008 AC6 (master side)
	m, _ := loadMaster(t, ist(2026, 10, 12, 9, 0), nil)
	nf, _ := m.ByID("NFO:NIFTY26OCTFUT")
	if nf.LotSize != 65 || UnitsForLots(2, nf) != 130 {
		t.Fatalf("before: %d", nf.LotSize)
	}
	dump, _ := os.ReadFile("testdata/dump.csv")
	next := bytes.ReplaceAll(dump, []byte("NIFTY,0,2026-10-27,0,0.1,65,FUT"), []byte("NIFTY,0,2026-10-27,0,0.1,75,FUT"))
	m2, _ := loadMaster(t, ist(2026, 10, 13, 9, 0), next)
	nf, _ = m2.ByID("NFO:NIFTY26OCTFUT")
	if nf.LotSize != 75 || UnitsForLots(2, nf) != 150 {
		t.Fatalf("after: lot %d, 2 lots = %d units, want 75/150", nf.LotSize, UnitsForLots(2, nf))
	}
}
```

`internal/instruments/search_test.go`:

```go
package instruments

import (
	"testing"
	"time"

	"tradeloom/internal/clock"
)

func TestSearchRanking(t *testing.T) { // FR-WATCH-008 AC2
	m, _ := loadMaster(t, ist(2026, 10, 12, 9, 0), nil)
	res := m.Search("BANKNIFTY", 10)
	top := map[string]bool{}
	for i := 0; i < 3 && i < len(res); i++ {
		top[res[i].Instrument.ID()] = true
	}
	if !top["NSE:NIFTY BANK"] {
		t.Fatalf("NIFTY BANK not in top 3: %v", ids(res))
	}
	for _, q := range []string{"nifty", "banknifty", "sensex", "26oct", "ce", "pe", "25000"} {
		for _, r := range m.Search(q, 0) {
			if r.Instrument.Type != "EQ" && r.Instrument.Type != "FUT" {
				t.Fatalf("query %q returned %s", q, r.Instrument.ID())
			}
		}
	}
	rel := m.Search("reliance", 0)
	order := []string{"NSE:RELIANCE", "BSE:RELIANCE", "NFO:RELIANCE26OCTFUT", "NFO:RELIANCE26NOVFUT", "NFO:RELIANCE26DECFUT"}
	for i, id := range order {
		if rel[i].Instrument.ID() != id {
			t.Fatalf("reliance[%d] = %s, want %s (all: %v)", i, rel[i].Instrument.ID(), id, ids(rel))
		}
	}
	if !rel[1].AlsoOnNSE {
		t.Fatal("BSE RELIANCE must be flagged as also on NSE")
	}
	if r := m.Search("bseonly", 0); len(r) != 1 || r[0].AlsoOnNSE {
		t.Fatalf("BSE-only company: %+v", r)
	}
	if r := m.Search("reliance oct fut", 0); len(r) != 1 || r[0].Instrument.ID() != "NFO:RELIANCE26OCTFUT" || r[0].Rank != RankDisplayTokens {
		t.Fatalf("token search: %v", ids(r))
	}
	if r := m.Search("infosys", 0); len(r) != 2 || r[0].Rank != RankNameContains || r[0].Instrument.Tag != TagNSE {
		t.Fatalf("name search: %v", ids(r))
	}
	if r := m.Search("   ", 0); r != nil {
		t.Fatal("blank query must return nothing")
	}
}

func TestExpiredYesterdayDisappearsFromStaleList(t *testing.T) { // FR-WATCH-008 AC5
	m, fc := loadMaster(t, ist(2026, 10, 13, 9, 0), nil)
	if r := m.Search("twinco", 0); len(r) != 2 {
		t.Fatalf("on expiry day both contracts listed: %v", ids(r))
	}
	fc.Set(ist(2026, 10, 14, 9, 0)) // the 13 Oct contract expired yesterday; no new dump yet
	if r := m.Search("twinco", 0); len(r) != 1 || r[0].Instrument.Tradingsymbol != "TWINCO26OCTFUT" {
		t.Fatalf("after expiry: %v", ids(r))
	}
	if fs := m.Futures("NFO-FUT", "TWINCO"); len(fs) != 1 {
		t.Fatalf("Futures after expiry: %+v", fs)
	}
}

func TestSearchUnder100msOnFullSizeMaster(t *testing.T) { // FR-WATCH-008 AC3
	m := NewMaster(clock.NewFake(ist(2026, 10, 12, 9, 0)))
	var ins []Instrument
	for i := 0; i < 30000; i++ { // more rows than the filtered real dump
		sym := "SYM" + string(rune('A'+i%26)) + string(rune('A'+i/26%26)) + string(rune('A'+i/676%26))
		ins = append(ins, Instrument{Token: uint32(i + 1), Exchange: "NSE", Segment: "NSE", Tag: TagNSE, Type: "EQ",
			Tradingsymbol: sym, Name: sym + " LIMITED", DisplayName: sym})
	}
	m.Replace(ins, ist(2026, 10, 12, 8, 31))
	worst := time.Duration(0)
	for _, q := range []string{"s", "sym", "symabc", "limited", "zzz", "sym limited", "q"} {
		t0 := time.Now()
		m.Search(q, 50)
		if d := time.Since(t0); d > worst {
			worst = d
		}
	}
	t.Logf("worst query %v", worst)
	if worst > 100*time.Millisecond {
		t.Fatalf("search took %v", worst)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/instruments/ ./internal/store/`
Expected: the build fails — output contains `undefined: InstrumentRow` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/store/instruments.go`:

```go
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
	rows, err := db.R.QueryContext(ctx, `SELECT token, exchange_token, tradingsymbol, COALESCE(name,''), exchange, segment,
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
	err := db.R.QueryRowContext(ctx, `SELECT downloaded_at, row_count, kept_count, source FROM instrument_imports ORDER BY id DESC LIMIT 1`).
		Scan(&imp.DownloadedAt, &imp.RowCount, &imp.KeptCount, &imp.Source)
	if err == sql.ErrNoRows {
		return imp, false, nil
	}
	return imp, err == nil, err
}
```

`internal/instruments/instrument.go`:

```go
// Package instruments is the instrument master (FR-WATCH-008): import and scope filter (NFR-15),
// display naming (FR-WATCH-013), exchange tags (FR-WATCH-014), local search and daily refresh.
package instruments

import (
	"strings"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
	"tradeloom/internal/store"
)

// Tag is the exchange tag shown everywhere (FR-WATCH-014).
type Tag string

const (
	TagIndices Tag = "INDICES"
	TagNSE     Tag = "NSE"
	TagBSE     Tag = "BSE"
	TagNFOFut  Tag = "NFO-FUT"
	TagBFOFut  Tag = "BFO-FUT"
)

// Instrument is one kept row of the Kite dump plus derived fields.
type Instrument struct {
	Token         uint32    `json:"token"`
	ExchangeToken uint32    `json:"exchangeToken"`
	Exchange      string    `json:"exchange"` // Kite exchange: NSE, BSE, NFO, BFO
	Segment       string    `json:"segment"`
	Tag           Tag       `json:"tag"`
	Tradingsymbol string    `json:"tradingsymbol"`
	Name          string    `json:"name"`
	Type          string    `json:"instrumentType"` // EQ or FUT
	Expiry        time.Time `json:"expiry"`         // zero unless a future; 00:00 IST of the expiry date
	TickSizePaise int64     `json:"tickSizePaise"`
	LotSize       int64     `json:"lotSize"`
	DisplayName   string    `json:"displayName"`
	Underlying    string    `json:"underlying"` // futures: the dump's name field
	IsIndex       bool      `json:"isIndex"`
	IsWeekly      bool      `json:"isWeekly"`
}

// ID is Kite's EXCHANGE:TRADINGSYMBOL form used internally and in deep links (FR-WATCH-013).
func (i Instrument) ID() string { return i.Exchange + ":" + i.Tradingsymbol }

// IsFuture reports whether i is a futures contract (shown with an "F" badge).
func (i Instrument) IsFuture() bool { return i.Type == "FUT" }

// CalendarExchange is the calendar the instrument trades on.
func (i Instrument) CalendarExchange() calendar.Exchange { return calendar.Exchange(i.Exchange) }

// UnitsForLots converts a quantity in lots to units at the instrument's current lot size
// (FR-WATCH-008: saved lot quantities follow lot-size changes).
func UnitsForLots(lots int64, in Instrument) int64 {
	if in.LotSize <= 0 {
		return lots
	}
	return lots * in.LotSize
}

func toRow(i Instrument, importedAt time.Time) store.InstrumentRow {
	var exp int64
	if !i.Expiry.IsZero() {
		exp = int64(calendar.DateOf(i.Expiry).Int())
	}
	return store.InstrumentRow{
		Token: int64(i.Token), ExchangeToken: int64(i.ExchangeToken), Tradingsymbol: i.Tradingsymbol, Name: i.Name,
		Exchange: i.Exchange, Segment: i.Segment, InstrumentType: i.Type, Expiry: exp, TickSizePaise: i.TickSizePaise,
		LotSize: i.LotSize, DisplayName: i.DisplayName, Underlying: i.Underlying, IsIndex: i.IsIndex, IsWeekly: i.IsWeekly,
		ImportedAt: importedAt.UnixMilli(),
	}
}

func fromRow(r store.InstrumentRow) Instrument {
	var exp time.Time
	if r.Expiry != 0 {
		exp = calendar.DateFromInt(int(r.Expiry)).Midnight()
	}
	return Instrument{
		Token: uint32(r.Token), ExchangeToken: uint32(r.ExchangeToken), Exchange: r.Exchange, Segment: r.Segment,
		Tag: tagOf(r.Exchange, r.Segment), Tradingsymbol: r.Tradingsymbol, Name: r.Name, Type: r.InstrumentType, Expiry: exp,
		TickSizePaise: r.TickSizePaise, LotSize: r.LotSize, DisplayName: r.DisplayName, Underlying: r.Underlying,
		IsIndex: r.IsIndex, IsWeekly: r.IsWeekly,
	}
}

// tagOf derives the tag from the instrument's own record only (FR-WATCH-014).
func tagOf(exchange, segment string) Tag {
	switch segment {
	case "INDICES":
		return TagIndices
	case "NFO-FUT":
		return TagNFOFut
	case "BFO-FUT":
		return TagBFOFut
	}
	return Tag(strings.ToUpper(exchange))
}

func midnight(t time.Time) time.Time { return clock.MidnightIST(t) }
```

`internal/instruments/filter.go`:

```go
package instruments

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/kite"
)

// keep implements the NFR-15 scope guard: indices, NSE/BSE cash equities and NFO/BFO futures
// survive; options, MCX, currency and everything else are dropped, and so are expired futures.
func keep(r kite.Instrument, today calendar.Date) bool {
	switch {
	case r.Segment == "INDICES":
		return r.Exchange == "NSE" || r.Exchange == "BSE"
	case (r.Segment == "NSE" || r.Segment == "BSE") && r.Exchange == r.Segment:
		return r.InstrumentType == "EQ"
	case r.Segment == "NFO-FUT" || r.Segment == "BFO-FUT":
		return r.InstrumentType == "FUT" && !r.Expiry.Time.IsZero() && !calendar.DateOf(r.Expiry.Time).Before(today)
	}
	return false
}

// Filter keeps in-scope rows of a Kite dump and derives display names, tags, underlyings and the
// weekly marker. today is the IST date of the import (app clock).
func Filter(rows kite.Instruments, today calendar.Date) []Instrument {
	var out []Instrument
	for _, r := range rows {
		if !keep(r, today) {
			continue
		}
		in := Instrument{
			Token: uint32(r.InstrumentToken), ExchangeToken: uint32(r.ExchangeToken), Exchange: r.Exchange,
			Segment: r.Segment, Tag: tagOf(r.Exchange, r.Segment), Tradingsymbol: r.Tradingsymbol,
			Name: r.Name, Type: r.InstrumentType, TickSizePaise: int64(math.Round(r.TickSize * 100)),
			LotSize: int64(math.Round(r.LotSize)), IsIndex: r.Segment == "INDICES",
		}
		if in.IsFuture() {
			in.Expiry = midnight(r.Expiry.Time)
			in.Underlying = r.Name
		}
		out = append(out, in)
	}
	deriveFutures(out)
	for i := range out {
		if !out[i].IsFuture() {
			out[i].DisplayName = out[i].Tradingsymbol
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Token < out[j].Token })
	return out
}

// deriveFutures sets IsWeekly (expiry is not the last expiry of its month for that underlying
// on that exchange) and the FR-WATCH-013 display name: "UNDERLYING MON FUT", or
// "UNDERLYING DD MON YYYY FUT" when two contracts of the same underlying share a month.
func deriveFutures(all []Instrument) {
	type key struct {
		seg, und string
		y        int
		m        time.Month
	}
	groups := map[key][]int{}
	for i, in := range all {
		if in.IsFuture() {
			y, m, _ := in.Expiry.Date()
			k := key{in.Segment, in.Underlying, y, m}
			groups[k] = append(groups[k], i)
		}
	}
	for _, idx := range groups {
		var last time.Time
		for _, i := range idx {
			if all[i].Expiry.After(last) {
				last = all[i].Expiry
			}
		}
		for _, i := range idx {
			in := &all[i]
			in.IsWeekly = in.Expiry.Before(last)
			mon := strings.ToUpper(in.Expiry.Format("Jan"))
			if len(idx) > 1 {
				in.DisplayName = fmt.Sprintf("%s %02d %s %d FUT", in.Underlying, in.Expiry.Day(), mon, in.Expiry.Year())
			} else {
				in.DisplayName = fmt.Sprintf("%s %s FUT", in.Underlying, mon)
			}
		}
	}
}
```

`internal/instruments/aliases.go`:

```go
package instruments

// indexAliases maps an index tradingsymbol to the name its NFO/BFO futures use in the Kite dump,
// so a search for "BANKNIFTY" finds the NIFTY BANK index (FR-WATCH-008 AC2). Checked against a
// real dump in P0-2 Task 15 Step 5.
var indexAliases = map[string]string{
	"NIFTY 50":          "NIFTY",
	"NIFTY BANK":        "BANKNIFTY",
	"NIFTY FIN SERVICE": "FINNIFTY",
	"NIFTY MID SELECT":  "MIDCPNIFTY",
	"NIFTY NEXT 50":     "NIFTYNXT50",
	"SENSEX":            "SENSEX",
	"BANKEX":            "BANKEX",
}
```

`internal/instruments/master.go`:

```go
package instruments

import (
	"sort"
	"strings"
	"sync"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
)

// entry is an Instrument with search keys precomputed.
type entry struct {
	in        Instrument
	sym       string   // lower tradingsymbol
	alias     string   // lower F&O name for indices ("banknifty"), else ""
	name      string   // lower name
	tokens    []string // lower display-name tokens
	alsoOnNSE bool
}

// Master is the in-memory instrument master. It is replaced wholesale on each import.
type Master struct {
	clk        clock.Clock
	mu         sync.RWMutex
	all        []entry
	byToken    map[uint32]int
	byID       map[string]int
	importedAt time.Time
}

// NewMaster returns an empty master.
func NewMaster(clk clock.Clock) *Master {
	return &Master{clk: clk, byToken: map[uint32]int{}, byID: map[string]int{}}
}

// Replace installs a new set of instruments.
func (m *Master) Replace(ins []Instrument, importedAt time.Time) {
	nse := map[string]bool{}
	for _, in := range ins {
		if in.Tag == TagNSE {
			nse[strings.ToLower(in.Tradingsymbol)] = true
			if in.Name != "" {
				nse["name:"+strings.ToLower(in.Name)] = true
			}
		}
	}
	all := make([]entry, len(ins))
	byToken := make(map[uint32]int, len(ins))
	byID := make(map[string]int, len(ins))
	for i, in := range ins {
		e := entry{in: in, sym: strings.ToLower(in.Tradingsymbol), name: strings.ToLower(in.Name),
			tokens: strings.Fields(strings.ToLower(in.DisplayName))}
		if in.IsIndex {
			e.alias = strings.ToLower(indexAliases[in.Tradingsymbol])
		}
		if in.Tag == TagBSE {
			e.alsoOnNSE = nse[e.sym] || (e.name != "" && nse["name:"+e.name])
		}
		all[i] = e
		byToken[in.Token] = i
		byID[in.ID()] = i
	}
	m.mu.Lock()
	m.all, m.byToken, m.byID, m.importedAt = all, byToken, byID, importedAt
	m.mu.Unlock()
}

// ImportedAt is when the current list was downloaded.
func (m *Master) ImportedAt() time.Time { m.mu.RLock(); defer m.mu.RUnlock(); return m.importedAt }

// Len is the number of instruments.
func (m *Master) Len() int { m.mu.RLock(); defer m.mu.RUnlock(); return len(m.all) }

// ByToken looks an instrument up by Kite token.
func (m *Master) ByToken(token uint32) (Instrument, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i, ok := m.byToken[token]
	if !ok {
		return Instrument{}, false
	}
	return m.all[i].in, true
}

// ByID looks an instrument up by "EXCHANGE:TRADINGSYMBOL".
func (m *Master) ByID(id string) (Instrument, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i, ok := m.byID[id]
	if !ok {
		return Instrument{}, false
	}
	return m.all[i].in, true
}

// ExchangeOf returns the calendar exchange of token (NSE when unknown).
func (m *Master) ExchangeOf(token uint32) calendar.Exchange {
	if in, ok := m.ByToken(token); ok {
		return in.CalendarExchange()
	}
	return calendar.NSE
}

// Futures returns the unexpired futures of an underlying on segment ("NFO-FUT"/"BFO-FUT"),
// nearest expiry first.
func (m *Master) Futures(segment, underlying string) []Instrument {
	today := calendar.DateOf(m.clk.Now())
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Instrument
	for _, e := range m.all {
		if e.in.IsFuture() && e.in.Segment == segment && e.in.Underlying == underlying && !calendar.DateOf(e.in.Expiry).Before(today) {
			out = append(out, e.in)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Expiry.Before(out[j].Expiry) })
	return out
}
```

`internal/instruments/search.go`:

```go
package instruments

import (
	"sort"
	"strings"

	"tradeloom/internal/calendar"
)

// Rank tiers of FR-WATCH-008, best first.
const (
	RankExactSymbol   = 0
	RankSymbolPrefix  = 1
	RankDisplayTokens = 2
	RankNameContains  = 3
)

// Result is one search hit.
type Result struct {
	Instrument Instrument `json:"instrument"`
	Rank       int        `json:"rank"`
	AlsoOnNSE  bool       `json:"alsoOnNse"` // BSE row whose company is also listed on NSE
}

func rankOf(e *entry, q string, qt []string) (int, bool) {
	switch {
	case e.sym == q || (e.alias != "" && e.alias == q):
		return RankExactSymbol, true
	case strings.HasPrefix(e.sym, q) || (e.alias != "" && strings.HasPrefix(e.alias, q)):
		return RankSymbolPrefix, true
	case allTokensMatch(e.tokens, qt):
		return RankDisplayTokens, true
	case e.name != "" && strings.Contains(e.name, q):
		return RankNameContains, true
	}
	return 0, false
}

func allTokensMatch(have, want []string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if strings.HasPrefix(h, w) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return len(want) > 0
}

func classOf(in Instrument) int {
	if in.IsFuture() {
		return 1
	}
	return 0
}

func exchangeOrder(t Tag) int {
	switch t {
	case TagIndices:
		return 0
	case TagNSE:
		return 1
	case TagBSE:
		return 2
	case TagNFOFut:
		return 3
	}
	return 4
}

// Search returns at most limit instruments matching q, ranked per FR-WATCH-008: exact symbol >
// symbol prefix > display-name tokens all matched > name contains; on equal rank indices and
// cash equities before futures, NSE before BSE, nearest expiry first, then display name.
// Expired futures never appear, even from a stale list (AC5).
func (m *Master) Search(q string, limit int) []Result {
	q = strings.ToLower(strings.Join(strings.Fields(q), " "))
	if q == "" {
		return nil
	}
	qt := strings.Fields(q)
	today := calendar.DateOf(m.clk.Now())
	m.mu.RLock()
	var hits []Result
	for i := range m.all {
		e := &m.all[i]
		if e.in.IsFuture() && calendar.DateOf(e.in.Expiry).Before(today) {
			continue
		}
		if r, ok := rankOf(e, q, qt); ok {
			hits = append(hits, Result{Instrument: e.in, Rank: r, AlsoOnNSE: e.alsoOnNSE})
		}
	}
	m.mu.RUnlock()
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.Rank != b.Rank {
			return a.Rank < b.Rank
		}
		if ca, cb := classOf(a.Instrument), classOf(b.Instrument); ca != cb {
			return ca < cb
		}
		if ea, eb := exchangeOrder(a.Instrument.Tag), exchangeOrder(b.Instrument.Tag); ea != eb {
			return ea < eb
		}
		if !a.Instrument.Expiry.Equal(b.Instrument.Expiry) {
			return a.Instrument.Expiry.Before(b.Instrument.Expiry)
		}
		if a.Instrument.DisplayName != b.Instrument.DisplayName {
			return a.Instrument.DisplayName < b.Instrument.DisplayName
		}
		return a.Instrument.Token < b.Instrument.Token
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}
```

`internal/instruments/testdata/dump.csv`:

```csv
instrument_token,exchange_token,tradingsymbol,name,last_price,expiry,strike,tick_size,lot_size,instrument_type,segment,exchange
256265,1001,NIFTY 50,NIFTY 50,0,,0,0,0,EQ,INDICES,NSE
260105,1016,NIFTY BANK,NIFTY BANK,0,,0,0,0,EQ,INDICES,NSE
265,1,SENSEX,SENSEX,0,,0,0,0,EQ,INDICES,BSE
738561,2885,RELIANCE,RELIANCE INDUSTRIES,0,,0,0.1,1,EQ,NSE,NSE
408065,1594,INFY,INFOSYS,0,,0,0.1,1,EQ,NSE,NSE
128083204,500325,RELIANCE,RELIANCE INDUSTRIES,0,,0,0.05,1,EQ,BSE,BSE
128028676,500209,INFY,INFOSYS,0,,0,0.05,1,EQ,BSE,BSE
135200516,528126,BSEONLY,BSE ONLY CO,0,,0,0.01,1,EQ,BSE,BSE
13368834,52219,RELIANCE26OCTFUT,RELIANCE,0,2026-10-27,0,0.1,500,FUT,NFO-FUT,NFO
13369090,52220,RELIANCE26NOVFUT,RELIANCE,0,2026-11-24,0,0.1,500,FUT,NFO-FUT,NFO
13369346,52221,RELIANCE26DECFUT,RELIANCE,0,2026-12-29,0,0.1,500,FUT,NFO-FUT,NFO
13368578,52218,RELIANCE26SEPFUT,RELIANCE,0,2026-09-29,0,0.1,500,FUT,NFO-FUT,NFO
12345602,48225,NIFTY26OCTFUT,NIFTY,0,2026-10-27,0,0.1,65,FUT,NFO-FUT,NFO
12345858,48226,NIFTY26NOVFUT,NIFTY,0,2026-11-24,0,0.1,65,FUT,NFO-FUT,NFO
12346114,48227,BANKNIFTY26OCTFUT,BANKNIFTY,0,2026-10-27,0,0.2,30,FUT,NFO-FUT,NFO
12346370,48228,TWINCO26OCT13FUT,TWINCO,0,2026-10-13,0,0.05,100,FUT,NFO-FUT,NFO
12346626,48229,TWINCO26OCTFUT,TWINCO,0,2026-10-27,0,0.05,100,FUT,NFO-FUT,NFO
209876293,819829,SENSEX26OCTFUT,SENSEX,0,2026-10-29,0,0.05,20,FUT,BFO-FUT,BFO
12347138,48231,NIFTY26OCT25000CE,NIFTY,0,2026-10-27,25000,0.05,65,CE,NFO-OPT,NFO
12347394,48232,BANKNIFTY26OCT56000PE,BANKNIFTY,0,2026-10-27,56000,0.05,30,PE,NFO-OPT,NFO
209876549,819830,SENSEX26OCT82000CE,SENSEX,0,2026-10-29,82000,0.05,20,CE,BFO-OPT,BFO
53505799,209008,CRUDEOIL26OCTFUT,CRUDEOIL,0,2026-10-19,0,1,100,FUT,MCX-FUT,MCX
1373186,5364,USDINR26OCTFUT,USDINR,0,2026-10-27,0,0.0025,1,FUT,CDS-FUT,CDS
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/instruments/ ./internal/store/`
Expected: `ok  	tradeloom/internal/instruments`, `ok  	tradeloom/internal/store`; no `DATA RACE`.

- [ ] **Step 5: Check the index aliases against a real dump**

The aliases in `aliases.go` map index tradingsymbols to the `name` Kite uses for their futures. With a logged-in developer session (any day):

Run: `curl -s -H "X-Kite-Version: 3" -H "Authorization: token $KITE_API_KEY:$KITE_ACCESS_TOKEN" https://api.kite.trade/instruments | grep -E ',(NIFTY|BANKNIFTY|FINNIFTY|MIDCPNIFTY|NIFTYNXT50|SENSEX|BANKEX),' | grep -E 'FUT,(NFO|BFO)-FUT' | cut -d, -f4 | sort -u`
Expected: the seven names `BANKEX BANKNIFTY FINNIFTY MIDCPNIFTY NIFTY NIFTYNXT50 SENSEX` (one per line). Any name missing from the output is removed from `indexAliases`; any index future name that appears under a different spelling replaces its entry. Rerun the Step 4 command afterwards.

- [ ] **Step 6: Commit**

```bash
git add internal/store/instruments.go \
  internal/instruments/instrument.go \
  internal/instruments/filter.go \
  internal/instruments/aliases.go \
  internal/instruments/master.go \
  internal/instruments/search.go \
  internal/instruments/testdata/dump.csv \
  internal/store/instruments_test.go \
  internal/instruments/fixture_test.go \
  internal/instruments/filter_test.go \
  internal/instruments/search_test.go
git commit -m "$(cat <<'EOF'
feat(instruments): scoped instrument master with FR-WATCH-013 naming, tags and ranked search

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
