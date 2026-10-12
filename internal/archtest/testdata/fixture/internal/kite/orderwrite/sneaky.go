//go:build (orders && !noorders) || go1.1

package orderwrite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Sneaky is a G-0 violation (Ruling 79 I-1): the go1.1 term is true in every build, so the file links
// without the orders tag. Violations: g0-call (PlaceOrder), g0-kitehttp (URIPlaceOrder) and g0-path
// (the folded URIPlaceOrder + literal).
func Sneaky(c *kiteconnect.Client) string {
	c.PlaceOrder("regular", kiteconnect.OrderParams{})
	return kiteconnect.URIPlaceOrder + "/x"
}
