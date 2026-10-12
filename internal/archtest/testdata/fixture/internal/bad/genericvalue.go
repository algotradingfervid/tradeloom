package bad

import (
	"context"

	"fixture/internal/kite"
)

// Method-value variant of the probe: Call taken as a method value and as a method expression,
// CancelOrder taken as a method value on a type parameter.
func cancelValue[R any, C canceller[R]](kc C) error {
	m := kc.CancelOrder
	_, err := m("regular", "123", nil)
	return err
}

func EvilValue(c *kite.Client) error {
	f := c.Call
	if err := f(context.Background(), "x", cancelValue); err != nil {
		return err
	}
	return (*kite.Client).Call(c, context.Background(), "x", cancelValue)
}
