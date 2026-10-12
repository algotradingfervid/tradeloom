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

func TestLoadDropsOutOfScopeAndExpiredRows(t *testing.T) { // Ruling 40 carry-forward
	g := newRig(t, ist(2026, 10, 12, 9, 0))
	g.mustRefresh(t)
	rows, err := g.db.LoadInstruments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	good := len(rows)
	rows = append(rows,
		store.InstrumentRow{Token: 9001, Tradingsymbol: "FOO", Exchange: "MCX", Segment: "MCX-FUT", InstrumentType: "FUT", Expiry: 20261029},
		store.InstrumentRow{Token: 9002, Tradingsymbol: "OLDFUT", Exchange: "NFO", Segment: "NFO-FUT", InstrumentType: "FUT", Expiry: 20260929})
	imp := store.InstrumentImport{DownloadedAt: ist(2026, 10, 12, 9, 0).UnixMilli(), RowCount: len(rows), KeptCount: len(rows), Source: "kite"}
	if err := g.db.ReplaceInstruments(context.Background(), rows, imp); err != nil {
		t.Fatal(err)
	}
	m2 := NewMaster(g.clk)
	if err := NewRefresher(Options{Master: m2, DB: g.db, Clock: g.clk}).Load(context.Background()); err != nil || m2.Len() != good {
		t.Fatalf("loaded %d want %d (%v)", m2.Len(), good, err)
	}
}
