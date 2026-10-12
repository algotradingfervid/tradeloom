// internal/orders/scope.go
package orders

import (
	"fmt"
	"strings"
)

// Scope selects what a refresh fetches.
type Scope uint8

const (
	ScopeOrders Scope = 1 << iota
	ScopePositions
	ScopeHoldings
	ScopeAll = ScopeOrders | ScopePositions | ScopeHoldings
)

func ParseScope(s string) (Scope, error) {
	switch s {
	case "", "all":
		return ScopeAll, nil
	case "orders":
		return ScopeOrders, nil
	case "positions":
		return ScopePositions, nil
	case "holdings":
		return ScopeHoldings, nil
	}
	return 0, fmt.Errorf("unknown refresh scope %q", s)
}

// inEquityScope reports whether a row belongs to P0's equity (cash + futures) scope. Kite should
// not return anything else for an equity-only account; this defensively drops MCX/CDS/BCD rows
// and NFO/BFO options (only *FUT symbols pass). An empty exchange is kept (nothing to judge by).
func inEquityScope(exchange, symbol string) bool {
	switch strings.ToUpper(exchange) {
	case "", "NSE", "BSE":
		return true
	case "NFO", "BFO":
		return strings.HasSuffix(strings.ToUpper(symbol), "FUT")
	}
	return false
}
