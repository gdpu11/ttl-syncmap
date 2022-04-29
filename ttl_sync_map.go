package util

import (
	"sync"
	"time"
)

//TTLSyncMap 带有时效性的缓存
type TTLSyncMap struct {
	ttl  time.Duration
	data sync.Map
}

type cacheExpireMap struct {
	val      interface{}
	expireAt time.Time
}

//NewTTLSyncMap 谨慎使用，仅适用于较少元素且又高频的数据
func NewTTLSyncMap(ttl time.Duration) *TTLSyncMap {
	return &TTLSyncMap{ttl, sync.Map{}}
}

//Load ...
func (c *TTLSyncMap) Load(key interface{}) (data interface{}, ok bool) {
	if d, ok := c.data.Load(key); ok && d != nil {
		if m, okMap := d.(cacheExpireMap); okMap {
			if time.Since(m.expireAt) <= c.ttl {
				return m.val, true
			}
			c.data.Delete(key)
			return nil, false
		}
	}
	return nil, false
}

//Store ...
func (c *TTLSyncMap) Store(key interface{}, val interface{}) {
	c.data.Store(key, cacheExpireMap{val: val, expireAt: time.Now()})
}

//Delete 删除key
func (c *TTLSyncMap) Delete(key interface{}) {
	c.data.Delete(key)
}

//Clear 清空
func (c *TTLSyncMap) Clear() {
	c.data = sync.Map{}
}
