package rawpath

// placePath is a G-0 violation (g0-path): a plain literal naming an order-write endpoint.
const placePath = "/orders/regular"

const (
	seg      = "alerts"
	kiteHost = "https://api.kite.trade"
)

// Concatenations: no single literal is a write path, the folded constant is. Violations ×2.
var (
	AlertPath  = "/" + seg
	ConvertURL = kiteHost + "/" + "portfolio/positions"
)

// Leading whitespace, and a host without a scheme. Violations ×2.
var (
	Authorise = "  /portfolio/holdings/authorise"
	MFHost    = "api.kite.trade/mf/orders"
)
