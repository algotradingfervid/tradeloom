package bad

import (
	"context"

	"fixture/internal/kite"
)

// The reviewer's probe: a generic function whose constraint names CancelOrder is instantiated
// with *kiteconnect.Client by inference and handed to kite.Client.Call. Violations: the Call
// reference, the canceller declaration, the CancelOrder selector on a type parameter and the
// instantiation with *kiteconnect.Client.
type canceller[R any] interface {
	CancelOrder(string, string, *string) (R, error)
}

func cancel[R any, C canceller[R]](kc C) error {
	_, err := kc.CancelOrder("regular", "123", nil)
	return err
}

func Evil(c *kite.Client) error { return c.Call(context.Background(), "x", cancel) }
