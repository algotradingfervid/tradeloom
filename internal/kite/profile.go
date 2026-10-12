// internal/kite/profile.go
package kite

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"unicode"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// Profile is the FR-ACCT-001 view model. Read-only by construction (a copy).
type Profile struct {
	UserID     string   `json:"userId"`
	Name       string   `json:"name"`
	ShortName  string   `json:"shortName"`
	Email      string   `json:"email"`
	Broker     string   `json:"broker"`
	AvatarURL  string   `json:"avatarUrl"`
	Initials   string   `json:"initials"`
	Exchanges  []string `json:"exchanges"`
	Products   []string `json:"products"`
	OrderTypes []string `json:"orderTypes"`
}

func ProfileFrom(p kiteconnect.UserProfile) Profile {
	broker := p.Broker
	if strings.EqualFold(broker, "zerodha") || broker == "" {
		broker = "Zerodha"
	}
	return Profile{
		UserID: p.UserID, Name: p.UserName, ShortName: p.UserShortName, Email: p.Email, Broker: broker,
		AvatarURL: allowedAvatar(p.AvatarURL), Initials: initials(p.UserName),
		Exchanges: append([]string(nil), p.Exchanges...), Products: append([]string(nil), p.Products...),
		OrderTypes: append([]string(nil), p.OrderTypes...),
	}
}

// LogValue keeps PII out of logs if a Profile is ever logged.
func (p Profile) LogValue() slog.Value {
	return slog.GroupValue(slog.String("user", MaskUserID(p.UserID)))
}

func allowedAvatar(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() != "kite.zerodha.com" {
		return ""
	}
	return raw
}

func initials(name string) string {
	var out []rune
	for _, w := range strings.Fields(name) {
		for _, r := range w {
			out = append(out, unicode.ToUpper(r))
			break
		}
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}

// Profile fetches GET /user/profile (FR-ACCT-001).
func (s *Session) Profile(ctx context.Context) (Profile, error) {
	if s.State().State != StateReady {
		return Profile{}, &LoginError{Code: CodeNotLoggedIn, Message: "Log in to Kite to see your profile."}
	}
	p, err := s.d.Profiles.Profile(ctx)
	if err != nil {
		s.HandleTokenError(err)
		msg := "network error"
		var ke kiteconnect.Error
		if errors.As(err, &ke) {
			msg = ke.Message
		}
		return Profile{}, &LoginError{Code: CodeKiteError, Message: "Could not load the profile from Kite: " + msg}
	}
	return ProfileFrom(p), nil
}
