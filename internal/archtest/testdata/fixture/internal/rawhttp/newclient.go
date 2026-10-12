package rawhttp

import (
	"net/http"

	kc "github.com/zerodha/gokiteconnect/v4"
)

// Outside internal/kite: a function value of New and a method value of SetHTTPClient (g0-kitenew ×2).
var newClient = kc.New

func install(c *kc.Client) func(*http.Client) { return c.SetHTTPClient }
