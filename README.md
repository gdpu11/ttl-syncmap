# ttl-syncmap
带有过期时间的sync.map的实现

```go
go get -u github.com/gdpu11/ttl-syncmap
```

## 使用

```go
m := NewTTLSyncMap(5 * time.Second)

//存储
m.Store(1, 1)

//查询
d, ok := m.Load(1)

//查询,存在返回OK=true，不存在则store且返回false
d, ok := m.LoadOrStore(1,1)

//查询,存在则删除且返回OK=true，不存在则返回false
d, ok := m.LoadOrDelete(1)

//遍历 bool=true时继续
m.Range(func(key, value interface{}) bool {
    //do somethis
    return true
})

//删除
m.Delete(1)

//清空
m.Clear()
```
