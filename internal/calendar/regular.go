package calendar

// Regular-day phases (FR-DATA-001). These are the only session-time constants in the code base;
// every other package asks the Service (ARCHITECTURE §10.5 "literal session times" check).
var (
	cashRegular = []PhaseSpec{
		{PhasePreOpen, 9*60 + 0, 9*60 + 15},
		{PhaseContinuous, 9*60 + 15, 15*60 + 30},
		{PhaseClosing, 15*60 + 30, 15*60 + 40},
		{PhasePostClosing, 15*60 + 40, 16 * 60},
	}
	derivRegular = []PhaseSpec{
		{PhaseContinuous, 9*60 + 15, 15*60 + 30},
	}
	// instrumentsReady is when Kite publishes the day's instrument dump (A07 §A5: "around 08:30").
	instrumentsReady HM = 8*60 + 30
	// inferenceAfter is the instant after which a missing NIFTY 50 daily candle marks an unlisted
	// weekday as non-trading: end of the regular post-closing phase.
	inferenceAfter HM = 16 * 60
)

func regularFor(e Exchange) []PhaseSpec {
	if IsCash(e) {
		return cashRegular
	}
	return derivRegular
}
