# ttl-syncmap
带有过期时间的sync.map的实现

```go
go get -u github.com/gdpu11/ttl-syncmap
```

##使用

```go
m := NewTTLSyncMap(5 * time.Second)

//存储
m.Store(1, 1)

//查询
d, ok := m.Load(1)

//删除
m.Delete(1)

//清空
m.Clear()
```
