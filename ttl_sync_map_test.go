package ttlsyncmap

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testMap(t *testing.T) *TTLSyncMap {
	t.Helper()
	m := New(time.Hour)
	t.Cleanup(m.Close)
	return m
}

// Publish an already-expired immutable snapshot, without sleeping or racing
// with readers. Real clock expiration is tested separately below.
func expire(t *testing.T, m *TTLSyncMap, key interface{}) (*entry, *ttlVal) {
	t.Helper()
	item, ok := m.current().Load(key)
	if !ok {
		t.Fatalf("missing key %v", key)
	}
	e := item.(*entry)
	e.mu.Lock()
	v := &ttlVal{val: e.value.Load().(*ttlVal).val, expireAt: time.Now().Add(-time.Hour)}
	e.value.Store(v)
	e.mu.Unlock()
	return e, v
}

func assertValue(t *testing.T, m *TTLSyncMap, key, want interface{}) {
	t.Helper()
	if value, ok := m.Load(key); !ok || value != want {
		t.Fatalf("Load(%v) = (%v, %v), want (%v, true)", key, value, ok, want)
	}
}

func assertMissing(t *testing.T, m *TTLSyncMap, key interface{}) {
	t.Helper()
	if value, ok := m.Load(key); ok || value != nil {
		t.Fatalf("Load(%v) = (%v, %v), want (nil, false)", key, value, ok)
	}
}

func TestStoreLoadDelete(t *testing.T) {
	m := testMap(t)
	assertMissing(t, m, "missing")
	m.Delete("missing")
	m.Store("key", 1)
	assertValue(t, m, "key", 1)
	m.Store("key", 2)
	assertValue(t, m, "key", 2)
	m.Delete("key")
	assertMissing(t, m, "key")
	m.Store(nil, nil)
	assertValue(t, m, nil, nil)
	if value, loaded := m.LoadOrStore(nil, 3); !loaded || value != nil {
		t.Fatalf("LoadOrStore(nil) = (%v, %v), want (nil, true)", value, loaded)
	}
	if value, loaded := m.LoadAndDelete(nil); !loaded || value != nil {
		t.Fatalf("LoadAndDelete(nil) = (%v, %v), want (nil, true)", value, loaded)
	}
}

func TestUncomparableValues(t *testing.T) {
	m := testMap(t)
	m.Store("slice", []int{1, 2})
	m.Store("slice", []int{3})
	if value, ok := m.Load("slice"); !ok || value.([]int)[0] != 3 {
		t.Fatalf("Load(slice) = (%v, %v)", value, ok)
	}
	m.Store("map", map[string]int{"a": 1})
	if value, loaded := m.LoadOrStore("map", []int{2}); !loaded || value.(map[string]int)["a"] != 1 {
		t.Fatalf("LoadOrStore(map) = (%v, %v)", value, loaded)
	}
	m.Delete("map")
}

func TestExpiration(t *testing.T) {
	for _, operation := range []string{"Load", "LoadOrStore", "LoadAndDelete", "Range"} {
		t.Run(operation, func(t *testing.T) {
			m := testMap(t)
			m.Store("key", "old")
			expire(t, m, "key")
			switch operation {
			case "Load":
				assertMissing(t, m, "key")
			case "LoadOrStore":
				if value, loaded := m.LoadOrStore("key", "new"); loaded || value != "new" {
					t.Fatalf("LoadOrStore = (%v, %v), want (new, false)", value, loaded)
				}
				assertValue(t, m, "key", "new")
			case "LoadAndDelete":
				if value, loaded := m.LoadAndDelete("key"); loaded || value != nil {
					t.Fatalf("LoadAndDelete = (%v, %v), want (nil, false)", value, loaded)
				}
			case "Range":
				m.Range(func(key, value interface{}) bool {
					t.Error("Range visited expired value")
					return true
				})
			}
			if operation != "LoadOrStore" {
				if _, ok := m.current().Load("key"); ok {
					t.Fatal("expired entry was not physically removed")
				}
			}
		})
	}
}

func TestRealTTL(t *testing.T) {
	m := New(20 * time.Millisecond)
	defer m.Close()
	m.Store("key", 1)
	deadline := time.Now().Add(time.Second)
	for {
		if _, ok := m.Load("key"); !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("TTL did not expire")
		}
		time.Sleep(time.Millisecond)
	}
	if _, ok := m.current().Load("key"); ok {
		t.Fatal("expired entry remains in map")
	}
}

func TestNonPositiveTTL(t *testing.T) {
	for _, ttl := range []time.Duration{0, -time.Second} {
		m := New(ttl)
		m.Store("key", 1)
		assertMissing(t, m, "key")
		m.Close()
	}
}

func TestTTLRefresh(t *testing.T) {
	m := testMap(t)
	m.Store("key", 1)
	e, old := expire(t, m, "key")
	m.Store("key", 2)
	updated := e.value.Load().(*ttlVal)
	if updated == old || time.Until(updated.expireAt) <= 0 {
		t.Fatal("Store did not reset TTL")
	}
	assertValue(t, m, "key", 2)
	if value, loaded := m.LoadOrStore("key", 3); !loaded || value != 2 {
		t.Fatalf("LoadOrStore = (%v, %v), want (2, true)", value, loaded)
	}
	if e.value.Load().(*ttlVal) != updated {
		t.Fatal("a read extended the TTL")
	}
}

