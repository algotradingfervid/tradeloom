// internal/settings/registry_test.go
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
)

type regTestBackend struct{ m map[string]string }

func (b *regTestBackend) Load(context.Context) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range b.m {
		out[k] = v
	}
	return out, nil
}

func (b *regTestBackend) Save(_ context.Context, k, v string) error { b.m[k] = v; return nil }

func TestRegistry_NamesAreSortedAndIncludeP01Keys(t *testing.T) {
	names := Names()
	if !sort.StringsAreSorted(names) {
		t.Fatalf("not sorted: %v", names)
	}
	for _, want := range []string{LogLevel.Name, BackupScheduleEnabled.Name, BackupScheduleFolder.Name, BackupScheduleKeep.Name, BackupIncludeCandles.Name} {
		if i := sort.SearchStrings(names, want); i >= len(names) || names[i] != want {
			t.Fatalf("%s not registered: %v", want, names)
		}
	}
}

func TestRegistry_GetSetJSON(t *testing.T) {
	ctx := context.Background()
	s, err := Load(ctx, &regTestBackend{m: map[string]string{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := GetJSON(s, "log.level")
	if err != nil || string(raw) != `"info"` {
		t.Fatalf("default: %s %v", raw, err)
	}
	if err := SetJSON(ctx, s, "log.level", json.RawMessage(`"debug"`)); err != nil {
		t.Fatal(err)
	}
	if raw, _ = GetJSON(s, "log.level"); string(raw) != `"debug"` {
		t.Fatalf("after set: %s", raw)
	}
	var ve *ValidationError
	if err := SetJSON(ctx, s, "log.level", json.RawMessage(`"trace"`)); !errors.As(err, &ve) {
		t.Fatalf("validator not applied: %v", err)
	}
	if err := SetJSON(ctx, s, "backup.schedule.keep", json.RawMessage(`"seven"`)); !errors.As(err, &ve) {
		t.Fatalf("type mismatch not reported as a validation error: %v", err)
	}
	if raw, _ = GetJSON(s, "backup.schedule.keep"); string(raw) != `7` {
		t.Fatalf("value changed on a rejected set: %s", raw)
	}
	if _, err := GetJSON(s, "no.such.key"); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("GetJSON unknown: %v", err)
	}
	if err := SetJSON(ctx, s, "no.such.key", json.RawMessage(`1`)); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("SetJSON unknown: %v", err)
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(r.(string), "registered twice") {
			t.Fatalf("recover = %v", r)
		}
	}()
	Register(Key[string]{Name: LogLevel.Name})
}

func TestRegistry_AllRegisteredKeysAcceptedBySecretGuard(t *testing.T) {
	ctx := context.Background()
	s, err := Load(ctx, &regTestBackend{m: map[string]string{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range Names() {
		raw, err := GetJSON(s, n)
		if err != nil {
			t.Fatal(err)
		}
		if err := SetJSON(ctx, s, n, raw); errors.Is(err, ErrSecretKey) || errors.Is(err, ErrSecretValue) {
			t.Fatalf("registered key %s refused by the secret guard: %v", n, err)
		}
	}
}

func TestRegister_RejectsSecretLikeName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Register accepted a secret-like name")
		}
	}()
	Register(Key[string]{Name: "kite.api_secret"})
}
