package orderwrite

import (
	"context"
	"errors"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// ErrOrdersNotCompiled is returned by every Writer method in builds without the orders tag.
// The UI shows its text verbatim.
var ErrOrdersNotCompiled = errors.New("Order placement is not included in this build")

// Writer is the Kite order-write surface. Parameter and result types are gokiteconnect v4's.
type Writer interface {
	PlaceOrder(ctx context.Context, variety string, p kiteconnect.OrderParams) (kiteconnect.OrderResponse, error)
	ModifyOrder(ctx context.Context, variety, orderID string, p kiteconnect.OrderParams) (kiteconnect.OrderResponse, error)
	CancelOrder(ctx context.Context, variety, orderID string, parentOrderID *string) (kiteconnect.OrderResponse, error)
	ExitOrder(ctx context.Context, variety, orderID string, parentOrderID *string) (kiteconnect.OrderResponse, error)
	ConvertPosition(ctx context.Context, p kiteconnect.ConvertPositionParams) (bool, error)
	PlaceGTT(ctx context.Context, p kiteconnect.GTTParams) (kiteconnect.GTTResponse, error)
	ModifyGTT(ctx context.Context, triggerID int, p kiteconnect.GTTParams) (kiteconnect.GTTResponse, error)
	DeleteGTT(ctx context.Context, triggerID int) (kiteconnect.GTTResponse, error)
}
