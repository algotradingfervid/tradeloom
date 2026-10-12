// internal/kite/auth.go
package kite

import (
	"context"
	"net/http"
	"net/url"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// AuthSession is the part of kiteconnect.UserSession the app keeps.
type AuthSession struct {
	UserID      string
	AccessToken string
}

// AuthAPI covers the three session calls. None of them is an order write; they are allowed in internal/kite (ARCH §5.2).
type AuthAPI interface {
	LoginURL(state string) string
	GenerateSession(ctx context.Context, requestToken, apiSecret string) (AuthSession, error)
	Invalidate(ctx context.Context, accessToken string) error
}

// NewAuth returns a factory of AuthAPI bound to an API key. baseURI "" keeps https://api.kite.trade (tests pass a fake).
// hc nil gets authHTTPClient(nil): the NFR-04 allow-list transport, never gokiteconnect's default client.
func NewAuth(hc *http.Client, baseURI string) func(apiKey string) AuthAPI {
	hc = authHTTPClient(hc)
	return func(apiKey string) AuthAPI {
		c := kiteconnect.New(apiKey)
		c.SetHTTPClient(hc)
		if baseURI != "" {
			c.SetBaseURI(baseURI)
		}
		return &kcAuth{c: c}
	}
}

// authHTTPClient returns hc, or for nil a 10 s client on NewAllowListTransport(http.DefaultTransport, AllowedHosts).
func authHTTPClient(hc *http.Client) *http.Client {
	if hc != nil {
		return hc
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: NewAllowListTransport(http.DefaultTransport, AllowedHosts)}
}

type kcAuth struct{ c *kiteconnect.Client }

func (a *kcAuth) LoginURL(state string) string {
	return a.c.GetLoginURLWithparams(url.Values{"state": {state}})
}

func (a *kcAuth) GenerateSession(_ context.Context, requestToken, apiSecret string) (AuthSession, error) {
	us, err := a.c.GenerateSession(requestToken, apiSecret)
	if err != nil {
		return AuthSession{}, err
	}
	return AuthSession{UserID: us.UserID, AccessToken: us.AccessToken}, nil
}

func (a *kcAuth) Invalidate(_ context.Context, accessToken string) error {
	a.c.SetAccessToken(accessToken)
	_, err := a.c.InvalidateAccessToken()
	return err
}
