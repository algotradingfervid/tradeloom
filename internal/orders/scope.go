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

// outOfScope reports whether a row is outside P0's equity (cash + futures) scope (Ruling 38). Rows are
// never dropped; they are flagged. Detection fails open: only MCX/CDS/BCD, and NFO/BFO option symbols
// (ending CE/PE), are out of scope. Anything else (empty exchange, FUT, unknown suffixes) is in scope.
func outOfScope(exchange, symbol string) bool {
	switch strings.ToUpper(exchange) {
	case "MCX", "CDS", "BCD":
		return true
	case "NFO", "BFO":
		sym := strings.ToUpper(symbol)
		return strings.HasSuffix(sym, "CE") || strings.HasSuffix(sym, "PE")
	}
	return false
}
