package bad

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Place is a G-0 violation: order-write call outside internal/kite/orderwrite.
func Place(c *kiteconnect.Client) { c.PlaceOrder("regular", kiteconnect.OrderParams{}) }

type notKite struct{}

func (notKite) PlaceOrder() {}

// Harmless calls a same-named method on another type: not a violation.
func Harmless() { notKite{}.PlaceOrder() }
