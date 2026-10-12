// Package kiteconnect is a minimal stand-in for gokiteconnect used only by archtest fixtures.
package kiteconnect

type Client struct{}
type OrderParams struct{}

func (c *Client) PlaceOrder(variety string, p OrderParams) (string, error)       { return "", nil }
func (c *Client) ModifyOrder(variety, id string, p OrderParams) (string, error)  { return "", nil }
func (c *Client) GetOrders() ([]string, error)                                   { return nil, nil }
func (c *Client) CancelOrder(variety, id string, parent *string) (string, error) { return "", nil }

// Alert (alert-triggered order baskets) and mutual-fund order writes (Ruling 47).
type AlertParams struct{}

func (c *Client) CreateAlert(p AlertParams) (string, error)              { return "", nil }
func (c *Client) ModifyAlert(uuid string, p AlertParams) (string, error) { return "", nil }
func (c *Client) DeleteAlerts(uuids ...string) error                     { return nil }
func (c *Client) GetAlerts() ([]string, error)                           { return nil, nil }
func (c *Client) PlaceMFOrder(p OrderParams) (string, error)             { return "", nil }
func (c *Client) CancelMFOrder(id string) (string, error)                { return "", nil }
func (c *Client) PlaceMFSIP(p OrderParams) (string, error)               { return "", nil }
func (c *Client) ModifyMFSIP(id string, p OrderParams) (string, error)   { return "", nil }
func (c *Client) CancelMFSIP(id string) (string, error)                  { return "", nil }
