package shared

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestSetPaused(t *testing.T) {
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)

		go func(val int32) {
			defer wg.Done()

			SetPaused(val)
		}(int32(i % 2)) // will constantly set Paused to 0 or 1
	}

	wg.Wait()

	finalVal := atomic.LoadInt32(&Paused)

	if finalVal != 0 && finalVal != 1 {
		t.Errorf("SetPaused is not concurrent safe, finalVal: %v", finalVal)
	}
}

func TestGetPaused(t *testing.T) {
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(val int32) {
			defer wg.Done()
			SetPaused(val)
		}(int32(i % 2)) // will constantly set Paused to 0 or 1
	}

	// Now launch a bunch of goroutines to call GetPaused.
	// We don't care about the actual value, as long as the call
	// to GetPaused doesn't cause a panic or return an unexpected value.
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val := GetPaused()
			if val != 0 && val != 1 {
				t.Errorf("GetPaused returned an unexpected value: %v", val)
			}
		}()
	}

	wg.Wait()
}
