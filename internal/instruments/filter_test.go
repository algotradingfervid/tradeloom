package instruments

import (
	"bytes"
	"os"
	"testing"

	"tradeloom/internal/calendar"
	"tradeloom/internal/kite"
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

// NFR-15 is fail-closed: only the exact in-scope (segment, exchange, instrument type) shapes survive;
// any unknown or inconsistent combination is dropped (user scope decision: no options, MCX, currency).
func TestScopeFilterFailsClosed(t *testing.T) {
	today := calendar.DateOf(ist(2026, 10, 12, 9, 0))
	exp := func(d int) kite.Time { return kite.Time{Time: ist(2026, 10, d, 0, 0)} }
	row := func(tok int, seg, exch, typ string, e kite.Time) kite.Instrument {
		return kite.Instrument{InstrumentToken: tok, Tradingsymbol: "X", Name: "X", Segment: seg, Exchange: exch, InstrumentType: typ, Expiry: e, TickSize: 0.05, LotSize: 1}
	}
	kept := kite.Instruments{
		row(1, "INDICES", "NSE", "EQ", kite.Time{}),
		row(2, "INDICES", "BSE", "EQ", kite.Time{}),
		row(3, "NSE", "NSE", "EQ", kite.Time{}),
		row(4, "BSE", "BSE", "EQ", kite.Time{}),
		row(5, "NFO-FUT", "NFO", "FUT", exp(12)), // expires today: still listed
		row(6, "BFO-FUT", "BFO", "FUT", exp(29)),
	}
	dropped := kite.Instruments{
		row(101, "INDICES", "MCX", "EQ", kite.Time{}),
		row(102, "INDICES", "NSE", "FUT", kite.Time{}),
		row(103, "INDICES", "NSE", "", kite.Time{}),
		row(104, "NSE", "BSE", "EQ", kite.Time{}),
		row(105, "NSE", "NSE", "CE", kite.Time{}),
		row(106, "NSE", "NSE", "", kite.Time{}),
		row(107, "NFO-FUT", "BFO", "FUT", exp(27)),
		row(108, "BFO-FUT", "NFO", "FUT", exp(27)),
		row(109, "NFO-FUT", "NFO", "CE", exp(27)),
		row(110, "NFO-FUT", "NFO", "FUT", kite.Time{}),
		row(111, "NFO-FUT", "NFO", "FUT", exp(11)), // expired yesterday
		row(112, "NFO-OPT", "NFO", "PE", exp(27)),
		row(113, "BFO-OPT", "BFO", "CE", exp(29)),
		row(114, "MCX-FUT", "MCX", "FUT", exp(19)),
		row(115, "CDS-FUT", "CDS", "FUT", exp(27)),
		row(116, "BCD-FUT", "BCD", "FUT", exp(27)),
		row(117, "nfo-fut", "NFO", "FUT", exp(27)),
		row(118, "", "NSE", "EQ", kite.Time{}),
		row(119, "NSE-NEW", "NSE", "EQ", kite.Time{}),
	}
	got := Filter(append(append(kite.Instruments{}, kept...), dropped...), today)
	if len(got) != len(kept) {
		t.Fatalf("kept %d rows, want %d: %+v", len(got), len(kept), got)
	}
	for i, in := range got {
		if int(in.Token) != kept[i].InstrumentToken {
			t.Fatalf("row %d: token %d survived, want %d", i, in.Token, kept[i].InstrumentToken)
		}
		if in.Type != "EQ" && in.Type != "FUT" {
			t.Fatalf("token %d kept with type %q", in.Token, in.Type)
		}
	}
}
