// internal/service/system.go
package service

import (
	"bufio"
	"bytes"
	"context"
	"runtime"
	"runtime/debug"
	"strings"

	"tradeloom/internal/kite"
)

const systemStage = "system"

// Licence is one third-party component for the About panel.
type Licence struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Licence string `json:"licence"`
}

// AppInfo is System.Info.
type AppInfo struct {
	Version       string    `json:"version"`
	Build         string    `json:"build"`
	SchemaVersion int       `json:"schemaVersion"`
	DataDir       string    `json:"dataDir"`
	Licences      []Licence `json:"licences"`
}

// StatusSnapshot is System.Status: what the status bar shows before the first events arrive. Feed and
// Clock are the same ms-only DTOs the feed.state and clock.offset events carry (Ruling 71: no time.Time
// on the wire).
type StatusSnapshot struct {
	Feed   FeedStateDTO `json:"feed"`
	Market MarketStatus `json:"market"`
	Clock  OffsetDTO    `json:"clock"`
}

// ParseNotices reads THIRD_PARTY_NOTICES.md: sections "## <module> <version> — <licence>" (Go modules and,
// as scripts/gen-notices.sh writes them, npm packages too) and npm lines "[name@version](repository) - <licence>"
// (license-checker --markdown). Lines inside ``` fences are licence texts and are never parsed, so a heading
// inside a third-party licence cannot appear as a component.
func ParseNotices(md []byte) []Licence {
	out := []Licence{}
	sc := bufio.NewScanner(bytes.NewReader(md))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	fenced := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		switch {
		case strings.HasPrefix(line, "## "):
			head, lic, ok := strings.Cut(strings.TrimPrefix(line, "## "), " — ")
			if !ok {
				continue
			}
			name, ver, _ := strings.Cut(strings.TrimSpace(head), " ")
			out = append(out, Licence{Name: name, Version: strings.TrimSpace(ver), Licence: strings.TrimSpace(lic)})
		case strings.HasPrefix(line, "[") && strings.Contains(line, "](") && strings.Contains(line, ") - "):
			ref, lic, _ := strings.Cut(line[1:], ") - ")
			pkg, _, _ := strings.Cut(ref, "](")
			at := strings.LastIndex(pkg, "@")
			if at <= 0 {
				continue
			}
			out = append(out, Licence{Name: pkg[:at], Version: pkg[at+1:], Licence: strings.TrimSpace(lic)})
		}
	}
	return out
}

// buildString is "go1.25.x tags=webkit2_41,noorders" (tags from the build info; "none" when absent).
func buildString() string {
	tags := "none"
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "-tags" && s.Value != "" {
				tags = s.Value
			}
		}
	}
	return runtime.Version() + " tags=" + tags
}

// AppInfo is System.Info (FR-SHELL-019 About): version, build, schema version, data directory and licences.
func (s *Service) AppInfo(_ context.Context) AppInfo {
	info := AppInfo{Version: s.d.AppVersion, Build: buildString(), DataDir: s.d.Layout.Root, Licences: ParseNotices(s.d.Licences)}
	if s.d.DB != nil {
		info.SchemaVersion = s.d.DB.SchemaVersion()
	}
	return info
}

// Status is System.Status: the last feed state, the NSE market status and the clock offset, in one call.
func (s *Service) Status(_ context.Context) StatusSnapshot {
	return StatusSnapshot{Feed: FeedStateDTOFrom(s.FeedState()), Market: s.MarketStatus(), Clock: OffsetDTOFrom(s.d.Clock.Offset())}
}

// ReconnectFeed is the status indicator's "Retry now" (FR-ACCT-007 degraded states).
func (s *Service) ReconnectFeed(_ context.Context) error {
	if s.d.Session.State().State != kite.StateReady {
		return s.refuse(systemStage, refused(systemStage, "not_logged_in", "Log in before reconnecting the market feed."))
	}
	if s.d.FeedCtl == nil {
		return s.refuse(systemStage, refused(systemStage, "unavailable", "The market feed is not available in this build."))
	}
	s.d.FeedCtl.Reconnect()
	return nil
}
