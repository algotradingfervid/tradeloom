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
