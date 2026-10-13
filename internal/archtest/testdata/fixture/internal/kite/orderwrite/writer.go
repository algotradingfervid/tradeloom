//go:build orders && !noorders

package orderwrite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Place is allowed: an orders-only file in internal/kite/orderwrite (g0-call).
func Place(c *kiteconnect.Client) { c.PlaceOrder("regular", kiteconnect.OrderParams{}) }

// Raw is allowed: the raw HTTP client and write URIs in an orders-only orderwrite file (g0-kitehttp, g0-path).
func Raw() kiteconnect.HTTPClient { _ = kiteconnect.URIPlaceGTT; return kiteconnect.NewHTTPClient(nil) }

// regularPath is allowed: a write path in an orders-only orderwrite file (g0-path).
const regularPath = "/orders/regular"
