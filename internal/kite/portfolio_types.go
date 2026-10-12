// internal/kite/portfolio_types.go
package kite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Position and Holding complete P0-2's alias list (types.go has Positions and Holdings only), so
// internal/orders can name a single row without importing gokiteconnect (rule 3).
type (
	Position = kiteconnect.Position
	Holding  = kiteconnect.Holding
)
