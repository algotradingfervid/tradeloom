// internal/service/settings_test.go
package service

import (
	"context"
	"testing"
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
	if _, err := e.svc.SetSetting(ctx, "no.such.key", `1`); refusedCode(t, err) != "unknown_setting" {
		t.Fatal(err)
	}
	if _, err := e.svc.SetSetting(ctx, "appearance.theme", `not json`); refusedCode(t, err) != "invalid_setting" {
		t.Fatal(err)
	}
}
