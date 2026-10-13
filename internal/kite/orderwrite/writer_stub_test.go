//go:build !orders || noorders

package orderwrite

import (
	"context"
	"errors"
	"testing"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

func TestStubRefusesEveryWrite(t *testing.T) {
	w := New(kiteconnect.New("not-a-real-key"))
	ctx := context.Background()
	calls := map[string]func() error{
		"PlaceOrder":      func() error { _, err := w.PlaceOrder(ctx, "regular", kiteconnect.OrderParams{}); return err },
		"ModifyOrder":     func() error { _, err := w.ModifyOrder(ctx, "regular", "1", kiteconnect.OrderParams{}); return err },
		"CancelOrder":     func() error { _, err := w.CancelOrder(ctx, "regular", "1", nil); return err },
		"ExitOrder":       func() error { _, err := w.ExitOrder(ctx, "regular", "1", nil); return err },
		"ConvertPosition": func() error { _, err := w.ConvertPosition(ctx, kiteconnect.ConvertPositionParams{}); return err },
		"PlaceGTT":        func() error { _, err := w.PlaceGTT(ctx, kiteconnect.GTTParams{}); return err },
		"ModifyGTT":       func() error { _, err := w.ModifyGTT(ctx, 1, kiteconnect.GTTParams{}); return err },
		"DeleteGTT":       func() error { _, err := w.DeleteGTT(ctx, 1); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, ErrOrdersNotCompiled) {
			t.Errorf("%s: want ErrOrdersNotCompiled, got %v", name, err)
		}
	}
	if Compiled() {
		t.Fatal("stub build reports Compiled() == true")
	}
}
