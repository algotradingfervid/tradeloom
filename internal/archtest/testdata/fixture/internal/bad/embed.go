package bad

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

type wrapper struct{ *kiteconnect.Client }

// ViaEmbedding is a G-0 violation: promoted method of the embedded client.
func ViaEmbedding(w wrapper) { w.PlaceOrder("regular", kiteconnect.OrderParams{}) }
