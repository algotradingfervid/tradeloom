package kite

import (
	"context"
	"errors"
	"fmt"
	"testing"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		err  error
		want Kind
	}{
		{nil, KindNone},
		{kiteconnect.Error{Code: 403, ErrorType: kiteconnect.TokenError}, KindSession},
		{kiteconnect.Error{Code: 403, ErrorType: kiteconnect.PermissionError}, KindPermission},
		{kiteconnect.Error{Code: 429, ErrorType: kiteconnect.NetworkError}, KindRateLimited},
		{kiteconnect.Error{Code: 429, ErrorType: kiteconnect.GeneralError}, KindRateLimited},
		{kiteconnect.NewError(kiteconnect.NetworkError, "Request failed.", nil), KindNetwork},
		{kiteconnect.Error{Code: 400, ErrorType: kiteconnect.InputError}, KindInput},
		{kiteconnect.Error{Code: 400, ErrorType: kiteconnect.DataError}, KindInput},
		{kiteconnect.NewError(kiteconnect.DataError, "Error parsing response.", nil), KindServer},
		{kiteconnect.Error{Code: 400, ErrorType: kiteconnect.OrderError}, KindOrder},
		{kiteconnect.Error{Code: 500, ErrorType: kiteconnect.GeneralError}, KindServer},
		{kiteconnect.Error{Code: 502, ErrorType: ""}, KindServer},
		{context.Canceled, KindCanceled},
		{fmt.Errorf("wrapped: %w", kiteconnect.Error{Code: 403, ErrorType: kiteconnect.TokenError}), KindSession},
		{errors.New("boom"), KindUnknown},
	}
	for i, c := range cases {
		if got := Classify(c.err); got != c.want {
			t.Errorf("case %d (%v): %v, want %v", i, c.err, got, c.want)
		}
	}
}

func TestErrKeepsKiteFieldsAndNoHeaders(t *testing.T) {
	e := wrap("Historical", kiteconnect.Error{Code: 400, ErrorType: kiteconnect.InputError, Message: "interval exceeds max limit: 60 days"})
	if e.Kind != KindInput || e.Code != 400 || e.Op != "Historical" {
		t.Fatalf("%+v", e)
	}
	if got := e.Error(); got != "kite Historical: interval exceeds max limit: 60 days (input, 400 InputException)" {
		t.Fatalf("Error() = %q", got)
	}
}
