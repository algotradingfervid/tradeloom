package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCalendarOverridesRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, filepath.Join(dir, "app.db"), Options{BackupDir: filepath.Join(dir, "backups"), Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := []CalendarOverrideRow{
		{"NSE", 20261108, "special", `[{"kind":"continuous","start":"18:15","end":"19:15"}]`, "user"},
		{"NSE", 20261014, "inferred-non-trading", `[]`, "inferred"},
	}
	if err := db.UpsertCalendarOverrides(ctx, rows); err != nil {
		t.Fatal(err)
	}
	rows[0].Phases = `[{"kind":"continuous","start":"18:00","end":"19:00"}]`
	if err := db.UpsertCalendarOverrides(ctx, rows[:1]); err != nil {
		t.Fatal(err)
	}
	got, err := db.CalendarOverrides(ctx)
	if err != nil || len(got) != 2 || got[0].Date != 20261014 || got[1].Phases != rows[0].Phases {
		t.Fatalf("got %+v %v", got, err)
	}
	if err := db.DeleteCalendarOverride(ctx, "NSE", 20261014); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.CalendarOverrides(ctx); len(got) != 1 {
		t.Fatalf("after delete: %+v", got)
	}
}
