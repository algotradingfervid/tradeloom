//go:build oracle

package ta

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Every function called from many goroutines at once, while another goroutine
// keeps rewriting the global settings (to their current values, so results
// must not change), returns exactly what a serial call returns. Run it with
// -race to also check the settings locking.
func TestConcurrentCalls(t *testing.T) {
	s := fixtures(500)[1]
	c := &fuzzCase{n: 500, ohlcv: [5][]float64{s.O, s.H, s.L, s.C, s.V}}
	type job struct {
		name string
		fn   reflect.Value
		args []reflect.Value
		want []reflect.Value
	}
	var jobs []job
	for _, e := range fuzzRegistry {
		fi, err := oracle.Info(e.name)
		if err != nil {
			t.Fatal(err)
		}
		jc := *c
		jc.reals, jc.opts = nil, nil
		for _, in := range fi.Inputs {
			if in.Kind == oracle.InputReal {
				jc.reals = append(jc.reals, s.C)
			}
		}
		if e.name == "MAVP" {
			p := make([]float64, 500)
			for i := range p {
				p[i] = float64(2 + i%20)
			}
			jc.reals[1] = p
		}
		for _, oi := range fi.Opts {
			jc.opts = append(jc.opts, oi.Default)
		}
		fn := reflect.ValueOf(e.pub)
		args := fuzzArgs(fn.Type(), fi, &jc, nil)
		want := fn.Call(args)
		if err, _ := want[len(want)-1].Interface().(error); err != nil {
			t.Fatalf("%s: %v", e.name, err)
		}
		jobs = append(jobs, job{e.name, fn, args, want})
	}

	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			SetAllUnstablePeriods(0)
			for st := CandleSettingType(0); st < candleSettingCount; st++ {
				d := defaultCandleSettings[st]
				SetCandleSettings(st, d.RangeType, d.AvgPeriod, d.Factor)
			}
			RestoreAllCandleDefaultSettings()
		}
	}()

	const workers = 16
	var wg sync.WaitGroup
	errs := make(chan string, workers*len(jobs))
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for k := range jobs {
				j := jobs[(k*7+w*13)%len(jobs)]
				got := j.fn.Call(j.args)
				for i := 0; i < len(got)-1; i++ {
					if !reflect.DeepEqual(fmt.Sprint(got[i].Interface()), fmt.Sprint(j.want[i].Interface())) {
						errs <- fmt.Sprintf("worker %d: %s output %d differs from the serial call", w, j.name, i)
					}
				}
			}
		}(w)
	}
	wg.Wait()
	close(stop)
	writer.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
