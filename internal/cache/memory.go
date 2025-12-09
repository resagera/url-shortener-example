package cache

import (
	"container/list"
	"sync"
)

type entry struct {
	key   string
	value string
}

type URLCache struct {
	mu    sync.Mutex
	ll    *list.List
	cache map[string]*list.Element
	max   int
}

func NewURLCache(max int) *URLCache {
	if max <= 0 {
		max = 1
	}
	return &URLCache{
		ll:    list.New(),
		cache: make(map[string]*list.Element, max),
		max:   max,
	}
}

func (c *URLCache) Get(code string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ele, ok := c.cache[code]; ok {
		c.ll.MoveToFront(ele)
		ent := ele.Value.(*entry)
		return ent.value, true
	}
	return "", false
}

func (c *URLCache) Set(code, url string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ele, ok := c.cache[code]; ok {
		c.ll.MoveToFront(ele)
		ent := ele.Value.(*entry)
		ent.value = url
		return
	}

	ele := c.ll.PushFront(&entry{key: code, value: url})
	c.cache[code] = ele

	if c.ll.Len() > c.max {
		c.removeOldest()
	}
}

func (c *URLCache) removeOldest() {
	ele := c.ll.Back()
	if ele == nil {
		return
	}
	c.ll.Remove(ele)
	ent := ele.Value.(*entry)
	delete(c.cache, ent.key)
}
