package store

import (
	"context"
	"testing"
)

func TestSettingsRepo_RoundTrip(t *testing.T) {
	db := mustOpen(t, t.TempDir(), nil, "v0.1.0")
	defer db.Close()
	r := db.SettingsRepo()
	ctx := context.Background()
	if err := r.Save(ctx, "log.level", `"debug"`); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(ctx, "log.level", `"info"`); err != nil {
		t.Fatal(err)
	}
	m, err := r.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if m["log.level"] != `"info"` || m[SettingAppVersionLast] != `"v0.1.0"` {
		t.Fatalf("%v", m)
	}
}
