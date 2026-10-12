// internal/app/contract_test.go
package app

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"tradeloom/internal/service"
	"tradeloom/internal/service/servicetest"
)

// Contract §D: the UI event catalogue, in order.
var contractEvents = []string{
	"session.state", "feed.state", "tick.batch", "candle.forming", "candle.closed", "clock.offset", "backfill.progress",
	"limits.usage", "orders.changed", "positions.changed", "holdings.changed", "order.update", "watchlists.changed",
	"settings.changed", "market.status", "job.progress", "app.navigate",
}

// Contract §C: the bound structs, in Bind order.
var contractBindings = []string{
	"*api.Session", "*api.Portfolio", "*api.Settings", "*api.Chart", "*api.Instruments", "*api.Quotes",
	"*api.Watchlists", "*api.Data", "*api.Calendar", "*api.Diagnostics", "*api.System",
}

func TestContract_EventCatalogue(t *testing.T) {
	cat := service.EventCatalogue()
	if len(cat) != len(contractEvents) {
		t.Fatalf("%d events, want %d", len(cat), len(contractEvents))
	}
	for i, spec := range cat {
		if spec.Name != contractEvents[i] {
			t.Fatalf("event %d = %q, want %q", i, spec.Name, contractEvents[i])
		}
	}
}

func TestContract_BoundStructs(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	a := New(Deps{Service: svc})
	o := Options(a, fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}, "tradeloom-0123456789ab", false)
	var got []string
	for _, b := range o.Bind {
		got = append(got, fmt.Sprintf("%T", b))
	}
	if strings.Join(got, ",") != strings.Join(contractBindings, ",") {
		t.Fatalf("bound %v\nwant  %v", got, contractBindings)
	}
}

// Ruling 76: the committed frontend/src/api/events.gen.ts must equal what the go:generate line in events.go
// produces now, so a payload change without `make bindings` fails CI. The generator is a main package, so it
// is run exactly as go:generate runs it (same tags), into a temporary file.
func TestContract_EventsGenIsCurrent(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("the go command is needed to run the events generator: %v", err)
	}
	out := filepath.Join(t.TempDir(), "events.gen.ts")
	cmd := exec.CommandContext(t.Context(), goBin, "run", "-tags", "webkit2_41,noorders", "./eventsgen", "-out", out)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go run ./eventsgen: %v\n%s", err, msg)
	}
	want, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "api", "events.gen.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("frontend/src/api/events.gen.ts is stale: run `make bindings` and commit the result (committed %d bytes, generator %d bytes)", len(got), len(want))
	}
}
