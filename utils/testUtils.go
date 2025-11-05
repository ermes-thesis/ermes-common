package utils

import (
	"sync"
	"testing"
	"time"
)

// WaitForCalls allows to wait for N calls.
// Returns (notifyFn, waitFn):
func WaitForCalls(t *testing.T, expected int, timeout time.Duration) (func(), func()) {
	t.Helper()

	var wg sync.WaitGroup
	wg.Add(expected)

	notify := func() {
		wg.Done()
	}

	wait := func() {
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(timeout):
			t.Fatalf("timeout waiting for %d calls", expected)
		}
	}

	return notify, wait
}
