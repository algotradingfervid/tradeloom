package orders

import (
	"sync/atomic"

	"fixture/internal/kite"
	"fixture/internal/kite/orderwrite"
)

// Submit is allowed: the method is orderwrite.Writer's, which G-0 gates by build tag.
func Submit(w orderwrite.Writer) { w.PlaceOrder("regular") }

// Current is allowed: kite.Client (not *kiteconnect.Client) as a type argument.
var Current atomic.Pointer[kite.Client]
