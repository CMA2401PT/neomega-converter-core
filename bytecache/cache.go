// Package bytecache 按调用方估算的字节占用淘汰冷数据，供读取器与响应缓存共用。
package bytecache

import "container/list"

// cacheItem 保留实际计费大小，替换条目时归还原预算。
type cacheItem[K comparable, V any] struct {
	key   K
	value V
	size  int
}

// Cache 以字节限制最近使用的结果，由调用方同步访问。
type Cache[K comparable, V any] struct {
	// 缓存由调用方同步，来源 goroutine 和 View 各自管理其生命周期。
	limit, size int
	order       *list.List
	items       map[K]*list.Element
}

// New 为独立来源分配预算，避免不同会话共用淘汰状态。
func New[K comparable, V any](limit int) *Cache[K, V] {
	return &Cache[K, V]{
		limit: limit,
		order: list.New(),
		items: map[K]*list.Element{},
	}
}

// Get 将命中项移到队首，使经常读取的数据优先保留。
func (c *Cache[K, V]) Get(key K) (V, bool) {
	if e, ok := c.items[key]; ok {
		c.order.MoveToFront(e)
		return e.Value.(cacheItem[K, V]).value, true
	}
	var zero V
	return zero, false
}

// Put 计入条目开销后淘汰冷数据，超大单项直接交由调用方使用。
func (c *Cache[K, V]) Put(key K, value V, size int) {
	size += 128
	if old, ok := c.items[key]; ok {
		c.size -= old.Value.(cacheItem[K, V]).size
		c.order.Remove(old)
		delete(c.items, key)
	}
	if size > c.limit {
		return
	}
	for c.size+size > c.limit {
		e := c.order.Back()
		item := e.Value.(cacheItem[K, V])
		c.size -= item.size
		delete(c.items, item.key)
		c.order.Remove(e)
	}
	c.items[key] = c.order.PushFront(cacheItem[K, V]{
		key:   key,
		value: value,
		size:  size,
	})
	c.size += size
}
