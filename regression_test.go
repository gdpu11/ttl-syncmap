package ttlsyncmap

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestLoadOrStoreAtomic(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := "missing"
		if expired {
			name = "expired"
		}
		t.Run(name, func(t *testing.T) {
			m := testMap(t)
			for round := 0; round < 20; round++ {
				if expired {
					m.Store(round, -1)
					expire(t, m, round)
				}
				start := make(chan struct{})
				results := make(chan interface{}, 128)
				var wg sync.WaitGroup
				var stored int32
				for i := 0; i < 128; i++ {
					wg.Add(1)
					go func(value int) {
						defer wg.Done()
						<-start
						actual, loaded := m.LoadOrStore(round, value)
						if !loaded {
							atomic.AddInt32(&stored, 1)
						}
						results <- actual
					}(i)
				}
				close(start)
				wg.Wait()
				close(results)
				if stored != 1 {
					t.Fatalf("round %d: %d callers stored the same key; want 1", round, stored)
				}
				want, ok := m.Load(round)
				if !ok {
					t.Fatal("stored value is missing")
				}
				for actual := range results {
					if actual != want {
						t.Fatalf("LoadOrStore returned %v, want winner %v", actual, want)
					}
				}
			}
		})
	}
}
