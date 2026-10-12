package orderwrite

// Writer is allowed: order-write method names may be declared in internal/kite/orderwrite.
type Writer interface {
	PlaceOrder(variety string) (string, error)
}
