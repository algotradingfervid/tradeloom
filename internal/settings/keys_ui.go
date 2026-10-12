// internal/settings/keys_ui.go
package settings

import (
	"errors"
	"strings"
	"time"
)

// KiteIntervals are the eight native Kite candle intervals (chart timeframes).
var KiteIntervals = []string{"minute", "3minute", "5minute", "10minute", "15minute", "30minute", "60minute", "day"}

// ValidateRoute accepts an in-app route: "/" followed by path characters, no whitespace.
func ValidateRoute(v string) error {
	if !strings.HasPrefix(v, "/") || strings.ContainsAny(v, " \t\r\n") {
		return errors.New("must be an in-app route starting with /")
	}
	if strings.HasPrefix(v, "//") {
		return errors.New("must not start with //")
	}
	for _, seg := range strings.Split(v, "/") {
		if seg == ".." {
			return errors.New("must not contain .. segments")
		}
	}
	return nil
}

// ValidateDateOrEmpty accepts "" or a YYYY-MM-DD date.
func ValidateDateOrEmpty(v string) error {
	if v == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", v); err != nil {
		return errors.New("must be a date formatted YYYY-MM-DD")
	}
	return nil
}

// UI settings keys of contract §E (P0-4 A7). Declared through Register so the generic façade lists them.
var (
	AppearanceTheme            = Register(Key[string]{Name: "appearance.theme", Default: "light", Validate: OneOf("light", "dark")})
	AppearanceFollowSystem     = Register(Key[bool]{Name: "appearance.follow_system", Default: false})
	AppearanceSidebarCollapsed = Register(Key[bool]{Name: "appearance.sidebar_collapsed", Default: false})
	UILastRoute                = Register(Key[string]{Name: "ui.last_route", Default: "/home", Validate: ValidateRoute})
	ChartDefaultTimeframe      = Register(Key[string]{Name: "chart.default_timeframe", Default: "60minute", Validate: OneOf(KiteIntervals...)})
	CalendarStaleToastDay      = Register(Key[string]{Name: "calendar.stale_toast_day", Default: "", Validate: ValidateDateOrEmpty})
)
