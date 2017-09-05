package annonce

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

// key is a comparable type
type key interface{}

type getFunc func(context.Context, key) (item, error)

type item struct {
	exp time.Time
	v   interface{}
}
type ttlCache struct {
	m  map[key]item
	h  heap.Interface
	f  getFunc
	mu sync.Mutex
}

func newCache(f getFunc) *ttlCache {
	return &ttlCache{
		m: make(map[key]item),
		h: new(timeKeyHeap),
		f: f,
	}
}

func (c *ttlCache) get(ctx context.Context, k key) (item, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clean()
	if i, ok := c.m[k]; ok {
		return i, nil
	}
	c.mu.Unlock()
	i, err := c.f(ctx, k)
	c.mu.Lock()
	if err != nil {
		return item{}, err
	}
	c.add(k, i)
	return i, nil
}

func (c *ttlCache) clean() {
	var x timeKey
	now := time.Now()
	for c.h.Len() > 0 {
		x = heap.Pop(c.h).(timeKey)
		if now.After(x.exp) {
			delete(c.m, x)
			continue
		}
		heap.Push(c.h, x)
		return
	}
}

func (c *ttlCache) add(k key, i item) {
	c.m[k] = i
	heap.Push(c.h, timeKey{k: k, exp: i.exp})
}

type timeKey struct {
	k   key
	exp time.Time
}

type timeKeyHeap []timeKey

func (h *timeKeyHeap) Len() int {
	return len(*h)
}

func (h *timeKeyHeap) Less(i, j int) bool {
	return (*h)[i].exp.Before((*h)[j].exp)
}

func (h *timeKeyHeap) Swap(i, j int) {
	(*h)[i], (*h)[j] = (*h)[j], (*h)[i]
}

func (h *timeKeyHeap) Push(x interface{}) {
	*h = append(*h, x.(timeKey))
}

func (h *timeKeyHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
