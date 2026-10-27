// Package ttlsyncmap provides a concurrent map with a fixed TTL per write.
package ttlsyncmap

import (
	"sync"
	"sync/atomic"
	"time"
)

const defaultTimerAging = 10 * time.Second

// TTLSyncMap is a concurrent map whose entries expire after a fixed TTL.
// It must not be copied after first use. The zero value has an immediate TTL;
// use New to configure a positive TTL. Call Close when the map is no longer used.
type TTLSyncMap struct {
	timerAge atomic.Value // time.Duration; safe when embedded on 32-bit platforms.
	ttl      time.Duration
	once     sync.Once
	data     atomic.Value // *sync.Map; Clear publishes a new generation.
	done     chan struct{}
	stopped  chan struct{}
	reset    chan struct{}
	close    sync.Once
}

// An entry stays associated with its key until it is marked deleted and removed
// under mu. Writers that encounter a deleted entry retry against the map.
// Values are immutable snapshots, so live reads need no entry lock.
type entry struct {
	mu    sync.Mutex
	value atomic.Value // *ttlVal; a typed nil means deleted.
}

type ttlVal struct {
	val      interface{}
	expireAt time.Time
}

// New returns a map with the given TTL. A non-positive TTL expires immediately.
// Each successful write starts a new TTL; reads do not extend it.
func New(ttl time.Duration) *TTLSyncMap {
	c := &TTLSyncMap{ttl: ttl}
	c.init()
	return c
}

func (c *TTLSyncMap) init() {
	c.once.Do(func() {
		c.data.Store(&sync.Map{})
		c.timerAge.Store(defaultTimerAging)
		c.done = make(chan struct{})
		c.stopped = make(chan struct{})
		c.reset = make(chan struct{}, 1)
		go c.aging()
	})
}

func (c *TTLSyncMap) current() *sync.Map {
	c.init()
	return c.data.Load().(*sync.Map)
}

// Load returns the value for key if it has not expired.
func (c *TTLSyncMap) Load(key interface{}) (value interface{}, ok bool) {
	data := c.current()
	if item, found := data.Load(key); found {
		e := item.(*entry)
		v := e.value.Load().(*ttlVal)
		if v != nil {
			if time.Until(v.expireAt) > 0 {
				return v.val, true
			}
			removeExpired(data, key, e, v)
		}
	}
	return nil, false
}

// Store stores value for key and resets its TTL.
func (c *TTLSyncMap) Store(key, value interface{}) {
	c.store(key, value, false)
}

// LoadOrStore atomically returns the existing, unexpired value if present.
// Otherwise it stores and returns value. loaded is true only for an existing value.
func (c *TTLSyncMap) LoadOrStore(key, value interface{}) (actual interface{}, loaded bool) {
	return c.store(key, value, true)
}

func (c *TTLSyncMap) store(key, value interface{}, loadOnly bool) (interface{}, bool) {
	data := c.current()
	for {
		item, found := data.Load(key)
		if !found {
			e := &entry{}
			e.value.Store(&ttlVal{val: value, expireAt: time.Now().Add(c.ttl)})
			item, found = data.LoadOrStore(key, e)
			if !found {
				return value, false
			}
		}
		e := item.(*entry)
		if loadOnly {
			if v := e.value.Load().(*ttlVal); v != nil && time.Until(v.expireAt) > 0 {
				return v.val, true
			}
		}
		e.mu.Lock()
		v := e.value.Load().(*ttlVal)
		if v == nil {
			e.mu.Unlock()
			continue
		}
		if loadOnly && time.Until(v.expireAt) > 0 {
			e.mu.Unlock()
			return v.val, true
		}
		e.value.Store(&ttlVal{val: value, expireAt: time.Now().Add(c.ttl)})
		e.mu.Unlock()
		return value, false
	}
}

// Delete removes key.
func (c *TTLSyncMap) Delete(key interface{}) {
	c.LoadAndDelete(key)
}

// LoadAndDelete atomically deletes key and returns its previous value if unexpired.
func (c *TTLSyncMap) LoadAndDelete(key interface{}) (value interface{}, loaded bool) {
	data := c.current()
	if item, found := data.Load(key); found {
		e := item.(*entry)
		e.mu.Lock()
		v := e.value.Load().(*ttlVal)
		live := v != nil && time.Until(v.expireAt) > 0
		if v != nil {
			e.value.Store((*ttlVal)(nil))
			data.Delete(key)
		}
		e.mu.Unlock()
		if live {
			return v.val, true
		}
	}
	return nil, false
}

// Range calls f sequentially for each unexpired key and value until f returns
// false. No key is visited more than once. Range is not a consistent snapshot:
// it may observe any value stored during the call. f may call any map method.
// A Range overlapping Clear may finish visiting the previous generation.
func (c *TTLSyncMap) Range(f func(key, value interface{}) bool) {
	data := c.current()
	data.Range(func(key, item interface{}) bool {
		e := item.(*entry)
		v := e.value.Load().(*ttlVal)
		if v == nil {
			return true
		}
		if time.Until(v.expireAt) > 0 {
			return f(key, v.val)
		}
		removeExpired(data, key, e, v)
		return true
	})
}

// removeExpired only removes the observed value. A concurrent Store must not
// lose its replacement to an expired reader or a background sweep.
func removeExpired(data *sync.Map, key interface{}, e *entry, observed *ttlVal) {
	e.mu.Lock()
	if e.value.Load().(*ttlVal) == observed {
		e.value.Store((*ttlVal)(nil))
		data.Delete(key)
	}
	e.mu.Unlock()
}

// Clear atomically replaces the map with an empty generation, without waiting
// for readers. Operations overlapping Clear may finish on the old generation.
func (c *TTLSyncMap) Clear() {
	c.init()
	c.data.Store(&sync.Map{})
}

// SetTimerAge sets the background cleanup interval and reschedules the next
// sweep. Non-positive intervals restore the default of ten seconds.
// Expiration on reads is independent of this interval.
func (c *TTLSyncMap) SetTimerAge(age time.Duration) {
	c.init()
	if age <= 0 {
		age = defaultTimerAging
	}
	c.timerAge.Store(age)
	select {
	case c.reset <- struct{}{}:
	default:
	}
}

// Close stops background cleanup and waits for it to exit. It is idempotent.
// Map operations still work after Close, including lazy expiration on reads.
func (c *TTLSyncMap) Close() {
	c.init()
	c.close.Do(func() { close(c.done) })
	<-c.stopped
}

func (c *TTLSyncMap) aging() {
	defer close(c.stopped)
	timer := time.NewTimer(c.timerAge.Load().(time.Duration))
	defer timer.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-c.reset:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
			c.Range(func(key, value interface{}) bool { return true })
		}
		timer.Reset(c.timerAge.Load().(time.Duration))
	}
}
