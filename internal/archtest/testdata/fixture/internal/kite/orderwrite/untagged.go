package orderwrite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Modify is a G-0 violation: right package, but compiled without the orders tag.
func Modify(c *kiteconnect.Client) { c.ModifyOrder("regular", "1", kiteconnect.OrderParams{}) }
