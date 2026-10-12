package settings

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/logging"
	"tradeloom/internal/store"
)

type memBackend struct {
	m       map[string]string
	saveErr error
}

func (b *memBackend) Load(context.Context) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range b.m {
		out[k] = v
	}
	return out, nil
}

func (b *memBackend) Save(_ context.Context, k, v string) error {
	if b.saveErr != nil {
		return b.saveErr
	}
	b.m[k] = v
	return nil
}

func TestGet_DefaultsAndStoredValues(t *testing.T) {
	b := &memBackend{m: map[string]string{
		"backup.schedule.keep":   "12",
		"log.level":              `"verbose"`, // fails validation → default
		"backup.schedule.folder": `not json`,  // corrupt → default
	}}
	s, err := Load(context.Background(), b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if Get(s, BackupScheduleKeep) != 12 || Get(s, LogLevel) != "info" || Get(s, BackupScheduleFolder) != "" || Get(s, BackupScheduleEnabled) {
		t.Fatal("unexpected values")
	}
}

func TestSet_ValidatesPersistsAndPublishes(t *testing.T) {
	bs := bus.New(nil)
	defer bs.Close()
	sub := bus.Subscribe(bs, Topic, bus.SubOpts[Changed]{Name: "test"})
	b := &memBackend{m: map[string]string{}}
	s, _ := Load(context.Background(), b, bs)

	cases := []struct {
		name    string
		set     func() error
		wantErr bool
	}{
		{"valid level", func() error { return Set(context.Background(), s, LogLevel, "debug") }, false},
		{"invalid level", func() error { return Set(context.Background(), s, LogLevel, "trace") }, true},
		{"keep too low", func() error { return Set(context.Background(), s, BackupScheduleKeep, 0) }, true},
		{"relative folder", func() error { return Set(context.Background(), s, BackupScheduleFolder, "backups") }, true},
		{"absolute folder", func() error { return Set(context.Background(), s, BackupScheduleFolder, "/media/usb/tl") }, false},
	}
	for _, tc := range cases {
		err := tc.set()
		if (err != nil) != tc.wantErr {
			t.Fatalf("%s: err=%v", tc.name, err)
		}
		var ve *ValidationError
		if tc.wantErr && !errors.As(err, &ve) {
			t.Fatalf("%s: want ValidationError, got %T", tc.name, err)
		}
	}
	if Get(s, LogLevel) != "debug" || b.m["log.level"] != `"debug"` || b.m["backup.schedule.keep"] != "" {
		t.Fatalf("state %v", b.m)
	}
	for _, want := range []string{"log.level", "backup.schedule.folder"} {
		select {
		case ev := <-sub.C():
			if ev.Key != want {
				t.Fatalf("event %s, want %s", ev.Key, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no event for %s", want)
		}
	}
}

func TestSet_BackendErrorLeavesCacheUnchanged(t *testing.T) {
	b := &memBackend{m: map[string]string{}, saveErr: errors.New("disk I/O error")}
	s, _ := Load(context.Background(), b, nil)
	if err := Set(context.Background(), s, BackupScheduleKeep, 9); err == nil {
		t.Fatal("want error")
	}
	if Get(s, BackupScheduleKeep) != 7 {
		t.Fatal("cache changed on failed save")
	}
}

// NFR-04: settings never hold secrets.
func TestSet_RefusesSecrets(t *testing.T) {
	s, _ := Load(context.Background(), &memBackend{m: map[string]string{}}, nil)
	if err := Set(context.Background(), s, Key[string]{Name: "kite.api_secret"}, "x"); !errors.Is(err, ErrSecretKey) {
		t.Fatalf("want ErrSecretKey, got %v", err)
	}
	logging.RegisterSecret("registeredSecretValue42")
	if err := Set(context.Background(), s, Key[string]{Name: "note"}, "has registeredSecretValue42 inside"); !errors.Is(err, ErrSecretValue) {
		t.Fatalf("want ErrSecretValue, got %v", err)
	}
}

func TestSettings_OverStore(t *testing.T) {
	dir := t.TempDir()
	open := func() *store.DB {
		db, err := store.Open(context.Background(), filepath.Join(dir, "app.db"), store.Options{AppVersion: "v0.1.0",
			BackupDir: filepath.Join(dir, "backups"), Now: func() time.Time { return time.Unix(1, 0) }})
		if err != nil {
			t.Fatal(err)
		}
		return db
	}
	db := open()
	s, err := Load(context.Background(), db.SettingsRepo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Set(context.Background(), s, BackupScheduleKeep, 30); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db = open()
	defer db.Close()
	s2, _ := Load(context.Background(), db.SettingsRepo(), nil)
	if Get(s2, BackupScheduleKeep) != 30 {
		t.Fatal("value not persisted")
	}
	snap := s2.Snapshot()
	var found bool
	for _, c := range snap {
		if c.Key == "backup.schedule.keep" && string(c.Value) == "30" {
			found = true
		}
	}
	if !found {
		t.Fatalf("snapshot %v", snap)
	}
}
