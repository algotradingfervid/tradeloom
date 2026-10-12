// internal/kite/redirect.go
package kite

import (
	"net/url"
	"regexp"
	"strings"
)

// Redirect is what Kite sends back to the redirect URL after a login.
type Redirect struct {
	RequestToken string
	State        string
	Status       string
}

// maxRedirectInput bounds pasted/callback input; real redirects are well under 1 KiB.
const maxRedirectInput = 4096

var (
	requestTokenRe = regexp.MustCompile(`^[A-Za-z0-9]{8,128}$`)
	schemeRe       = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)
)

func badRedirect(msg string) error { return &LoginError{Code: CodeBadRedirect, Message: msg} }

// ParseRedirect extracts the request_token from a callback query, a pasted URL, a pasted query or a bare token.
// The token is never included in error messages. Host and state are not checked here: the caller compares
// Redirect.State with the state it issued.
func ParseRedirect(input string) (Redirect, error) {
	if len(input) > maxRedirectInput {
		return Redirect{}, badRedirect("That text is too long to be a login address. Copy only the address from the browser's address bar.")
	}
	in := strings.Trim(strings.TrimSpace(input), "\"'<>")
	if in == "" {
		return Redirect{}, badRedirect("Paste the full address from the browser after logging in, or just the request_token.")
	}
	if requestTokenRe.MatchString(in) {
		return Redirect{RequestToken: in}, nil
	}
	if m := schemeRe.FindString(in); m != "" {
		if s := strings.ToLower(m); s != "http://" && s != "https://" {
			return Redirect{}, badRedirect("That address is not a web address. Copy the whole address bar after logging in.")
		}
	}
	q := in
	if i := strings.IndexByte(q, '?'); i >= 0 {
		q = q[i+1:]
	}
	if i := strings.IndexByte(q, '#'); i >= 0 {
		q = q[:i]
	}
	vals, err := url.ParseQuery(q)
	if err != nil {
		return Redirect{}, badRedirect("That address could not be read. Copy the whole address bar after logging in.")
	}
	r := Redirect{
		RequestToken: strings.TrimSpace(vals.Get("request_token")),
		State:        vals.Get("state"),
		Status:       vals.Get("status"),
	}
	if r.Status != "" && r.Status != "success" {
		return Redirect{}, &LoginError{Code: CodeLoginCancelled, Message: "Kite reports the login was not completed (status \"" + r.Status + "\"). Click \"Log in with Kite\" to try again."}
	}
	if !requestTokenRe.MatchString(r.RequestToken) {
		return Redirect{}, badRedirect("No request_token found in the pasted text. After logging in, copy the whole address from the browser's address bar.")
	}
	return r, nil
}
