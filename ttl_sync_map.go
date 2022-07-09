package ttlSyncMap

import (
	"sync"
	"time"
)

//TTLSyncMap 带有时效性的SyncMap
type TTLSyncMap struct {
	//有效时长
	ttl  time.Duration
	data sync.Map
}

type ttlVal struct {
	val interface{}
	// 过期时间
	expireAt time.Time
}

//New 谨慎使用，仅适用于较少元素且又高频的数据
func New(ttl time.Duration) *TTLSyncMap {
	return &TTLSyncMap{ttl, sync.Map{}}
}

//Load 查询
func (c *TTLSyncMap) Load(key interface{}) (data interface{}, ok bool) {
	if d, ok := c.data.Load(key); ok && d != nil {
		m := d.(ttlVal)
		if time.Since(m.expireAt) <= c.ttl {
			return m.val, true
		}
		c.data.Delete(key)
	}

	return nil, false
}

//Store 存储
func (c *TTLSyncMap) Store(key interface{}, val interface{}) {
	c.data.Store(key, ttlVal{val: val, expireAt: time.Now()})
}

//Delete 删除key
func (c *TTLSyncMap) Delete(key interface{}) {
	c.data.Delete(key)
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
//LoadOrStore返回键的现有值（如果存在）。
//否则，它将存储并返回给定的值。
//如果加载了值，则加载的结果为true；如果存储了值，则加载的结果为false。
func (c *TTLSyncMap) LoadOrStore(key, value interface{}) (actual interface{}, loaded bool) {
	d, ok := c.data.Load(key)
	if ok && d != nil && time.Since(d.(ttlVal).expireAt) <= c.ttl {
		return d.(ttlVal).val, true
	}
	c.data.Store(key, ttlVal{val: value, expireAt: time.Now()})
	return value, false
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
//LoadAndDelete删除键的值，如果有，则返回上一个值。
//加载的结果报告密钥是否存在。
func (c *TTLSyncMap) LoadAndDelete(key interface{}) (value interface{}, loaded bool) {
	d, ok := c.data.LoadAndDelete(key)
	if ok && d != nil && time.Since(d.(ttlVal).expireAt) <= c.ttl {
		return d.(ttlVal).val, true
	}
	return nil, false
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Map's
// contents: no key will be visited more than once, but if the value for any key
// is stored or deleted concurrently, Range may reflect any mapping for that key
// from any point during the Range call.
//
// Range may be O(N) with the number of elements in the map even if f returns
// false after a constant number of calls.
func (c *TTLSyncMap) Range(f func(key, value interface{}) bool) {
	ff := func(key, d interface{}) bool {
		if d != nil {
			m := d.(ttlVal)
			if time.Since(m.expireAt) <= c.ttl {
				return f(key, d.(ttlVal).val)
			}
			c.data.Delete(key)
		}
		return true
	}
	c.data.Range(ff)
}

//Clear 清空
func (c *TTLSyncMap) Clear() {
	c.data = sync.Map{}
}
