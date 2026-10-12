// internal/store/sessions_test.go
package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

func TestSessionsRepo_Lifecycle(t *testing.T) {
	ctx := context.Background()
	r := store.NewSessionsRepo(storetest.Open(t))
	if _, err := r.Get(ctx, "missing"); !errors.Is(err, store.ErrNoRow) {
		t.Fatalf("Get missing = %v, want ErrNoRow", err)
	}
	row := store.SessionRow{ID: "s1", UserID: "AB1234", LoggedInAtMs: 1000, ExpiresAtMs: 2000, DataPlan: "unknown"}
	if err := r.Insert(ctx, row); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDataPlan(ctx, "s1", "connect"); err != nil {
		t.Fatal(err)
	}
	if err := r.AckRisk(ctx, "s1", 1500); err != nil {
		t.Fatal(err)
	}
	if err := r.AckRisk(ctx, "s1", 1600); err != nil { // second ack keeps the first time
		t.Fatal(err)
	}
	if err := r.End(ctx, "s1", 1900, "logout"); err != nil {
		t.Fatal(err)
	}
	if err := r.End(ctx, "s1", 1950, "session_expired"); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	want := store.SessionRow{ID: "s1", UserID: "AB1234", LoggedInAtMs: 1000, ExpiresAtMs: 2000, RiskAckAtMs: 1500, DataPlan: "connect", EndedAtMs: 1900, EndReason: "logout"}
	if got != want {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

// Secrets (access token, api secret) live in the secrets store only; the sessions table must have no column for them.
func TestSessionsTable_HoldsNoSecrets(t *testing.T) {
	ctx := context.Background()
	db := storetest.Open(t)
	rows, err := db.R.QueryContext(ctx, `SELECT name FROM pragma_table_info('sessions')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, c)
	}
	want := "id,user_id,logged_in_at,expires_at,risk_ack_at,data_plan,ended_at,end_reason"
	if got := strings.Join(cols, ","); got != want {
		t.Fatalf("sessions columns = %s, want %s", got, want)
	}
	for _, c := range cols {
		l := strings.ToLower(c)
		if strings.Contains(l, "token") || strings.Contains(l, "secret") {
			t.Fatalf("sessions has secret-like column %q", c)
		}
	}
}
