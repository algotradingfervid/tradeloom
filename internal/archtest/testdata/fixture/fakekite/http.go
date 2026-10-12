package kiteconnect

import "net/http"

// HTTPResponse, HTTPClient, httpClient and NewHTTPClient mirror gokiteconnect v4's raw HTTP helper (Ruling 69).
type HTTPResponse struct{ Body []byte }

type HTTPClient interface {
	Do(method, rURL string) (HTTPResponse, error)
	GetClient() *httpClient
}

type httpClient struct{ client *http.Client }

func NewHTTPClient(h *http.Client) HTTPClient { return &httpClient{client: h} }

func (h *httpClient) Do(method, rURL string) (HTTPResponse, error) { return HTTPResponse{}, nil }
func (h *httpClient) GetClient() *httpClient                       { return h }

// Write endpoints (banned outside orders-tagged internal/kite/orderwrite files) and two read endpoints.
const (
	URIPlaceOrder       = "/orders/%s"
	URIModifyOrder      = "/orders/%s/%s"
	URICancelOrder      = "/orders/%s/%s"
	URIConvertPosition  = "/portfolio/positions"
	URIInitHoldingsAuth = "/portfolio/holdings/authorise"
	URIPlaceGTT         = "/gtt/triggers"
	URIModifyGTT        = "/gtt/triggers/%d"
	URIDeleteGTT        = "/gtt/triggers/%d"
	URIAlerts           = "/alerts"
	URIAlert            = "/alerts/%s"
	URIPlaceMFOrder     = "/mf/orders"
	URICancelMFOrder    = "/mf/orders/%s"
	URIPlaceMFSIP       = "/mf/sips"
	URIModifyMFSIP      = "/mf/sips/%s"
	URICancelMFSIP      = "/mf/sips/%s"

	URIGetOrders    = "/orders"
	URIGetPositions = "/portfolio/positions"
)
