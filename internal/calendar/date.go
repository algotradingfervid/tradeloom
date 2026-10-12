// Package calendar is the market calendar and session model (FR-DATA-001): trading days,
// holidays, special sessions and phases per exchange, from a shipped CSV plus user overrides.
// It is pure: no I/O packages (ARCHITECTURE §2.4 rule 1); persistence is done by callers.
package calendar

import (
	"fmt"
	"time"

	"tradeloom/internal/clock"
)

// Date is an IST calendar date.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the IST date containing t.
func DateOf(t time.Time) Date {
	y, m, d := t.In(clock.IST).Date()
	return Date{y, m, d}
}

// ParseDate parses YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	t, err := time.ParseInLocation("2006-01-02", s, clock.IST)
	if err != nil {
		return Date{}, err
	}
	return DateOf(t), nil
}

// DateFromInt converts yyyymmdd.
func DateFromInt(n int) Date { return Date{n / 10000, time.Month(n / 100 % 100), n % 100} }

func (d Date) Int() int              { return d.Year*10000 + int(d.Month)*100 + d.Day }
func (d Date) String() string        { return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day) }
func (d Date) Display() string       { return fmt.Sprintf("%02d-%02d-%04d", d.Day, int(d.Month), d.Year) }
func (d Date) Midnight() time.Time   { return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, clock.IST) }
func (d Date) AddDays(n int) Date    { return DateOf(d.Midnight().AddDate(0, 0, n)) }
func (d Date) Weekday() time.Weekday { return d.Midnight().Weekday() }
func (d Date) Before(o Date) bool    { return d.Int() < o.Int() }
func (d Date) After(o Date) bool     { return d.Int() > o.Int() }
func (d Date) IsWeekend() bool {
	w := d.Weekday()
	return w == time.Saturday || w == time.Sunday
}

// At returns the instant hm on date d in IST.
func (d Date) At(hm HM) time.Time {
	return time.Date(d.Year, d.Month, d.Day, int(hm)/60, int(hm)%60, 0, 0, clock.IST)
}

// HM is a time of day in minutes after midnight.
type HM int

// ParseHM parses HH:MM (00:00–23:59).
func ParseHM(s string) (HM, error) {
	var h, m int
	if len(s) != 5 || s[2] != ':' {
		return 0, fmt.Errorf("invalid time %q", s)
	}
	if _, err := fmt.Sscanf(s, "%02d:%02d", &h, &m); err != nil || h > 23 || m > 59 || h < 0 || m < 0 {
		return 0, fmt.Errorf("invalid time %q", s)
	}
	return HM(h*60 + m), nil
}

func (h HM) String() string { return fmt.Sprintf("%02d:%02d", int(h)/60, int(h)%60) }

func (h HM) MarshalText() ([]byte, error) { return []byte(h.String()), nil }

func (h *HM) UnmarshalText(b []byte) error {
	v, err := ParseHM(string(b))
	if err != nil {
		return err
	}
	*h = v
	return nil
}
