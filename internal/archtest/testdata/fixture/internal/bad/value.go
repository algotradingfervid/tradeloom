package bad

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// ViaMethodValue is a G-0 violation: method value taken, then called.
func ViaMethodValue(c *kiteconnect.Client) {
	f := c.PlaceOrder
	f("regular", kiteconnect.OrderParams{})
}
