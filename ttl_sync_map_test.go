package ttlsyncmap

import (
	"sync"
	"testing"
	"time"
)

//go test -bench=. -run=^a -benchmem -count=3 -memprofile=mem.pb.gz
//go test -cover -coverprofile=cover.pb.gz
//go tool pprof -http=:6565 mem.pb.gz
//go tool cover -html=coverage.out
//PASS
//coverage: 100.0% of statements
//ok      github.com/gdpu11/ttl-syncmap   21.338s
//go tool cover -html=cover.pb.gz

func TestConcurrentClearAndStore(t *testing.T) {
	m := New(2 * time.Minute)
	go func() {
		for i := 0; i < 100; i++ {
			time.Sleep(time.Millisecond * 10)
			m.Clear()
		}
	}()
	for i := 0; i < 1000000; i++ {
		m.Store(i, i)
	}
}

func TestNewTTLSyncMap(t *testing.T) {
	m := New(5 * time.Second)
	m.Store(1, 1)
	time.Sleep(5 * time.Second)
	if d, ok := m.Load(1); ok || d == 1 {
		t.Fatal("val is expire", ok, d)
	}
	m.Store(1, 1)
	time.Sleep(1 * time.Second)
	if d, ok := m.Load(1); !ok || d != 1 {
		t.Fatal("val is expire", ok, d)
	}
}

func TestTTLSyncMap_Store(t *testing.T) {
	m := New(5 * time.Second)
	m.Store(1, 1)
	if d, ok := m.Load(1); !ok || d != 1 {
		t.Fatal("val is expire", ok, d)
	}
	m.Store(1, 2)
	if d, ok := m.Load(1); !ok || d != 2 {
		t.Fatal("val is expire", ok, d)
	}
	m.Store(1, 3)
	m.Store(1, 4)
	if d, ok := m.Load(1); !ok || d != 4 {
		t.Fatal("val is expire", ok, d)
	}
}

func TestTTLSyncMap_Load(t *testing.T) {
	m := New(5 * time.Second)
	m.Store(1, 1)
	if d, ok := m.Load(1); !ok || d != 1 {
		t.Fatal("val is expire", ok, d)
	}
	m.Store(1, 2)
	if d, ok := m.Load(1); !ok || d != 2 {
		t.Fatal("val is expire", ok, d)
	}
	m.Store(1, 3)
	m.Store(1, 4)
	if d, ok := m.Load(1); !ok || d != 4 {
		t.Fatal("val is expire", ok, d)
	}
}

func TestTTLSyncMap_Delete(t *testing.T) {
	m := New(5 * time.Second)
	m.Store(1, 1)
	if d, ok := m.Load(1); !ok || d != 1 {
		t.Fatal("val is expire", ok, d)
	}
	m.Delete(1)
	if d, ok := m.Load(1); ok || d == 1 {
		t.Fatal("val is expire", ok, d)
	}
	i := 0
	m.Range(func(key, value interface{}) bool {
		i++
		if value == nil || !(key == 1 && value == 1) {
			t.Fatal("value not expire", key, value)
			return false
		}
		return true
	})
	if i > 0 {
		t.Fatal(i)
	}
}

func TestTTLSyncMap_LoadOrStore(t *testing.T) {
	m := New(5 * time.Second)
	d, ok := m.LoadOrStore(1, 1)
	if ok || d != 1 {
		t.Fatal("LoadOrStore error", ok, d)
	}
	d, ok = m.LoadOrStore(1, 1)
	if !ok || d != 1 {
		t.Fatal("LoadOrStore error", ok, d)
	}
	time.Sleep(5 * time.Second)
	d, ok = m.LoadOrStore(1, 1)
	if ok || d != 1 {
		t.Fatal("LoadOrStore error", ok, d)
	}
}

func TestTTLSyncMap_LoadAndDelete(t *testing.T) {
	m := New(5 * time.Second)
	d, ok := m.LoadAndDelete(1)
	if ok || d != nil {
		t.Fatal("LoadAndDelete error", ok, d)
	}
	m.Store(1, 1)
	d, ok = m.LoadAndDelete(1)
	if !ok || d != 1 {
		t.Fatal("LoadOrStore error", ok, d)
	}
	m.Store(1, 1)
	//wait to expire and delete
	time.Sleep(5 * time.Second)
	d, ok = m.LoadAndDelete(1)
	if ok || d != nil {
		t.Fatal("LoadOrStore error", ok, d)
	}
}

func TestTTLSyncMap_Range(t *testing.T) {
	m := New(5 * time.Second)
	m.Store(1, 1)
	m.Range(func(key, value interface{}) bool {
		if key == 1 && value != 1 {
			t.Fatal("key and value not equal", key, value)
			return false
		}
		return key == 1 && value == 1
	})
	time.Sleep(5 * time.Second)
	m.Range(func(key, value interface{}) bool {
		if value == nil || !(key == 1 && value == 1) {
			t.Fatal("value not expire", key, value)
			return false
		}
		return true
	})
}

func TestTTLSyncMap_Clear(t *testing.T) {
	m := New(5 * time.Second)
	m.Store(1, 1)
	if d, ok := m.Load(1); !ok || d != 1 {
		t.Fatal("val is expire", ok, d)
	}
	m.Clear()
	if d, ok := m.Load(1); ok || d == 1 {
		t.Fatal("val is expire", ok, d)
	}
	i := 0
	m.Range(func(key, value interface{}) bool {
		i++
		if value == nil || !(key == 1 && value == 1) {
			t.Fatal("value not expire", key, value)
			return false
		}
		return true
	})
	if i > 0 {
		t.Fatal(i)
	}
}

var (
	ttlM = New(time.Minute)
	m    = sync.Map{}
)

// go test -bench=. -run=^a -benchmem -count=3 -memprofile=mem.pb.gz
func BenchmarkTTLSyncMap_Store(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			ttlM.Store(j, j)
		}
	}
}

func BenchmarkSyncMap_Store(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			m.Store(j, j)
		}
	}
}
