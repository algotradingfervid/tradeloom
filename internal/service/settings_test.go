// internal/service/settings_test.go
package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/settings"
)

func TestSettingsFacade(t *testing.T) {
	e := newSvcEnv(t)
	ctx := context.Background()
	all, err := e.svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range all {
		if len(v.Name) >= 5 && v.Name[:5] == "kite." {
			t.Fatalf("credential key %s listed", v.Name)
		}
	}
	v, err := e.svc.SetSetting(ctx, "appearance.theme", `"dark"`)
	if err != nil || v.ValueJSON != `"dark"` {
		t.Fatalf("%+v %v", v, err)
	}
	if _, err := e.svc.SetSetting(ctx, "kite.redirect_port", `8000`); refusedCode(t, err) != "use_credentials_flow" {
		t.Fatal(err)
	}
	if v, err := e.svc.SetSetting(ctx, "watchlist.active", `"wl-9"`); err != nil || v.ValueJSON != `"wl-9"` {
		t.Fatalf("%+v %v", v, err)
	}
	if _, err := e.svc.SetSetting(ctx, "no.such.key", `1`); refusedCode(t, err) != "unknown_setting" {
		t.Fatal(err)
	}
	if _, err := e.svc.SetSetting(ctx, "appearance.theme", `not json`); refusedCode(t, err) != "invalid_setting" {
		t.Fatal(err)
	}
}

type failBackend struct{}

func (failBackend) Load(context.Context) (map[string]string, error) { return map[string]string{}, nil }
func (failBackend) Save(context.Context, string, string) error {
	return errors.New("disk exploded at /secret/path")
}

func TestSetSetting_StoreFailureIsInternal(t *testing.T) {
	e := newSvcEnv(t)
	st, err := settings.Load(context.Background(), failBackend{}, e.bus)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(Deps{Session: e.session, Orders: e.sync, Settings: st, Clock: e.clock, Bus: e.bus, Log: slog.Default()})
	_, err = svc.SetSetting(context.Background(), "appearance.theme", `"dark"`)
	if c := refusedCode(t, err); c != "internal" {
		t.Fatalf("code %q, want internal", c)
	}
	// validation failure keeps invalid_setting
	_, err = svc.SetSetting(context.Background(), "appearance.theme", `"blue"`)
	if c := refusedCode(t, err); c != "invalid_setting" {
		t.Fatalf("code %q, want invalid_setting", c)
	}
}

func TestSetSetting_SecretKeyIsNotInvalidSetting(t *testing.T) {
	e := newSvcEnv(t)
	_, err := e.svc.SetSetting(context.Background(), "x.api_secret", `"v"`)
	if c := refusedCode(t, err); c != "unknown_setting" {
		t.Fatalf("code %q", c)
	}
}

func TestSetSetting_PublishesOneEventPerSuccess(t *testing.T) {
	e := newSvcEnv(t)
	sub := bus.Subscribe(e.bus, settings.Topic, bus.SubOpts[settings.Changed]{Name: "t17", Buffer: 16, Critical: true})
	ctx := context.Background()
	if _, err := e.svc.SetSetting(ctx, "appearance.theme", `"dark"`); err != nil {
		t.Fatal(err)
	}
	e.svc.SetSetting(ctx, "appearance.theme", `"blue"`)
	e.svc.SetSetting(ctx, "appearance.theme", `not json`)
	e.svc.SetSetting(ctx, "kite.redirect_port", `8000`)
	e.svc.SetSetting(ctx, "no.such.key", `1`)
	n := 0
	timeout := time.After(300 * time.Millisecond)
loop:
	for {
		select {
		case ch := <-sub.C():
			if ch.Key != "appearance.theme" {
				t.Fatalf("unexpected event %+v", ch)
			}
			n++
		case <-timeout:
			break loop
		}
	}
	if n != 1 {
		t.Fatalf("%d events, want 1", n)
	}
}

func TestSettings_KiteAPIKeyRegisteredButHidden(t *testing.T) {
	e := newSvcEnv(t)
	found := false
	for _, n := range settings.Names() {
		if n == "kite.api_key" {
			found = true
		}
	}
	if !found {
		t.Fatal("kite.api_key is not registered")
	}
	all, err := e.svc.Settings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range all {
		if v.Name == "kite.api_key" {
			t.Fatal("kite.api_key listed")
		}
	}
}
