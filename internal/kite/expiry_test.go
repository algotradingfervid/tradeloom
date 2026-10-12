// internal/kite/expiry_test.go
package kite

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/clock"
)

func ist(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, clock.IST)
}

func TestNextExpiry(t *testing.T) {
	cases := []struct {
		name     string
		in, want time.Time
	}{
		{"morning login expires next day 06:00", ist(2026, 10, 12, 9, 2), ist(2026, 10, 13, 6, 0)},
		{"login at 05:59 expires the same day 06:00", ist(2026, 10, 12, 5, 59), ist(2026, 10, 12, 6, 0)},
		{"login exactly at 06:00 expires next day", ist(2026, 10, 12, 6, 0), ist(2026, 10, 13, 6, 0)},
		{"late night", ist(2026, 10, 12, 23, 59), ist(2026, 10, 13, 6, 0)},
		{"UTC input is converted to IST", time.Date(2026, 10, 12, 3, 32, 0, 0, time.UTC), ist(2026, 10, 13, 6, 0)},
		{"month end", ist(2026, 10, 31, 10, 0), ist(2026, 11, 1, 6, 0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NextExpiry(c.in); !got.Equal(c.want) {
				t.Fatalf("NextExpiry(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestSessionStateJSONIsBindingFriendly(t *testing.T) {
	b, err := json.Marshal(SessionState{State: StateReady, ExpiresAtMs: 1, DataPlan: PlanConnect})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"state":"ready"`, `"expiresAtMs":1`, `"dataPlan":"connect"`, `"riskAckRequired":false`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("%s missing in %s", want, b)
		}
	}
}

func TestMaskUserID(t *testing.T) {
	if got := MaskUserID("AB1234"); got != "AB****" {
		t.Fatalf("got %q", got)
	}
	if got := MaskUserID("A"); got != "*" {
		t.Fatalf("got %q", got)
	}
}
