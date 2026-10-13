package calendar

import "time"

// Exchange is a Kite exchange code. INDICES rows use their Kite exchange (NSE or BSE).
type Exchange string

const (
	NSE Exchange = "NSE"
	BSE Exchange = "BSE"
	NFO Exchange = "NFO"
	BFO Exchange = "BFO"
)

// Exchanges lists the exchanges the calendar models.
var Exchanges = []Exchange{NSE, BSE, NFO, BFO}

func validExchange(e Exchange) bool {
	for _, x := range Exchanges {
		if x == e {
			return true
		}
	}
	return false
}

// parent maps a derivatives exchange to the cash exchange whose list it follows.
func parent(e Exchange) (Exchange, bool) {
	switch e {
	case NFO:
		return NSE, true
	case BFO:
		return BSE, true
	}
	return "", false
}

// IsCash reports whether e is a cash-equity exchange (pre-open and post-closing apply).
func IsCash(e Exchange) bool { return e == NSE || e == BSE }

// PhaseKind names a session phase.
type PhaseKind string

const (
	PhasePreOpen     PhaseKind = "pre-open"
	PhaseContinuous  PhaseKind = "continuous"
	PhaseClosing     PhaseKind = "closing"
	PhasePostClosing PhaseKind = "post-closing"
	PhaseClosed      PhaseKind = "closed"
)

func validPhase(k PhaseKind) bool {
	switch k {
	case PhasePreOpen, PhaseContinuous, PhaseClosing, PhasePostClosing:
		return true
	}
	return false
}

// DayType classifies a date for one exchange.
type DayType string

const (
	DayTrading        DayType = "trading"
	DayHoliday        DayType = "holiday"
	DaySpecial        DayType = "special"
	DayWeekend        DayType = "weekend"
	DayInferredClosed DayType = "inferred-non-trading"
)

// PhaseSpec is a phase as written in the calendar file: kind and HH:MM bounds [Start, End).
type PhaseSpec struct {
	Kind  PhaseKind `json:"kind"`
	Start HM        `json:"start"`
	End   HM        `json:"end"`
}

// Entry is one calendar list entry for (Exchange, Date): a holiday, a special session with its
// phases, a trading day (no phases = regular phases) or an inferred non-trading day.
type Entry struct {
	Date     Date
	Exchange Exchange
	Type     DayType
	Phases   []PhaseSpec
	Source   string // "shipped", "import", "user", "inferred"
}

// Phase is a concrete phase on a date: [Start, End) in IST.
type Phase struct {
	Kind  PhaseKind `json:"kind"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Day is the resolved calendar for (Exchange, Date).
type Day struct {
	Exchange Exchange `json:"exchange"`
	Date     Date     `json:"-"`
	Type     DayType  `json:"type"`
	Phases   []Phase  `json:"phases"`
}

// IsTrading reports whether the exchange trades on this day.
func (d Day) IsTrading() bool { return d.Type == DayTrading || d.Type == DaySpecial }

// Phase returns the phase of kind k, if the day has one.
func (d Day) Phase(k PhaseKind) (Phase, bool) {
	for _, p := range d.Phases {
		if p.Kind == k {
			return p, true
		}
	}
	return Phase{}, false
}
