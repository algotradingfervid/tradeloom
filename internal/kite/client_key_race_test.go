package kite_test

import (
	"context"
	"sync"
	"testing"
)

// Ruling 52 (7): SetAPIKey is safe while calls are in flight (run with -race).
func TestSetAPIKey_ConcurrentWithCalls(t *testing.T) {
	e := newEnv(t)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, _ = e.cli.Profile(context.Background())
				_ = e.cli.APIKey()
			}
		}()
	}
	for _, k := range []string{"key2", "key3", "key4", "key5", "key"} {
		e.cli.SetAPIKey(k)
	}
	wg.Wait()
	if e.cli.APIKey() != "key" {
		t.Fatalf("APIKey() = %q", e.cli.APIKey())
	}
}
