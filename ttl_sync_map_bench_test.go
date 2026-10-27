package ttlsyncmap

import (
	"testing"
	"time"
)

func benchmarkMap(b *testing.B) *TTLSyncMap {
	b.Helper()
	m := New(time.Hour)
	b.Cleanup(func() {
		if closer, ok := interface{}(m).(interface{ Close() }); ok {
			closer.Close()
		}
	})
	return m
}

func BenchmarkLoadHit(b *testing.B) {
	m := benchmarkMap(b)
	m.Store("key", 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Load("key")
	}
}

func BenchmarkStoreExisting(b *testing.B) {
	m := benchmarkMap(b)
	m.Store("key", 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Store("key", 1)
	}
}

func BenchmarkLoadOrStoreHit(b *testing.B) {
	m := benchmarkMap(b)
	m.Store("key", 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.LoadOrStore("key", 2)
	}
}

func BenchmarkParallelLoad(b *testing.B) {
	m := benchmarkMap(b)
	m.Store("key", 1)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Load("key")
		}
	})
}

func BenchmarkParallelStore(b *testing.B) {
	m := benchmarkMap(b)
	for i := 0; i < 1024; i++ {
		m.Store(i, 1)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Store(i&1023, 1)
			i++
		}
	})
}

func BenchmarkClear(b *testing.B) {
	m := benchmarkMap(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Clear()
	}
}

func BenchmarkParallelHotStore(b *testing.B) {
	m := benchmarkMap(b)
	m.Store("key", 1)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Store("key", 1)
		}
	})
}

func BenchmarkParallelLoadOrStoreHit(b *testing.B) {
	m := benchmarkMap(b)
	m.Store("key", 1)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.LoadOrStore("key", 2)
		}
	})
}

func BenchmarkStoreDelete(b *testing.B) {
	m := benchmarkMap(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Store("key", 1)
		m.Delete("key")
	}
}
