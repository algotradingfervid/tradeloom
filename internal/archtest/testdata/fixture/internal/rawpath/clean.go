package rawpath

// Reads and unrelated strings are allowed: read endpoints, a bare word, a near-miss prefix, the host alone.
var Reads = []string{"/portfolio/holdings", "/user/profile", "/quote/ltp", "/session/token",
	"/instruments/historical/1/day", "orders", "/mfx", "mf/orders", kiteHost}
