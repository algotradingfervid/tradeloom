//go:build !orders || noorders

package orderwrite

import (
	"context"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// Compiled reports whether this binary contains the real order-write implementation.
func Compiled() bool { return false }

type stub struct{}

// New returns the stub Writer; the client is ignored so no order-write method is linked.
func New(_ *kiteconnect.Client) Writer { return stub{} }

func (stub) PlaceOrder(context.Context, string, kiteconnect.OrderParams) (kiteconnect.OrderResponse, error) {
	return kiteconnect.OrderResponse{}, ErrOrdersNotCompiled
}

func (stub) ModifyOrder(context.Context, string, string, kiteconnect.OrderParams) (kiteconnect.OrderResponse, error) {
	return kiteconnect.OrderResponse{}, ErrOrdersNotCompiled
}

func (stub) CancelOrder(context.Context, string, string, *string) (kiteconnect.OrderResponse, error) {
	return kiteconnect.OrderResponse{}, ErrOrdersNotCompiled
}

func (stub) ExitOrder(context.Context, string, string, *string) (kiteconnect.OrderResponse, error) {
	return kiteconnect.OrderResponse{}, ErrOrdersNotCompiled
}

func (stub) ConvertPosition(context.Context, kiteconnect.ConvertPositionParams) (bool, error) {
	return false, ErrOrdersNotCompiled
}

func (stub) PlaceGTT(context.Context, kiteconnect.GTTParams) (kiteconnect.GTTResponse, error) {
	return kiteconnect.GTTResponse{}, ErrOrdersNotCompiled
}

func (stub) ModifyGTT(context.Context, int, kiteconnect.GTTParams) (kiteconnect.GTTResponse, error) {
	return kiteconnect.GTTResponse{}, ErrOrdersNotCompiled
}

func (stub) DeleteGTT(context.Context, int) (kiteconnect.GTTResponse, error) {
	return kiteconnect.GTTResponse{}, ErrOrdersNotCompiled
}
