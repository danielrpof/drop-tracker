package artistart

import (
	"sync"
	"testing"
)

func TestActivityGate_FreshGateIsNotActive(t *testing.T) {
	g := NewActivityGate()
	if g.Active() {
		t.Fatalf("Active() = true, want false for a fresh gate")
	}
}

func TestActivityGate_ActiveWhileBegunNotEnded(t *testing.T) {
	g := NewActivityGate()
	end := g.Begin()
	if !g.Active() {
		t.Fatalf("Active() = false, want true after Begin() with no matching end()")
	}
	end()
	if g.Active() {
		t.Fatalf("Active() = true, want false after end() was invoked")
	}
}

func TestActivityGate_TwoConcurrentBeginsBothMustEnd(t *testing.T) {
	g := NewActivityGate()
	end1 := g.Begin()
	end2 := g.Begin()
	if !g.Active() {
		t.Fatalf("Active() = false, want true with two Begin()s in flight")
	}

	end1()
	if !g.Active() {
		t.Fatalf("Active() = false, want true -- ending one of two in-flight Begin()s must still report active")
	}

	end2()
	if g.Active() {
		t.Fatalf("Active() = true, want false once both Begin()s have ended")
	}
}

func TestActivityGate_DoubleEndDoesNotCorruptState(t *testing.T) {
	g := NewActivityGate()
	end := g.Begin()
	end()
	end() // second call must be a no-op, not a second decrement

	if g.Active() {
		t.Fatalf("Active() = true after a double end() call, want false")
	}

	// A fresh Begin/end cycle afterward must still behave correctly -- a
	// double-end must never drive the counter negative in a way that
	// corrupts future accounting.
	end2 := g.Begin()
	if !g.Active() {
		t.Fatalf("Active() = false, want true after a fresh Begin() following an earlier double-end()")
	}
	end2()
	if g.Active() {
		t.Fatalf("Active() = true, want false after the fresh Begin()'s end() was invoked")
	}
}

func TestActivityGate_ConcurrentUse(t *testing.T) {
	g := NewActivityGate()
	const goroutines = 50
	const cyclesPerGoroutine = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < cyclesPerGoroutine; j++ {
				end := g.Begin()
				_ = g.Active()
				end()
			}
		}()
	}

	// A separate goroutine polls Active() concurrently, exercising the
	// -race detector against simultaneous Begin/end/Active calls.
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				_ = g.Active()
			}
		}
	}()

	wg.Wait()
	close(done)

	if g.Active() {
		t.Fatalf("Active() = true after every goroutine's Begin()/end() pairs completed, want false")
	}
}
