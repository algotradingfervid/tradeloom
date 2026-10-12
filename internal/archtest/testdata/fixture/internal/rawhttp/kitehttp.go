package rawhttp

import (
	"net/http"

	kc "github.com/zerodha/gokiteconnect/v4"
)

// RawWrite is a G-0 violation (Ruling 69): an order POST through gokiteconnect's raw HTTP client,
// which the nm check cannot see. Violations: NewHTTPClient, HTTPClient.Do, URIPlaceOrder.
func RawWrite() error {
	h := kc.NewHTTPClient(http.DefaultClient)
	_, err := h.Do(http.MethodPost, "https://api.kite.trade"+kc.URIPlaceOrder)
	return err
}

// doer aliases the banned type. Violation: HTTPClient (uses of the alias resolve to it).
type doer = kc.HTTPClient

// gttPath copies a banned constant. Violation: URIModifyGTT.
const gttPath = kc.URIModifyGTT

// Concrete reaches the unexported implementation. Violations: GetClient, (*httpClient).Do, URIDeleteGTT.
func Concrete(h doer) error {
	f := h.GetClient().Do
	_, err := f(http.MethodDelete, gttPath+kc.URIDeleteGTT)
	return err
}

// Reads is allowed: read endpoints.
var Reads = kc.URIGetOrders + kc.URIGetPositions
