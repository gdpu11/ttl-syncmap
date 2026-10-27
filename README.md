# ttl-syncmap

基于 `sync.Map` 的并发 TTL 缓存，无第三方依赖，支持 Go 1.17 及以上版本。

```sh
go get github.com/gdpu11/ttl-syncmap
```

## 使用

```go
package main

import (
    "fmt"
    "time"

    ttlsyncmap "github.com/gdpu11/ttl-syncmap"
)

func main() {
    m := ttlsyncmap.New(5 * time.Second)
    defer m.Close() // 不再使用时，停止后台清理协程

    m.Store("key", 1)
    value, ok := m.Load("key")
    fmt.Println(value, ok) // 1 true

    actual, loaded := m.LoadOrStore("key", 2)
    fmt.Println(actual, loaded) // 1 true

    m.Range(func(key, value interface{}) bool {
        fmt.Println(key, value)
        return true // false 停止遍历
    })

    value, loaded = m.LoadAndDelete("key")
    fmt.Println(value, loaded) // 1 true

    m.Delete("other-key")
    m.Clear()
    m.SetTimerAge(time.Second) // 调整后台清理间隔，立即重新计时
}
```

## 语义与并发保证

- TTL 从每次写入开始计算。`Store` 覆盖值并重置 TTL；`Load`、`Range` 和命中时的 `LoadOrStore` 不续期。
- 过期值视为不存在。`LoadOrStore` 对不存在或已过期的 key 原子写入；同一 key 的竞争调用中，只有写入者返回 `loaded=false`（除非期间再次过期或被删除）。
- `LoadAndDelete` 原子删除并返回未过期的旧值；值已过期或不存在时返回 `(nil, false)`。支持存储 `nil` 值，此时应通过布尔返回值区分是否存在。
- `Clear` 原子切换到空 map，不等待旧读写完成。与它重叠的操作可能在旧 map 上结束；返回后新开始的操作使用新 map。
- `Range` 不提供一致快照，每个 key 最多访问一次，跳过过期值。回调可重入调用其他方法；与 `Clear` 重叠的遍历可能继续访问旧 map。
- 默认每 10 秒清理过期数据。读取和遍历也会清理遇到的过期项；清理间隔不影响 TTL 的有效性。`SetTimerAge` 的参数小于等于零时恢复默认间隔。
- `Close` 可重复、并发调用，返回时后台协程已退出。之后仍可读写，过期项由读取和遍历清理，后台清理不会重启。每个不再使用的实例都应调用 `Close`。
- TTL 小于等于零时，写入的值立即过期。零值 `TTLSyncMap` 也采用此行为；建议使用 `New` 显式设置 TTL。
- key 的约束与 Go map 相同，必须可比较；value 可以是任意类型。map 及返回的 value 指向的业务对象是不同的并发边界：业务对象自身的修改需要调用方同步。首次使用后不要复制 `TTLSyncMap`。

## 验证

```sh
go test ./...
go test -race -count=10 ./...
go vet ./...
go test -run '^$' -bench . -benchmem
```

测试包含 TTL、原子写入/删除、过期清理与覆盖写的竞争、并发清空、重入遍历、清理间隔更新和协程关闭。基准覆盖命中读取、覆盖写入、并发读写及清空。

## 性能取舍

为兼容 Go 1.17，每个 key 使用独立的同步状态，命中读取无需获取该锁，覆盖写入只分配新的值快照。`Clear` 通过原子切换 map 完成，旧 map 在使用它的操作结束后由 GC 回收。

这个实现优先兼顾缓存常见的命中读取和覆盖写入。频繁新增后立即删除 key 的场景会增加分配和同步开销；请使用基准和实际业务负载评估。清空后，仍在执行的旧操作可能暂时保留旧 map 的内存。
