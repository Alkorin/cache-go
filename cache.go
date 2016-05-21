package cache

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type CachedElement struct {
	Value     interface{}
	Timestamp time.Time
}

type cacheGetterFunc func(interface{}) (interface{}, error)

type cacheQueueElementResult struct {
	value interface{}
	error error
}

type cacheQueueElement struct {
	wait   chan struct{}
	result *cacheQueueElementResult
}

// Cache implements a thread-safe cache where
// getting the real data is expensive.
//
// If two goroutines ask the same key and the
// data is not already in the cache, the getter
// getter sub will only be called once.
//
// All goroutines will waits this result.
type Cache struct {
	cache      map[string]*CachedElement
	cacheMutex sync.RWMutex

	cacheQueue      map[string]*cacheQueueElement
	cacheQueueMutex sync.RWMutex

	getter cacheGetterFunc

	strategy CachingStrategy
}

func (c *Cache) cleanup(interval time.Duration) {

	if interval == 0 {
		return
	}

	ticker := time.Tick(interval)

	for {
		select {
		case <-ticker:
			// Do cleanup
			c.cacheMutex.Lock()
			for k, v := range c.cache {
				if c.strategy.IsCleanable(v) {
					delete(c.cache, k)
				}
			}
			c.cacheMutex.Unlock()
		}
	}
}

// NewCache returns a new Cache with getter f.
// f will be called to fetch cache-missing data.
// If expiration interval is non null, data will
// be refreshed if too old.
func NewCache(f cacheGetterFunc, cs CachingStrategy) *Cache {

	if cs == nil {
		cs = NewDefaultCachingStrategy(0, 0)
	}

	c := Cache{
		cache:      make(map[string]*CachedElement),
		cacheQueue: make(map[string]*cacheQueueElement),
		getter:     f,
		strategy:   cs,
	}

	go c.cleanup(cs.CleanupTick())

	return &c
}

// Get retrieve a data from the cache which is associated
// to the key 'key'. If data is missing in cache, the
// getter will be called to obtain it and store it in
// the cache
func (c *Cache) Get(key string, data interface{}) (interface{}, error) {

	// Keep track of previous cached version
	var old *CachedElement

	// First try to see if result is already in cache
	c.cacheMutex.RLock()
	if v, ok := c.cache[key]; ok {
		if !c.strategy.IsExpired(v) {
			// Result found in cache, return it
			c.cacheMutex.RUnlock()
			return v.Value, nil
		}
		old = v
	}

	// Result was not found in cache, let see is someone
	// is already working to fetch this value.
	//
	// Workers are stored in cacheQueue, we write-lock
	// cacheQueue to avoid race condition where we think
	// nobody is working on it but someone waits to insert
	// its lock.
	c.cacheQueueMutex.Lock()
	c.cacheMutex.RUnlock()

	for {
		if queue, ok := c.cacheQueue[key]; ok {
			// Someone is already fetching this value, wait it's answer
			c.cacheQueueMutex.Unlock()
			<-queue.wait

			// If found return it, else retry
			if queue.result != nil {
				return queue.result.value, queue.result.error
			}

			c.cacheQueueMutex.Lock()
		} else {
			// Nobody is already fetching this value, let's go
			break
		}
	}

	// Nobody is fetching this key, so we will insert
	// wait lock and do the real job. The wait lock
	// is made by a simple chan, as a read in a chan
	// is a blocking operation, unblocked when the chan
	// is closed.
	queue := &cacheQueueElement{wait: make(chan struct{})}
	c.cacheQueue[key] = queue
	c.cacheQueueMutex.Unlock()

	// Do Real Call which may be time consuming
	result, err := func(in interface{}) (out interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				errRecover, ok := r.(error)
				if !ok {
					errRecover = errors.New(fmt.Sprint(r))
				}
				err = errRecover
			}
		}()

		return c.getter(in)
	}(data)

	e, err := c.strategy.NewCachedElement(old, result, err)
	// Protect against faulty strategy components
	if e == nil {
		e = &CachedElement{Value: result, Timestamp: time.Now()}
	}
	result = e.Value

	// Store result if callee said it's ok
	if err == nil {
		c.cacheMutex.Lock()
		c.cache[key] = e
		c.cacheMutex.Unlock()
	}

	// Propagate result
	if err == nil || c.strategy.ShouldPropagateError(err) {
		queue.result = &cacheQueueElementResult{result, err}
	}

	// Clean cacheQueue
	c.cacheQueueMutex.Lock()
	delete(c.cacheQueue, key)
	c.cacheQueueMutex.Unlock()

	// Unlock waiters by closing the chan
	close(queue.wait)

	// Return result
	return result, err
}
