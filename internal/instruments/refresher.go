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
type Refresher struct {
	o       Options
	trigger chan struct{}
}

// NewRefresher builds a Refresher.
func NewRefresher(o Options) *Refresher {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	return &Refresher{o: o, trigger: make(chan struct{}, 1)}
}

// Load fills the Master from app.db (startup; works offline).
func (r *Refresher) Load(ctx context.Context) error {
	rows, err := r.o.DB.LoadInstruments(ctx)
	if err != nil {
		return err
	}
	imp, hasImp, err := r.o.DB.LastInstrumentImport(ctx)
	if err != nil {
		return err
	}
	today := calendar.DateOf(r.o.Clock.Now())
	ins := make([]Instrument, 0, len(rows))
	var dropped, expired int
	for _, row := range rows {
		in, ok := fromRow(row)
		if !ok {
			dropped++
			continue
		}
		if in.IsFuture() && (in.Expiry.IsZero() || calendar.DateOf(in.Expiry).Before(today)) {
			expired++ // fromRow does not re-check expiry against today
			continue
		}
		ins = append(ins, in)
	}
	if dropped > 0 || expired > 0 {
		r.o.Log.Warn("instrument rows dropped on load", "out_of_scope", dropped, "expired_futures", expired)
	}
	var at time.Time
	if hasImp {
		at = time.UnixMilli(imp.DownloadedAt).In(clock.IST)
	}
	r.o.Master.Replace(ins, at)
	if hasImp {
		publish(r.o.Bus, TopicStatus, Status{OK: true, ImportedAt: at, Kept: len(ins)})
	}
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
		if ctx.Err() != nil {
			return err // shutting down: not a refresh failure worth a WARNING
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

// retryBackoff is the same-day retry schedule after a failed refresh; the last step repeats.
var retryBackoff = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute}

// Trigger asks Run to refresh now if due (e.g. after login). It never blocks and coalesces.
func (r *Refresher) Trigger() {
	select {
	case r.trigger <- struct{}{}:
	default:
	}
}

// Run refreshes now if due, retries failures on a same-day back-off, then waits for the next
// trading day's dump time, until ctx ends. Trigger wakes it early.
func (r *Refresher) Run(ctx context.Context) {
	failures := 0
	for {
		now := r.o.Clock.Now()
		if r.Due(now) {
			if err := r.Refresh(ctx); err != nil {
				failures++
			} else {
				failures = 0
			}
			now = r.o.Clock.Now()
		}
		if ctx.Err() != nil {
			return
		}
		var target time.Time
		today := calendar.DateOf(now)
		if r.Due(now) { // still no list for today: retry later the same day
			step := failures - 1
			if step < 0 {
				step = 0
			}
			if step >= len(retryBackoff) {
				step = len(retryBackoff) - 1
			}
			target = now.Add(retryBackoff[step])
			if calendar.DateOf(target) != today {
				target = time.Time{} // the day is over; fall through to the next dump time
			}
		}
		if target.IsZero() {
			next := today
			if !r.o.Calendar.IsTradingDay(calendar.NSE, next) || !now.Before(r.o.Calendar.InstrumentsReadyAt(next)) {
				var ok bool
				if next, ok = r.o.Calendar.NextTradingDay(calendar.NSE, next); !ok {
					r.o.Log.Warn("instrument refresher stopping: no further trading day in the calendar", "from", today.String())
					return
				}
			}
			target = r.o.Calendar.InstrumentsReadyAt(next)
			failures = 0
		}
		t := r.o.Clock.At(target)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C():
		case <-r.trigger:
			t.Stop()
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
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmp)
		}
	}()
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
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
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
