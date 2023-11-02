package ttlsyncmap

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	//clearDataDelayTime 清除数据时，延迟多久清空，避免切换下标的时候还有用户在读，会有panic
	clearDataDelayTime = 200 * time.Millisecond
	defaultTimerAging  = 10 * time.Second
)

// TTLSyncMap 并发安全，带有时效性的缓存
type TTLSyncMap struct {
	data     [2]sync.Map   // 数据段 ps:unsafe.Sizeof=80
	ttl      time.Duration // 有效时间，比如1个小时：time.Hour ps:unsafe.Sizeof=8
	timerAge time.Duration // 数据淘汰定时器:time.Hour ps:unsafe.Sizeof=8
	clearing int32         // 并发清除数据时，只允许一个穿透进去清理 ps:unsafe.Sizeof=4
	idx      int8          // 当前使用的下标 ps:unsafe.Sizeof=1
}

type ttlVal struct {
	val interface{}
	// 过期时间
	expireAt time.Time
}

// New New
func New(ttl time.Duration) *TTLSyncMap {
	t := &TTLSyncMap{
		data:     [2]sync.Map{{}, {}},
		clearing: 0,
		ttl:      ttl,
		idx:      0,
	}
	t.aging()
	return t
}

// Load 查询
func (c *TTLSyncMap) Load(key interface{}) (data interface{}, ok bool) {
	if d, ok := c.data[c.idx].Load(key); ok && d != nil {
		m := d.(ttlVal)
		if time.Since(m.expireAt) <= c.ttl {
			return m.val, true
		}
		c.data[c.idx].Delete(key)
	}

	return nil, false
}

// Store 存储
func (c *TTLSyncMap) Store(key interface{}, val interface{}) {
	c.data[c.idx].Store(key, ttlVal{val: val, expireAt: time.Now()})
}

// Delete 删除key
func (c *TTLSyncMap) Delete(key interface{}) {
	c.data[c.idx].Delete(key)
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
// LoadOrStore返回键的现有值（如果存在）。
// 否则，它将存储并返回给定的值。
// 如果加载了值，则加载的结果为true；如果存储了值，则加载的结果为false。
func (c *TTLSyncMap) LoadOrStore(key, value interface{}) (actual interface{}, loaded bool) {
	d, ok := c.data[c.idx].Load(key)
	if ok && d != nil && time.Since(d.(ttlVal).expireAt) <= c.ttl {
		return d.(ttlVal).val, true
	}
	c.data[c.idx].Store(key, ttlVal{val: value, expireAt: time.Now()})
	return value, false
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
// LoadAndDelete删除键的值，如果有，则返回上一个值。
// 加载的结果报告密钥是否存在。
func (c *TTLSyncMap) LoadAndDelete(key interface{}) (value interface{}, loaded bool) {
	d, ok := c.data[c.idx].LoadAndDelete(key)
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
			c.Delete(key)
		}
		return true
	}
	c.data[c.idx].Range(ff)
}

// Clear 清空数据
func (c *TTLSyncMap) Clear() {
	//防止并发清除
	if !atomic.CompareAndSwapInt32(&c.clearing, 0, 1) {
		//sleep1毫秒保证idx已经切换
		time.Sleep(1 * time.Millisecond)
		return
	}
	//避免读到旧数据，先马上切下标
	oldIdx := c.idx
	c.idx ^= 1

	defer atomic.StoreInt32(&c.clearing, 0)

	//避免切换后仍有用户在读旧数据，所以这里延迟一下再清空数据，
	time.Sleep(clearDataDelayTime)
	c.data[oldIdx] = sync.Map{}
}

// SetTimerAge 设置淘汰定时器的时间，比如1小时，则每小时扫一下数据进行淘汰
func (c *TTLSyncMap) SetTimerAge(age time.Duration) {
	c.timerAge = age
}

// aging 数据老化
func (c *TTLSyncMap) aging() {
	for {
		if c.timerAge != 0 {
			time.Sleep(c.timerAge)
		} else {
			time.Sleep(defaultTimerAging)
		}
		c.Range(func(key, value interface{}) bool {
			return true
		})
	}
}
