> **P0-2 Time & Market Data Implementation Plan — Task 16.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 16: Instrument refresher: daily download after 08:30, failure fallback, dump copy

**Files:**
- Create: `internal/instruments/refresher.go`
- Test: `internal/instruments/refresher_test.go`

**Interfaces:**
- Consumes: Task 1: `clock.Clock`, `clock.FormatDateIST`. Task 4: `(*calendar.Service).IsTradingDay`, `InstrumentsReadyAt`, `NextTradingDay`. Task 6: `ratelimit.WithPriority`. Task 7: `kite.ReadClient`. Task 15: `Master`, `Filter`, `toRow`, `fromRow`, `store.InstrumentRow`, `(*store.DB).ReplaceInstruments/LoadInstruments/LastInstrumentImport`, `kiteFixture` (test helper). P0-1: `bus.*`, `store.Open`, `store.Options`, `DB.R`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
type Status struct {
	OK         bool      `json:"ok"`
	ImportedAt time.Time `json:"importedAt"` // date of the list in use
	Kept       int       `json:"kept"`
	Message    string    `json:"message"` // WARNING text when !OK (FR-WATCH-008 AC4)
}
var TopicStatus = bus.NewTopic[Status]("instruments.status")
type Options struct {
	Client   kite.ReadClient
	Master   *Master
	DB       *store.DB
	Calendar *calendar.Service
	Clock    clock.Clock
	Bus      *bus.Bus
	DataDir  string // the dump copy goes to <DataDir>/instruments/last.csv.gz
	Log      *slog.Logger
}
type Refresher struct{ o Options }
func NewRefresher(o Options) *Refresher
func (r *Refresher) Load(ctx context.Context) error
func (r *Refresher) Due(now time.Time) bool
func (r *Refresher) Refresh(ctx context.Context) error
func (r *Refresher) Run(ctx context.Context)
func WriteDump(path string, ins []Instrument) error
func ReadDump(path string, today calendar.Date) ([]Instrument, error)
```

FR-WATCH-008 AC1 (one download at the first start after 08:30 IST on a trading day, none on a holiday), AC4 (on failure the previous list stays, search works, and a WARNING names its date), offline start from `app.db`, and the `instruments/last.csv.gz` copy (§4.5).

- [ ] **Step 1: Write the failing tests**

`internal/instruments/refresher_test.go`:

```go
package instruments

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/mock"
	"tradeloom/internal/store"
)

type rig struct {
	srv *mock.Server
	cli *kite.Client
	db  *store.DB
	clk *clock.Fake
	cal *calendar.Service
	m   *Master
	r   *Refresher
	bus *bus.Bus
	dir string
}

func newRig(t *testing.T, now time.Time) *rig {
	t.Helper()
	g := &rig{bus: bus.New(nil), dir: t.TempDir()}
	g.cli, g.srv, g.clk = kiteFixture(t, now, nil)
	var err error
	if g.db, err = store.Open(context.Background(), filepath.Join(g.dir, "app.db"), store.Options{BackupDir: filepath.Join(g.dir, "backups"), Now: time.Now}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.db.Close() })
	g.cal, _ = calendar.NewDefault()
	g.m = NewMaster(g.clk)
	g.r = NewRefresher(Options{Client: g.cli, Master: g.m, DB: g.db, Calendar: g.cal, Clock: g.clk, Bus: g.bus, DataDir: g.dir})
	return g
}

