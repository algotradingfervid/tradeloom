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
