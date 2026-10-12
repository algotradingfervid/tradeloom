package candles

import (
	"math"
	"time"
)

// Source of a stored candle (§4.3 column src).
type Source uint8

const (
	SourceHistorical Source = 0
	SourceTick       Source = 1
	SourceManual     Source = 2 // FR-CHART-006 manual refresh
)

// Candle is what the engine and the chart see. Prices are rupees (stored as integer paise).
type Candle struct {
	Start    time.Time `json:"t"` // candle start, exchange time, IST
	Open     float64   `json:"o"`
	High     float64   `json:"h"`
	Low      float64   `json:"l"`
	Close    float64   `json:"c"`
	Volume   int64     `json:"v"`
	OI       int64     `json:"oi"`
	HasOI    bool      `json:"hasOi"`
	Source   Source    `json:"src"`
	Complete bool      `json:"complete"` // false: tick-built or still forming; replaced by historical
}

// ToPaise converts rupees to integer paise (exact for Kite's 0.05 ticks).
func ToPaise(v float64) int64 { return int64(math.Round(v * 100)) }

// FromPaise converts integer paise to rupees.
func FromPaise(p int64) float64 { return float64(p) / 100 }

// Range is a half-open time range [From, To).
type Range struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Coverage is the merged, non-overlapping set of fetched ranges of (Token, Interval).
type Coverage struct {
	Token    uint32   `json:"token"`
	Interval Interval `json:"interval"`
	Ranges   []Range  `json:"ranges"`
}
