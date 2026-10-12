package clock

import (
	"context"
	"testing"
	"time"
)

func TestISTIsKolkataEvenWhenLocalIsUTC(t *testing.T) {
	saved := time.Local
	time.Local = time.UTC
	defer func() { time.Local = saved }()

	f := NewFake(time.Date(2026, 10, 13, 0, 30, 0, 0, time.UTC))
	if got := f.Now().Location().String(); got != "Asia/Kolkata" {
		t.Fatalf("Now() zone = %q, want Asia/Kolkata", got)
	}
	if got := f.Now().Format("15:04"); got != "06:00" {
		t.Fatalf("00:30 UTC rendered as %s IST, want 06:00", got)
	}
}

func TestExpiryAt0600ISTFiresAt0030UTC(t *testing.T) { // FR-DATA-004 AC2
	f := NewFake(time.Date(2026, 10, 13, 0, 29, 59, 0, time.UTC))
	expiry := time.Date(2026, 10, 13, 6, 0, 0, 0, IST)
	tm := f.At(expiry)
	select {
	case <-tm.C():
		t.Fatal("fired one second early")
	default:
	}
	f.Advance(time.Second)
	select {
	case got := <-tm.C():
		if got.UTC().Format("15:04:05") != "00:30:00" {
			t.Fatalf("fired at %s UTC, want 00:30:00", got.UTC().Format("15:04:05"))
		}
	default:
		t.Fatal("did not fire at 00:30:00 UTC")
	}
}

func TestNextBoundaryAndBucketStart(t *testing.T) {
	anchor := time.Date(2026, 10, 12, 9, 15, 0, 0, IST)
	cases := []struct {
		now  time.Time
		step time.Duration
		next string
	}{
		{time.Date(2026, 10, 12, 9, 0, 0, 0, IST), 5 * time.Minute, "09:15:00"},
		{anchor, 5 * time.Minute, "09:20:00"},
		{time.Date(2026, 10, 12, 9, 19, 59, 0, IST), 5 * time.Minute, "09:20:00"},
		{time.Date(2026, 10, 12, 10, 14, 0, 0, IST), time.Hour, "10:15:00"},
		{time.Date(2026, 10, 12, 10, 15, 0, 0, IST), time.Hour, "11:15:00"},
	}
	for _, c := range cases {
		if got := NextBoundary(c.now, anchor, c.step).Format("15:04:05"); got != c.next {
			t.Errorf("NextBoundary(%s, %v) = %s, want %s", c.now.Format("15:04:05"), c.step, got, c.next)
		}
	}
	if got := BucketStart(time.Date(2026, 10, 12, 10, 2, 30, 0, IST), anchor, 5*time.Minute).Format("15:04"); got != "10:00" {
		t.Fatalf("BucketStart = %s, want 10:00", got)
	}
}

func TestFakeOffsetMovesNowNotSystem(t *testing.T) {
	sys := time.Date(2026, 10, 12, 10, 0, 3, 0, IST)
	f := NewFake(sys)
	f.SetOffset(Offset{Value: -3 * time.Second, Source: SourceTicks})
	if !f.System().Equal(sys) {
		t.Fatalf("System moved: %v", f.System())
	}
	if got := f.Now().Format("15:04:05"); got != "10:00:00" {
		t.Fatalf("Now = %s, want 10:00:00", got)
	}
	if f.Offset().Millis != -3000 {
		t.Fatalf("Millis = %d", f.Offset().Millis)
	}
}

func TestFakeStopAndSleep(t *testing.T) {
	f := NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, IST))
	tm := f.After(time.Minute)
	if !tm.Stop() {
		t.Fatal("Stop on pending timer returned false")
	}
	f.Advance(time.Hour)
	select {
	case <-tm.C():
		t.Fatal("stopped timer fired")
	default:
	}

	done := make(chan error, 1)
	go func() { done <- Sleep(context.Background(), f, 10*time.Second) }()
	if !f.WaitTimers(1, time.Second) {
		t.Fatal("Sleep never armed a timer")
	}
	f.Advance(10 * time.Second)
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Sleep(ctx, f, time.Hour); err != context.Canceled {
		t.Fatalf("Sleep on cancelled ctx = %v", err)
	}
}
