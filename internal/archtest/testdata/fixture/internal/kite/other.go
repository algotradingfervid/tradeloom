package kite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Other builds a gokiteconnect client outside auth.go and client.go, so it could skip the write
// deny-list (g0-kitenew ×2).
func Other() *Client {
	kc := kiteconnect.New("k")
	kc.SetHTTPClient(nil)
	return &Client{api: kc}
}
