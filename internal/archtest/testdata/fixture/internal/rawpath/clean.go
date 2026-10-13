package rawpath

// Reads and unrelated strings are allowed: read endpoints, a bare word, a near-miss prefix, the host alone,
// and labels that start with a write prefix but are not paths (a method name, an event name, an error format).
var Reads = []string{"/portfolio/holdings", "/user/profile", "/quote/ltp", "/session/token",
	"/instruments/historical/1/day", "orders", "/mfx", "mfx/orders", kiteHost,
	"GetOrders", "orders.sync", "orders: %w", "portfolio refresh failed", "/quote/%zz", "%zz/orders"}
