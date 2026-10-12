package reads

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Orders is allowed: read-only call.
func Orders(c *kiteconnect.Client) { c.GetOrders() }
