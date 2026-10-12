// Package kiteconnect is a minimal stand-in for gokiteconnect used only by archtest fixtures.
package kiteconnect

type Client struct{}
type OrderParams struct{}

func (c *Client) PlaceOrder(variety string, p OrderParams) (string, error)      { return "", nil }
func (c *Client) ModifyOrder(variety, id string, p OrderParams) (string, error) { return "", nil }
func (c *Client) GetOrders() ([]string, error)                                  { return nil, nil }
