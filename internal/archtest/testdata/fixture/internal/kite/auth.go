package kite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// NewAuth is allowed: kiteconnect.New and SetHTTPClient in internal/kite/auth.go (g0-kitenew).
func NewAuth() *Client {
	kc := kiteconnect.New("k")
	kc.SetHTTPClient(nil)
	return &Client{api: kc}
}
