package bad

import (
	"context"

	"fixture/internal/kite"
)

// Interface variant: Call reached through a generic interface, F inferred from Client.Call's
// method signature. Violations: instantiations whose type arguments mention *kiteconnect.Client.
type caller[F any] interface {
	Call(context.Context, string, F) error
}

func via[F any](c caller[F], f F) error { return c.Call(context.Background(), "x", f) }

func EvilIface(c *kite.Client) error { return via(c, cancel) }
