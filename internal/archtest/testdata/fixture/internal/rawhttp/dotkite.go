package rawhttp

import . "github.com/zerodha/gokiteconnect/v4"

// DotWrite names banned identifiers through a dot-import. Violations: g0-kitehttp HTTPClient, URIAlerts
// and URIPlaceMFSIP; g0-path for the folded URIAlerts+URIPlaceMFSIP.
func DotWrite(h HTTPClient) string { return URIAlerts + URIPlaceMFSIP }

// DotRead is a read constant through the dot-import: no g0-kitehttp, but its value "/orders" is a
// g0-path violation.
var DotRead = URIGetOrders