func (g *rig) mustRefresh(t *testing.T) {
	t.Helper()
	if err := g.r.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshPersistsScopedRows(t *testing.T) { // NFR-15 (app.db side)
	g := newRig(t, ist(2026, 10, 12, 9, 0))
	g.mustRefresh(t)
	var n int
	if err := g.db.R.QueryRow(`SELECT COUNT(*) FROM instruments WHERE segment LIKE '%OPT' OR exchange IN ('MCX','CDS','BCD')`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("out-of-scope rows in app.db: %d %v", n, err)
	}
	if imp, ok, _ := g.db.LastInstrumentImport(context.Background()); !ok || imp.RowCount != 23 || imp.KeptCount != 17 {
		t.Fatalf("import record %+v", imp)
	}
}

func TestDueOncePerTradingDayAfterDumpTime(t *testing.T) { // FR-WATCH-008 AC1
	g := newRig(t, ist(2026, 10, 12, 8, 45))
	if err := g.r.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	g.m.Replace(nil, ist(2026, 10, 9, 8, 40)) // list dated the previous trading day
	if !g.r.Due(ist(2026, 10, 12, 8, 45)) {
		t.Fatal("08:45 on a trading day with yesterday's list must be due")
	}
	if g.r.Due(ist(2026, 10, 12, 8, 29)) {
		t.Fatal("before 08:30 must not be due")
	}
	g.mustRefresh(t)
	if g.r.Due(ist(2026, 10, 12, 8, 50)) {
		t.Fatal("second check on the same day must not download again")
	}
	if g.r.Due(ist(2026, 10, 20, 8, 45)) { // Dussehra holiday
		t.Fatal("holiday must not download")
	}
	if g.srv.Calls(mock.RouteInstruments) != 1 {
		t.Fatalf("downloads = %d", g.srv.Calls(mock.RouteInstruments))
	}
}

func TestRunDownloadsAtStartThenAtNextDumpTime(t *testing.T) {
	g := newRig(t, ist(2026, 10, 16, 9, 0)) // Friday
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.r.Run(ctx)
	if !g.clk.WaitTimers(2, 2*time.Second) { // limiter usage loop + Run's next-dump timer
		t.Fatal("Run never armed its timer")
	}
	if n := g.srv.Calls(mock.RouteInstruments); n != 1 {
		t.Fatalf("downloads at start = %d", n)
	}
	g.clk.Set(ist(2026, 10, 19, 8, 30)) // Monday 08:30
	deadline := time.Now().Add(2 * time.Second)
	for g.srv.Calls(mock.RouteInstruments) < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if n := g.srv.Calls(mock.RouteInstruments); n != 2 {
		t.Fatalf("downloads after Monday 08:30 = %d", n)
	}
}

func TestFailedDownloadKeepsPreviousListAndWarnsWithDate(t *testing.T) { // FR-WATCH-008 AC4
	g := newRig(t, ist(2026, 10, 12, 9, 0))
	sub := bus.Subscribe(g.bus, TopicStatus, bus.SubOpts[Status]{Buffer: 8})
	g.mustRefresh(t)
	<-sub.C()
	g.clk.Set(ist(2026, 10, 13, 9, 0))
	for i := 0; i < 4; i++ { // first try + 3 network retries
		g.srv.FailNext(mock.RouteInstruments, mock.Failure{Status: 503, ErrorType: "NetworkException", Message: "down"})
	}
	go func() { // the client's network retries sleep on the app clock
		for i := 0; i < 400; i++ {
			g.clk.Advance(100 * time.Millisecond)
			time.Sleep(time.Millisecond)
		}
	}()
	if err := g.r.Refresh(context.Background()); err == nil {
		t.Fatal("refresh succeeded against a failing server")
	}
	st := <-sub.C()
	if st.OK || st.Message != "Instrument list refresh failed; using the list from 12-10-2026" {
		t.Fatalf("status %+v", st)
	}
	if r := g.m.Search("reliance", 1); len(r) != 1 {
		t.Fatal("search broken after failed refresh")
	}
	// Restart offline: the previous list still loads from app.db.
	m2 := NewMaster(g.clk)
	r2 := NewRefresher(Options{Client: g.cli, Master: m2, DB: g.db, Calendar: g.cal, Clock: g.clk})
	if err := r2.Load(context.Background()); err != nil || m2.Len() != 17 {
		t.Fatalf("reload: %d %v", m2.Len(), err)
	}
}

func TestDumpFileRoundTrip(t *testing.T) {
	g := newRig(t, ist(2026, 10, 12, 9, 0))
	g.mustRefresh(t)
	got, err := ReadDump(filepath.Join(g.dir, "instruments", "last.csv.gz"), calendar.Date{Year: 2026, Month: 10, Day: 12})
	if err != nil || len(got) != 17 {
		t.Fatalf("read dump: %d %v", len(got), err)
	}
	in, _ := g.m.ByToken(got[0].Token)
	if got[0] != in {
		t.Fatalf("round trip %+v != %+v", got[0], in)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/instruments/`
Expected: the build fails — output contains `undefined: NewRefresher` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/instruments/refresher.go`:

```go
package instruments

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/store"
)

// Status is published on TopicStatus after every load or refresh attempt.
type Status struct {
	OK         bool      `json:"ok"`
	ImportedAt time.Time `json:"importedAt"` // date of the list in use
	Kept       int       `json:"kept"`
	Message    string    `json:"message"` // WARNING text when !OK (FR-WATCH-008 AC4)
}

// TopicStatus is "instruments.status"; the app bridge turns !OK into a WARNING toast.
var TopicStatus = bus.NewTopic[Status]("instruments.status")

// Options configures the Refresher.
type Options struct {
	Client   kite.ReadClient
	Master   *Master
	DB       *store.DB
	Calendar *calendar.Service
	Clock    clock.Clock
	Bus      *bus.Bus
	DataDir  string // the dump copy goes to <DataDir>/instruments/last.csv.gz
	Log      *slog.Logger
}

// Refresher downloads the dump once per trading day after 08:30 IST and on demand.
type Refresher struct{ o Options }

// NewRefresher builds a Refresher.
func NewRefresher(o Options) *Refresher {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	return &Refresher{o: o}
}

// Load fills the Master from app.db (startup; works offline).
func (r *Refresher) Load(ctx context.Context) error {
	rows, err := r.o.DB.LoadInstruments(ctx)
	if err != nil {
		return err
	}
	imp, _, err := r.o.DB.LastInstrumentImport(ctx)
	if err != nil {
		return err
	}
	ins := make([]Instrument, len(rows))
	for i, row := range rows {
		ins[i] = fromRow(row)
	}
	r.o.Master.Replace(ins, time.UnixMilli(imp.DownloadedAt).In(clock.IST))
	return nil
}

// Due reports whether a download is needed at now: on an NSE trading day, at or after the dump
// time, when the list in use was downloaded before today's dump time (FR-WATCH-008 AC1).
func (r *Refresher) Due(now time.Time) bool {
	d := calendar.DateOf(now)
	if !r.o.Calendar.IsTradingDay(calendar.NSE, d) {
		return false
	}
	ready := r.o.Calendar.InstrumentsReadyAt(d)
	return !now.Before(ready) && r.o.Master.ImportedAt().Before(ready)
}

// Refresh downloads, filters and installs the dump. On failure the previous list stays in use
// and a !OK status names its date.
func (r *Refresher) Refresh(ctx context.Context) error {
	now := r.o.Clock.Now()
	rows, err := r.o.Client.Instruments(ratelimit.WithPriority(ctx, ratelimit.Interactive))
	if err == nil && len(rows) == 0 {
		err = fmt.Errorf("instruments: empty dump")
	}
	var kept []Instrument
	if err == nil {
		kept = Filter(rows, calendar.DateOf(now))
		imp := store.InstrumentImport{DownloadedAt: now.UnixMilli(), RowCount: len(rows), KeptCount: len(kept), Source: "kite"}
		dbRows := make([]store.InstrumentRow, len(kept))
		for i, in := range kept {
			dbRows[i] = toRow(in, now)
		}
		err = r.o.DB.ReplaceInstruments(ctx, dbRows, imp)
	}
	if err != nil {
		prev := r.o.Master.ImportedAt()
		msg := "Instrument list refresh failed and no earlier list is available"
		if !prev.IsZero() && prev.Unix() > 0 {
			msg = "Instrument list refresh failed; using the list from " + clock.FormatDateIST(prev)
		}
		r.o.Log.Warn("instrument refresh failed", "err", err, "using", prev)
		publish(r.o.Bus, TopicStatus, Status{OK: false, ImportedAt: prev, Kept: r.o.Master.Len(), Message: msg})
		return err
	}
	r.o.Master.Replace(kept, now)
	if r.o.DataDir != "" {
		if werr := WriteDump(filepath.Join(r.o.DataDir, "instruments", "last.csv.gz"), kept); werr != nil {
			r.o.Log.Warn("instrument dump copy not written", "err", werr)
		}
	}
	r.o.Log.Info("instruments imported", "rows", len(rows), "kept", len(kept))
	publish(r.o.Bus, TopicStatus, Status{OK: true, ImportedAt: now, Kept: len(kept)})
	return nil
}

// Run refreshes now if due, then at every following trading day's dump time, until ctx ends.
func (r *Refresher) Run(ctx context.Context) {
	for {
		if r.Due(r.o.Clock.Now()) {
			_ = r.Refresh(ctx)
		}
		next, ok := r.o.Calendar.NextTradingDay(calendar.NSE, calendar.DateOf(r.o.Clock.Now()))
		if !ok {
			return
		}
		t := r.o.Clock.At(r.o.Calendar.InstrumentsReadyAt(next))
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C():
		}
	}
}

var dumpHeader = []string{"instrument_token", "exchange_token", "tradingsymbol", "name", "expiry", "tick_size_paise", "lot_size", "instrument_type", "segment", "exchange"}

// WriteDump stores the kept instruments as gzipped CSV (offline re-import, §4.5).
func WriteDump(path string, ins []Instrument) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(f)
	w := csv.NewWriter(zw)
	_ = w.Write(dumpHeader)
	for _, in := range ins {
		exp := ""
		if !in.Expiry.IsZero() {
			exp = calendar.DateOf(in.Expiry).String()
		}
		_ = w.Write([]string{strconv.FormatUint(uint64(in.Token), 10), strconv.FormatUint(uint64(in.ExchangeToken), 10),
			in.Tradingsymbol, in.Name, exp, strconv.FormatInt(in.TickSizePaise, 10), strconv.FormatInt(in.LotSize, 10),
			in.Type, in.Segment, in.Exchange})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadDump reads a file written by WriteDump and re-derives names and flags.
func ReadDump(path string, today calendar.Date) ([]Instrument, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	recs, err := csv.NewReader(zr).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("instruments: empty dump file")
	}
	rows := make(kite.Instruments, 0, len(recs)-1)
	for _, rec := range recs[1:] {
		if len(rec) != len(dumpHeader) {
			return nil, fmt.Errorf("instruments: bad dump row %v", rec)
		}
		tok, _ := strconv.Atoi(rec[0])
		et, _ := strconv.Atoi(rec[1])
		tick, _ := strconv.ParseInt(rec[5], 10, 64)
		lot, _ := strconv.ParseFloat(rec[6], 64)
		k := kite.Instrument{InstrumentToken: tok, ExchangeToken: et, Tradingsymbol: rec[2], Name: rec[3],
			TickSize: float64(tick) / 100, LotSize: lot, InstrumentType: rec[7], Segment: rec[8], Exchange: rec[9]}
		if rec[4] != "" {
			d, err := calendar.ParseDate(rec[4])
			if err != nil {
				return nil, err
			}
			k.Expiry.Time = d.Midnight()
		}
		rows = append(rows, k)
	}
	return Filter(rows, today), nil
}

// publish sends v on t when a bus is configured (tests and tools may run without one).
func publish[T any](b *bus.Bus, t bus.Topic[T], v T) {
	if b != nil {
		bus.Publish(b, t, v)
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/instruments/`
Expected: `ok  	tradeloom/internal/instruments`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/instruments/refresher.go \
  internal/instruments/refresher_test.go
git commit -m "$(cat <<'EOF'
feat(instruments): daily refresher with offline fallback and dump copy

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
