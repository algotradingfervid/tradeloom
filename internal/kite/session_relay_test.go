// internal/kite/session_relay_test.go
package kite

import (
	"testing"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

func TestSessionErrorRelay_ForwardsOnceSet(t *testing.T) {
	var r SessionErrorRelay
	r.Handle(&Err{Kind: KindSession, Op: "Orders"}) // no target yet: must not panic
	var got []error
	r.Set(func(err error) { got = append(got, err) })
	e := &Err{Kind: KindSession, Op: "Orders", Code: 403, Type: kiteconnect.TokenError, Message: "Incorrect `api_key` or `access_token`."}
	r.Handle(e)
	r.Handle(nil)
	if len(got) != 1 || got[0] != e {
		t.Fatalf("got %v", got)
	}
	if !isTokenError(got[0]) {
		t.Fatal("a *kite.Err of KindSession must count as a token error")
	}
}