func TestExpiredObservationPreservesReplacement(t *testing.T) {
	m := testMap(t)
	m.Store("key", "old")
	e, old := expire(t, m, "key")
	data := m.current()
	m.Store("key", "updated")
	removeExpired(data, "key", e, old)
	assertValue(t, m, "key", "updated")

	m.Delete("key")
	m.Store("key", "reinserted")
	removeExpired(data, "key", e, old)
	assertValue(t, m, "key", "reinserted")

	fresh, observed := expire(t, m, "key")
	m.Clear()
	m.Store("key", "after-clear")
	removeExpired(data, "key", fresh, observed)
	assertValue(t, m, "key", "after-clear")
}

func TestLoadAndDelete(t *testing.T) {
	m := testMap(t)
	if value, loaded := m.LoadAndDelete("key"); loaded || value != nil {
		t.Fatalf("LoadAndDelete missing = (%v, %v)", value, loaded)
	}
	m.Store("key", 1)
	if value, loaded := m.LoadAndDelete("key"); !loaded || value != 1 {
		t.Fatalf("LoadAndDelete = (%v, %v), want (1, true)", value, loaded)
	}
	assertMissing(t, m, "key")
}

func TestRange(t *testing.T) {
	m := testMap(t)
	for i := 0; i < 10; i++ {
		m.Store(i, i)
	}
	seen := make(map[int]bool)
	m.Range(func(key, value interface{}) bool {
		if key != value || seen[key.(int)] {
			t.Errorf("invalid or duplicate entry (%v, %v)", key, value)
		}
		seen[key.(int)] = true
		return true
	})
	if len(seen) != 10 {
		t.Fatalf("Range visited %d entries, want 10", len(seen))
	}
	calls := 0
	m.Range(func(key, value interface{}) bool { calls++; return false })
	if calls != 1 {
		t.Fatalf("stopped Range made %d calls, want 1", calls)
	}
	m.Range(func(key, value interface{}) bool {
		m.Load(key)
		m.Delete(key)
		m.Store(key, value)
		m.Clear()
		return true
	})
}

func TestClearDuringRange(t *testing.T) {
	m := testMap(t)
	m.Store("old", 1)
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		m.Range(func(key, value interface{}) bool {
			close(entered)
			<-release
			m.Store("callback", 3)
			return true
		})
	}()
	<-entered
	m.Clear()
	m.Store("new", 2)
	close(release)
	<-finished
	assertMissing(t, m, "old")
	assertValue(t, m, "new", 2)
	assertValue(t, m, "callback", 3)
	m.Clear()
	m.Clear()
	assertMissing(t, m, "new")
}

func TestBackgroundCleanup(t *testing.T) {
	m := testMap(t)
	m.SetTimerAge(time.Hour)
	m.Store("expired", 1)
	expire(t, m, "expired")
	m.Store("live", 2)
	// This must wake the already-running timer, rather than wait an hour.
	m.SetTimerAge(time.Millisecond)
	deadline := time.Now().Add(time.Second)
	for {
		if _, ok := m.current().Load("expired"); !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background cleanup did not remove the expired entry")
		}
		time.Sleep(time.Millisecond)
	}
	assertValue(t, m, "live", 2)
	for _, age := range []time.Duration{0, -time.Second} {
		m.SetTimerAge(age)
		if got := m.timerAge.Load().(time.Duration); got != defaultTimerAging {
			t.Fatalf("SetTimerAge(%v) = %v, want %v", age, got, defaultTimerAging)
		}
	}
}

func TestClose(t *testing.T) {
	m := testMap(t)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.SetTimerAge(time.Nanosecond)
			m.Close()
			m.SetTimerAge(time.Millisecond)
		}()
	}
	wg.Wait()
	select {
	case <-m.stopped:
	default:
		t.Fatal("Close returned before cleanup stopped")
	}
	m.SetTimerAge(time.Millisecond)
	m.Store("key", 1)
	assertValue(t, m, "key", 1)
	expire(t, m, "key")
	assertMissing(t, m, "key")
	m.Clear()
}

func TestZeroValue(t *testing.T) {
	var m TTLSyncMap
	defer m.Close()
	m.Store("key", 1)
	assertMissing(t, &m, "key")
	m.Clear()
	m.SetTimerAge(time.Millisecond)
	m.Close()
}

func TestEmbeddedZeroValue(t *testing.T) {
	// On 32-bit platforms the map can start at a four-byte offset. Its internal
	// atomics must work without requiring eight-byte alignment from the caller.
	var holder struct {
		prefix uint32
		cache  TTLSyncMap
	}
	m := &holder.cache
	defer m.Close()
	m.SetTimerAge(time.Millisecond)
	m.Store("key", 1)
	assertMissing(t, m, "key")
	m.Clear()
}

func TestConcurrentOperations(t *testing.T) {
	m := testMap(t)
	m.SetTimerAge(time.Millisecond)
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				key := i % 32
				switch (i + worker) % 8 {
				case 0:
					m.Store(key, i)
				case 1:
					m.Load(key)
				case 2:
					m.LoadOrStore(key, i)
				case 3:
					m.LoadAndDelete(key)
				case 4:
					m.Delete(key)
				case 5:
					m.Range(func(key, value interface{}) bool { return true })
				case 6:
					m.Clear()
				case 7:
					m.SetTimerAge(time.Duration(i%3+1) * time.Millisecond)
				}
			}
		}(worker)
	}
	wg.Wait()
	m.Clear()
	m.Store("final", 42)
	assertValue(t, m, "final", 42)
}

func TestConcurrentLoadAndDelete(t *testing.T) {
	m := testMap(t)
	m.Store("key", 42)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var loaded int32
	for i := 0; i < 128; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if value, ok := m.LoadAndDelete("key"); ok {
				if value != 42 {
					t.Errorf("LoadAndDelete returned %v, want 42", value)
				}
				atomic.AddInt32(&loaded, 1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if loaded != 1 {
		t.Fatalf("%d callers loaded the deleted value, want 1", loaded)
	}
}
