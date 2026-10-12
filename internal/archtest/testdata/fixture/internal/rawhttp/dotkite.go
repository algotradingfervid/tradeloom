package rawhttp

import . "github.com/zerodha/gokiteconnect/v4"

// DotWrite names banned identifiers through a dot-import. Violations: HTTPClient, URIAlerts, URIPlaceMFSIP.
func DotWrite(h HTTPClient) string { return URIAlerts + URIPlaceMFSIP }

// DotRead is allowed: a read endpoint through the dot-import.
var DotRead = URIGetOrders
