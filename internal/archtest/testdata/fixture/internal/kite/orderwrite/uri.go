package orderwrite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// cancelPath is a G-0 violation (g0-kitehttp): right package, but compiled without the orders tag.
const cancelPath = kiteconnect.URICancelOrder
