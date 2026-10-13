package instruments

import (
	"context"
	"os"
	"reflect"
	"testing"

	"tradeloom/internal/calendar"
	"tradeloom/internal/store"
)

func TestFromRowFailsClosed(t *testing.T) {
	base := store.InstrumentRow{Token: 1, Tradingsymbol: "X", DisplayName: "X", ImportedAt: 1}
	bad := map[string]store.InstrumentRow{
		"NFO-OPT CE":          {Exchange: "NFO", Segment: "NFO-OPT", InstrumentType: "CE"},
		"MCX":                 {Exchange: "MCX", Segment: "MCX-FUT", InstrumentType: "FUT"},
		"INDICES-typed FUT":   {Exchange: "NSE", Segment: "INDICES", InstrumentType: "FUT"},
		"NSE exchange on BSE": {Exchange: "NSE", Segment: "BSE", InstrumentType: "EQ"},
	}
	for name, r := range bad {
		r.Token, r.Tradingsymbol, r.DisplayName = base.Token, base.Tradingsymbol, base.DisplayName
		if _, ok := fromRow(r); ok {
			t.Errorf("%s: fromRow ok=true", name)
		}
	}
	// IsIndex follows the segment, not the stored column.
	in, ok := fromRow(store.InstrumentRow{Token: 2, Exchange: "NSE", Segment: "INDICES", InstrumentType: "EQ", IsIndex: false})
	if !ok || !in.IsIndex || in.Tag != TagIndices {
		t.Fatalf("index row = %+v %v", in, ok)
	}
	in, ok = fromRow(store.InstrumentRow{Token: 3, Exchange: "NSE", Segment: "NSE", InstrumentType: "EQ", IsIndex: true})
	if !ok || in.IsIndex {
		t.Fatalf("equity row claimed index: %+v", in)
	}
	if _, ok := tagOf("MCX", "MCX-FUT"); ok {
		t.Fatal("tagOf accepted MCX")
	}
}

func TestRowRoundTrip(t *testing.T) {
	body, err := os.ReadFile("testdata/dump.csv")
	if err != nil {
		t.Fatal(err)
	}
	now := ist(2026, 10, 12, 9, 0)
	c, _, _ := kiteFixture(t, now, body)
	rows, err := c.Instruments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := Filter(rows, calendar.DateOf(now))
	if len(want) == 0 {
		t.Fatal("empty fixture")
	}
	for _, in := range want {
		got, ok := fromRow(toRow(in, now))
		if !ok {
			t.Fatalf("%s dropped by fromRow", in.ID())
		}
		if !got.Expiry.Equal(in.Expiry) {
			t.Fatalf("%s expiry %v != %v", in.ID(), got.Expiry, in.Expiry)
		}
		got.Expiry = in.Expiry // equal instants; normalise location
		if !reflect.DeepEqual(got, in) {
			t.Fatalf("round trip\n got %+v\nwant %+v", got, in)
		}
	}
}
