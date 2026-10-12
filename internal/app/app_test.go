package app

import (
	"context"
	"testing"
)

func TestHello(t *testing.T) {
	a := New()
	a.OnStartup(context.Background())
	if got := a.Hello(); got != "Tradeloom dev" {
		t.Fatalf("Hello() = %q", got)
	}
}
