package kiteconnect

import "net/http"

func New(apiKey string) *Client { return &Client{} }

func (c *Client) SetHTTPClient(h *http.Client) {}
