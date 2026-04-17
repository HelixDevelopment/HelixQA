package browser

import (
	"context"
	"sync"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	d := &mockDriver{kind: EngineChromedp}
	e, err := NewEngine(d, Config{Engine: EngineChromedp})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestNewPool_Validation(t *testing.T) {
	if _, err := NewPool(nil, 1); err == nil {
		t.Error("nil adapter must be rejected")
	}
	if _, err := NewPool(newTestEngine(t), 0); err == nil {
		t.Error("size 0 must be rejected")
	}
	if _, err := NewPool(newTestEngine(t), -1); err == nil {
		t.Error("negative size must be rejected")
	}
}

func TestPool_AcquireRelease(t *testing.T) {
	pool, err := NewPool(newTestEngine(t), 2)
	if err != nil {
		t.Fatal(err)
	}
	s1, err := pool.Acquire(context.Background(), nexus.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s2, err := pool.Acquire(context.Background(), nexus.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if pool.Active() != 2 {
		t.Errorf("Active = %d, want 2", pool.Active())
	}
	pool.Release(s1)
	if pool.Active() != 1 {
		t.Errorf("Active after first release = %d, want 1", pool.Active())
	}
	pool.Release(s2)
	if pool.Active() != 0 {
		t.Errorf("Active after all release = %d, want 0", pool.Active())
	}
}

func TestPool_AcquireBlocksUntilRelease(t *testing.T) {
	pool, _ := NewPool(newTestEngine(t), 1)
	s1, _ := pool.Acquire(context.Background(), nexus.SessionOptions{})

	acquired := make(chan nexus.Session, 1)
	go func() {
		s2, err := pool.Acquire(context.Background(), nexus.SessionOptions{})
		if err != nil {
			t.Errorf("second Acquire failed: %v", err)
			return
		}
		acquired <- s2
	}()

	select {
	case <-acquired:
		t.Fatal("second Acquire should block while pool is full")
	case <-time.After(20 * time.Millisecond):
	}

	pool.Release(s1)
	select {
	case s2 := <-acquired:
		pool.Release(s2)
	case <-time.After(2 * time.Second):
		t.Fatal("second Acquire did not unblock after Release")
	}
}

func TestPool_AcquireRespectsContext(t *testing.T) {
	pool, _ := NewPool(newTestEngine(t), 1)
	s1, _ := pool.Acquire(context.Background(), nexus.SessionOptions{})
	defer pool.Release(s1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := pool.Acquire(ctx, nexus.SessionOptions{}); err == nil {
		t.Fatal("Acquire should honour context deadline")
	}
}

func TestPool_ClosedRejectsAcquire(t *testing.T) {
	pool, _ := NewPool(newTestEngine(t), 1)
	pool.Close()
	if _, err := pool.Acquire(context.Background(), nexus.SessionOptions{}); err == nil {
		t.Fatal("closed pool should reject Acquire")
	}
}

func TestPool_StressNoLeaks(t *testing.T) {
	pool, _ := NewPool(newTestEngine(t), 4)

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := pool.Acquire(context.Background(), nexus.SessionOptions{})
			if err != nil {
				t.Errorf("acquire: %v", err)
				return
			}
			pool.Release(s)
		}()
	}
	wg.Wait()
	if pool.Active() != 0 {
		t.Errorf("Active after stress = %d, want 0", pool.Active())
	}
}
