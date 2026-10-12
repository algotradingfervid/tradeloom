// Package clock is Tradeloom's app clock: system time plus a measured offset to exchange
// time (FR-DATA-004). It is the only package that may call time.Now, time.Sleep, time.After,
// time.AfterFunc or time.NewTimer (ARCHITECTURE §2.4 rule 8); every other package receives a
// Clock and gets time from it.
package clock

import (
	"context"
	"time"
	_ "time/tzdata" // embed the zone database so IST resolves on minimal systems
)

// IST is Asia/Kolkata. Every time the app shows, logs or schedules is in this zone.
var IST = mustLoadIST()

func mustLoadIST() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		panic("clock: cannot load Asia/Kolkata: " + err.Error())
	}
	return loc
}

// Source says where the current offset estimate comes from.
type Source string

const (
	SourceNone     Source = "none"
	SourceTicks    Source = "ticks"
	SourceHTTPDate Source = "http-date"
)

// Offset is the measured difference exchange time − system time. App time = system + Value.
type Offset struct {
	Value     time.Duration `json:"-"`
	Millis    int64         `json:"millis"`
	Source    Source        `json:"source"`
	Samples   int           `json:"samples"`
	Warn      bool          `json:"warn"`   // |Value| > 1 s
	Skewed    bool          `json:"skewed"` // |Value| > 5 s and not yet back under 2 s or acknowledged
	UpdatedAt time.Time     `json:"updatedAt"`
}

// Timer fires once on C when the app clock reaches its target.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// Clock is the interface every package receives. Real is the production implementation,
// Fake the test implementation.
type Clock interface {
	Now() time.Time    // app clock (system + offset), in IST
	System() time.Time // raw system time in IST; only for stamping offset samples
	Offset() Offset
	At(target time.Time) Timer   // fires when Now() >= target; re-armed when the offset moves
	After(d time.Duration) Timer // At(Now()+d)
}

// Sleep waits d on c or until ctx ends.
func Sleep(ctx context.Context, c Clock, d time.Duration) error {
	t := c.After(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C():
		return nil
	}
}
