package orderwrite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// cancelPath is in the right package but in a file compiled without the orders tag. Violations:
// g0-kitehttp URICancelOrder; g0-path for its value.
const cancelPath = kiteconnect.URICancelOrder
