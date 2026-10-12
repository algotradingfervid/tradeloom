package candles

import (
	"sort"
	"time"
)

// Merge returns rs sorted, with overlapping or touching ranges joined and empty ones dropped.
func Merge(rs []Range) []Range {
	var in []Range
	for _, r := range rs {
		if r.To.After(r.From) {
			in = append(in, r)
		}
	}
	sort.Slice(in, func(i, j int) bool { return in[i].From.Before(in[j].From) })
	var out []Range
	for _, r := range in {
		if n := len(out); n > 0 && !r.From.After(out[n-1].To) {
			if r.To.After(out[n-1].To) {
				out[n-1].To = r.To
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

// Subtract returns the parts of want not covered by have (have need not be merged).
func Subtract(want Range, have []Range) []Range {
	if !want.To.After(want.From) {
		return nil
	}
	cur := want.From
	var out []Range
	for _, h := range Merge(have) {
		if !h.To.After(cur) {
			continue
		}
		if !h.From.Before(want.To) {
			break
		}
		if h.From.After(cur) {
			out = append(out, Range{cur, h.From})
		}
		cur = h.To
		if !cur.Before(want.To) {
			return out
		}
	}
	return append(out, Range{cur, want.To})
}

// Split cuts r into consecutive windows of at most max.
func Split(r Range, max time.Duration) []Range {
	var out []Range
	for f := r.From; f.Before(r.To); f = f.Add(max) {
		t := f.Add(max)
		if t.After(r.To) {
			t = r.To
		}
		out = append(out, Range{f, t})
	}
	return out
}
