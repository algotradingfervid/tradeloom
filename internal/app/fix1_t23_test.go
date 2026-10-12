package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"

	"tradeloom/internal/service/servicetest"
)

// Ruling 89 (I1): the restart re-exec lives in internal/app (cmd imports no syscall).
func TestReExec_UsesExecHook(t *testing.T) {
	var got []any
	prev := execFn
	t.Cleanup(func() { execFn = prev })
	execFn = func(exe string, args, env []string) error {
		got = []any{exe, args, env}
		return errors.New("exec refused")
	}
	err := ReExec("/opt/tradeloom/tradeloom", []string{"tradeloom", "--x"}, []string{"A=1"})
	if err == nil || err.Error() != "exec refused" {
		t.Fatalf("err %v", err)
	}
	want := []any{"/opt/tradeloom/tradeloom", []string{"tradeloom", "--x"}, []string{"A=1"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

// Ruling 89 (M2): a second launch before the runtime exists (zero Runtime) neither panics nor routes.
func TestSecondInstance_NilRuntimeReturnsEarly(t *testing.T) {
	svc, fs, _ := servicetest.New(t)
	a := New(Deps{Service: svc})
	a.mu.Lock()
	a.rt = Runtime{}
	a.ctx = context.Background()
	a.mu.Unlock()
	a.SecondInstance(options.SecondInstanceData{Args: []string{"http://127.0.0.1:7420/kite/callback?request_token=r&state=s"}})
	if len(fs.Forwarded()) != 0 {
		t.Fatal("routed without a runtime")
	}
}
