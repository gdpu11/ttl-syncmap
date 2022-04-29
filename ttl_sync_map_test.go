package util

import (
	"testing"
	"time"
)

func TestNewTTLSyncMap(t *testing.T) {
	m := NewTTLSyncMap(5 * time.Second)
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
	m := NewTTLSyncMap(5 * time.Second)
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
	m := NewTTLSyncMap(5 * time.Second)
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
	m := NewTTLSyncMap(5 * time.Second)
	m.Store(1, 1)
	if d, ok := m.Load(1); !ok || d != 1 {
		t.Fatal("val is expire", ok, d)
	}
	m.Delete(1)
	if d, ok := m.Load(1); ok || d == 1 {
		t.Fatal("val is expire", ok, d)
	}
}

func TestTTLSyncMap_Clear(t *testing.T) {
	m := NewTTLSyncMap(5 * time.Second)
	m.Store(1, 1)
	if d, ok := m.Load(1); !ok || d != 1 {
		t.Fatal("val is expire", ok, d)
	}
	m.Clear()
	if d, ok := m.Load(1); ok || d == 1 {
		t.Fatal("val is expire", ok, d)
	}
}