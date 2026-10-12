package rawhttp

import (
	"net/http"
	"testing"

	kc "github.com/zerodha/gokiteconnect/v4"
)

// Test files are not loaded (not linked into the binary): neither g0-http nor g0-kitehttp applies.
func TestRaw(t *testing.T) { _ = kc.NewHTTPClient(http.DefaultClient); _ = kc.URICancelOrder }
