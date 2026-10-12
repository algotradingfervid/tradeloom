//go:build orders && !noorders

package orderwrite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Place is allowed: tagged file in internal/kite/orderwrite.
func Place(c *kiteconnect.Client) { c.PlaceOrder("regular", kiteconnect.OrderParams{}) }

// Raw is allowed (Ruling 69): the raw HTTP client and write URIs in a tagged orderwrite file.
func Raw() kiteconnect.HTTPClient { _ = kiteconnect.URIPlaceGTT; return kiteconnect.NewHTTPClient(nil) }

// regularPath is allowed: a write path in an orders-tagged orderwrite file.
const regularPath = "/orders/regular"
