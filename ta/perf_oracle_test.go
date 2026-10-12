//go:build oracle

package ta

import (
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"tradeloom/ta/internal/oracle"
)

// TestPerfVsC times every function against the C library on 200k bars and
// lists the 25 slowest relative to C. Opt-in: TA_PERF=1 go test -tags oracle -run PerfVsC -v
func TestPerfVsC(t *testing.T) {
	if os.Getenv("TA_PERF") == "" {
		t.Skip("set TA_PERF=1")
	}
	const n = 200000
	s := fixtures(n)[1]
	type row struct {
		name   string
		gn, cn time.Duration
	}
	var rows []row
	for _, e := range fuzzRegistry {
		fi, _ := oracle.Info(e.name)
		c := &fuzzCase{n: n, ohlcv: [5][]float64{s.O, s.H, s.L, s.C, s.V}}
		for _, in := range fi.Inputs {
			if in.Kind == oracle.InputReal {
				c.reals = append(c.reals, s.C)
			}
		}
		if e.name == "MAVP" {
			p := make([]float64, n)
			for i := range p {
				p[i] = float64(2 + i%20)
			}
			c.reals[1] = p
		}
		for _, oi := range fi.Opts {
			c.opts = append(c.opts, oi.Default)
		}
		fn := reflect.ValueOf(e.pub)
		args := fuzzArgs(fn.Type(), fi, c, nil)
		best := func(f func()) time.Duration {
			b := time.Duration(1 << 62)
			for k := 0; k < 5; k++ {
				t0 := time.Now()
				f()
				if d := time.Since(t0); d < b {
					b = d
				}
			}
			return b
		}
		gn := best(func() { fn.Call(args) })
		cn := best(func() { oracle.Call(e.name, c.oracleInputs(fi), c.opts...) })
		rows = append(rows, row{e.name, gn, cn})
	}
	sort.Slice(rows, func(i, j int) bool {
		return float64(rows[i].gn)/float64(rows[i].cn) > float64(rows[j].gn)/float64(rows[j].cn)
	})
	var tg, tc time.Duration
	for _, r := range rows {
		tg += r.gn
		tc += r.cn
	}
	t.Logf("total go %v c %v (c includes copying inputs into C memory)", tg, tc)
	for _, r := range rows[:25] {
		t.Logf("%-22s go %10v c %10v ratio %.2f", r.name, r.gn, r.cn, float64(r.gn)/float64(r.cn))
	}
}
