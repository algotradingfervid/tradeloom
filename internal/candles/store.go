package candles

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Store is the candle store swap point (§4.3). candles/sqlite implements it; a colfile
// implementation would replace it if S0-2 fails.
type Store interface {
	// Read returns stored candles of a native interval with Start in [from, to), ascending.
	Read(ctx context.Context, token uint32, iv Interval, from, to time.Time) ([]Candle, error)
	// ReadResampled returns week/month/non-native intervals built from the base interval,
	// anchored per the calendar (FR-DATA-002).
	ReadResampled(ctx context.Context, token uint32, target Interval, from, to time.Time) ([]Candle, error)
	// Upsert writes candles idempotently on (token, start) and updates monthly checksums. A
	// tick-built candle never overwrites a complete historical one. Coverage is unchanged.
	Upsert(ctx context.Context, token uint32, iv Interval, cs []Candle, src Source) error
	// UpsertCovered is Upsert plus marking window as fetched, in one transaction (backfill).
	UpsertCovered(ctx context.Context, token uint32, iv Interval, window Range, cs []Candle, src Source) error
	// Missing returns the parts of [from, to) not covered by fetched ranges.
	Missing(ctx context.Context, token uint32, iv Interval, from, to time.Time) ([]Range, error)
	// Coverage returns the merged fetched ranges.
	Coverage(ctx context.Context, token uint32, iv Interval) (Coverage, error)
	// Checksum returns the hex SHA-256 of the complete candles of (token, iv, month yyyymm).
	Checksum(ctx context.Context, token uint32, iv Interval, month int) (string, error)
	// Stats returns sizes per interval and the configured budget.
	Stats(ctx context.Context) (Stats, error)
	// SetBudget stores the size budget in bytes (0 = none).
	SetBudget(ctx context.Context, bytes int64) error
	// AddRunRef records that backtest run runID used (token, iv, r) (purge guard).
	AddRunRef(ctx context.Context, runID string, token uint32, iv Interval, r Range) error
	// Purge deletes candles of rule.Interval starting before rule.Before. If stored backtest runs
	// reference that data and are not all listed in confirmRuns, nothing is deleted and
	// *RunsReferencedError names them (FR-DATA-002 AC3).
	Purge(ctx context.Context, rule PurgeRule, confirmRuns []string) error
	// Clear deletes every candle, coverage range, checksum and backfill job, with the same guard.
	Clear(ctx context.Context, confirmRuns []string) error
}

// Stats is shown in Settings → Data.
type Stats struct {
	Intervals   []IntervalStats `json:"intervals"`
	TotalBytes  int64           `json:"totalBytes"`
	BudgetBytes int64           `json:"budgetBytes"`
}

// IntervalStats is the size of one interval's table.
type IntervalStats struct {
	Interval Interval `json:"interval"`
	Rows     int64    `json:"rows"`
	Bytes    int64    `json:"bytes"`
}

// PurgeRule removes Interval candles that start before Before.
type PurgeRule struct {
	Interval Interval  `json:"interval"`
	Before   time.Time `json:"before"`
}

// RunsReferencedError lists the backtest runs a purge would affect.
type RunsReferencedError struct{ Runs []string }

func (e *RunsReferencedError) Error() string {
	return fmt.Sprintf("candles: purge would remove data used by backtest runs: %s", strings.Join(e.Runs, ", "))
}
