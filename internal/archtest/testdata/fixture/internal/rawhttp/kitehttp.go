// Package rawhttp holds Ruling 69 violations. It also imports net/http outside internal/kite (g0-http).
package rawhttp

import (
	"net/http"

	kc "github.com/zerodha/gokiteconnect/v4"
)

// RawWrite sends an order POST through gokiteconnect's raw HTTP client, which the nm check cannot see.
// Violations: g0-kitehttp NewHTTPClient, HTTPClient.Do and URIPlaceOrder; g0-path for the folded URL.
func RawWrite() error {
	h := kc.NewHTTPClient(http.DefaultClient)
	_, err := h.Do(http.MethodPost, "https://api.kite.trade"+kc.URIPlaceOrder)
	return err
}

// doer aliases the banned type. Violation: g0-kitehttp HTTPClient here; later uses of doer resolve to
// the alias, not to HTTPClient, so only this declaration is reported.
type doer = kc.HTTPClient

// gttPath copies a banned constant. Violations: g0-kitehttp URIModifyGTT; g0-path for its value.
const gttPath = kc.URIModifyGTT

// Concrete reaches the unexported implementation. Violations: g0-kitehttp GetClient, (*httpClient).Do
// and URIDeleteGTT; g0-path for the folded gttPath+URIDeleteGTT (its operands are not re-reported).
func Concrete(h doer) error {
	f := h.GetClient().Do
	_, err := f(http.MethodDelete, gttPath+kc.URIDeleteGTT)
	return err
}

// Reads uses read URI constants: no g0-kitehttp, but the folded "/orders/portfolio/positions" starts
// with /orders, so g0-path reports it.
var Reads = kc.URIGetOrders + kc.URIGetPositions
