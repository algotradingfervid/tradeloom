package instruments

import (
	"cmp"
	"slices"
	"strings"

	"tradeloom/internal/calendar"
)

// Rank tiers of FR-WATCH-008, best first.
const (
	RankExactSymbol   = 0
	RankSymbolPrefix  = 1
	RankDisplayTokens = 2
	RankNameContains  = 3
)

// Result is one search hit.
type Result struct {
	Instrument Instrument `json:"instrument"`
	Rank       int        `json:"rank"`
	AlsoOnNSE  bool       `json:"alsoOnNse"` // BSE row whose company is also listed on NSE
}

func rankOf(e *entry, q string, qt []string) (int, bool) {
	switch {
	case e.sym == q || (e.alias != "" && e.alias == q):
		return RankExactSymbol, true
	case strings.HasPrefix(e.sym, q) || (e.alias != "" && strings.HasPrefix(e.alias, q)):
		return RankSymbolPrefix, true
	case allTokensMatch(e.tokens, qt):
		return RankDisplayTokens, true
	case e.name != "" && strings.Contains(e.name, q):
		return RankNameContains, true
	}
	return 0, false
}

func allTokensMatch(have, want []string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if strings.HasPrefix(h, w) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return len(want) > 0
}

func classOf(in Instrument) int {
	if in.IsFuture() {
		return 1
	}
	return 0
}

func exchangeOrder(t Tag) int {
	switch t {
	case TagIndices:
		return 0
	case TagNSE:
		return 1
	case TagBSE:
		return 2
	case TagNFOFut:
		return 3
	}
	return 4
}

// Search returns at most limit instruments matching q, ranked per FR-WATCH-008: exact symbol >
// symbol prefix > display-name tokens all matched > name contains; on equal rank indices and
// cash equities before futures, NSE before BSE, nearest expiry first, then display name.
// Expired futures never appear, even from a stale list (AC5).
func (m *Master) Search(q string, limit int) []Result {
	q = strings.ToLower(strings.Join(strings.Fields(q), " "))
	if q == "" {
		return nil
	}
	qt := strings.Fields(q)
	today := calendar.DateOf(m.clk.Now())
	// Replace swaps in a fresh slice and never mutates entries, so the snapshot stays valid
	// after the lock is released.
	m.mu.RLock()
	all := m.all
	m.mu.RUnlock()
	var hits []hit
	for i := range all {
		e := &all[i]
		if e.in.IsFuture() && calendar.DateOf(e.in.Expiry).Before(today) {
			continue
		}
		if r, ok := rankOf(e, q, qt); ok {
			hits = append(hits, hit{e: e, rank: r, class: classOf(e.in), exch: exchangeOrder(e.in.Tag), expiry: e.in.Expiry.Unix()})
		}
	}
	if len(hits) == 0 {
		return nil
	}
	// A one-letter query can match the whole master: keep only the best limit hits (bounded
	// max-heap, O(n log limit)) before sorting, so search stays well inside 100 ms (AC3).
	if limit > 0 && len(hits) > limit {
		hits = topK(hits, limit)
	}
	slices.SortFunc(hits, cmpHit)
	out := make([]Result, len(hits))
	for i, h := range hits {
		out[i] = Result{Instrument: h.e.in, Rank: h.rank, AlsoOnNSE: h.e.alsoOnNSE}
	}
	return out
}

// hit is a match with its sort key precomputed.
type hit struct {
	e      *entry
	rank   int
	class  int
	exch   int
	expiry int64
}

// cmpHit orders hits per FR-WATCH-008 (see Search).
func cmpHit(a, b hit) int {
	if c := cmp.Compare(a.rank, b.rank); c != 0 {
		return c
	}
	if c := cmp.Compare(a.class, b.class); c != 0 {
		return c
	}
	if c := cmp.Compare(a.exch, b.exch); c != 0 {
		return c
	}
	if c := cmp.Compare(a.expiry, b.expiry); c != 0 {
		return c
	}
	if c := strings.Compare(a.e.in.DisplayName, b.e.in.DisplayName); c != 0 {
		return c
	}
	return cmp.Compare(a.e.in.Token, b.e.in.Token)
}

// topK returns the k best hits (unordered) using a max-heap of size k whose root is the worst kept hit.
func topK(hits []hit, k int) []hit {
	h := slices.Clone(hits[:k])
	down := func(i int) {
		for {
			w, l, r := i, 2*i+1, 2*i+2
			if l < k && cmpHit(h[l], h[w]) > 0 {
				w = l
			}
			if r < k && cmpHit(h[r], h[w]) > 0 {
				w = r
			}
			if w == i {
				return
			}
			h[i], h[w] = h[w], h[i]
			i = w
		}
	}
	for i := k/2 - 1; i >= 0; i-- {
		down(i)
	}
	for _, x := range hits[k:] {
		if cmpHit(x, h[0]) < 0 {
			h[0] = x
			down(0)
		}
	}
	return h
}
