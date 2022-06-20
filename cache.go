package cache

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type CachedElement[ResultType any] struct {
	Value     ResultType
	Timestamp time.Time
}

type cacheGetterFunc[ParamsType, ResultType any] func(ParamsType) (ResultType, error)

type cacheQueueElementResult[ResultType any] struct {
	value ResultType
	error error
}

type cacheQueueElement[ResultType any] struct {
	wait   chan struct{}
	result *cacheQueueElementResult[ResultType]
}

// Cache implements a thread-safe cache where
// getting the real data is expensive.
//
// If two goroutines ask the same key and the
// data is not already in the cache, the getter
// getter sub will only be called once.
//
// All goroutines will waits this result.
type Cache[ParamsType, ResultType any] struct {
	cache      map[string]*CachedElement[ResultType]
	cacheMutex sync.RWMutex

	cacheQueue      map[string]*cacheQueueElement[ResultType]
	cacheQueueMutex sync.RWMutex

	getter cacheGetterFunc[ParamsType, ResultType]

	strategy CachingStrategy[ResultType]
}

func (c *Cache[ParamsType, ResultType]) cleanup(interval time.Duration) {

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
func NewCache[ParamsType, ResultType any](f cacheGetterFunc[ParamsType, ResultType], cs CachingStrategy[ResultType]) *Cache[ParamsType, ResultType] {

	if cs == nil {
		cs = NewDefaultCachingStrategy[ResultType](0, 0)
	}

	c := Cache[ParamsType, ResultType]{
		cache:      make(map[string]*CachedElement[ResultType]),
		cacheQueue: make(map[string]*cacheQueueElement[ResultType]),
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
func (c *Cache[ParamsType, ResultType]) Get(key string, data ParamsType) (ResultType, error) {

	// Keep track of previous cached version
	var old *CachedElement[ResultType]

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
	queue := &cacheQueueElement[ResultType]{wait: make(chan struct{})}
	c.cacheQueue[key] = queue
	c.cacheQueueMutex.Unlock()

	// Do Real Call which may be time consuming
	result, err := func(in ParamsType) (out ResultType, err error) {
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
		e = &CachedElement[ResultType]{Value: result, Timestamp: time.Now()}
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
		queue.result = &cacheQueueElementResult[ResultType]{result, err}
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

// Delete removes the data stored under a given key from the cache.
// Returns a bool indicating whether any data was actually deleted.
func (c *Cache[ParamsType, ResultType]) Delete(key string) bool {
	c.cacheMutex.Lock()
	_, ok := c.cache[key]
	if ok {
		delete(c.cache, key)
	}
	c.cacheMutex.Unlock()
	return ok
}

// Set forces the data stored under a given key.
// If that key is being fetched through the normal workflow concurrently,
// your data may get overwritten.
func (c *Cache[ParamsType, ResultType]) Set(key string, i ResultType) {
	c.cacheMutex.Lock()
	c.cache[key] = &CachedElement[ResultType]{Value: i, Timestamp: time.Now()}
	c.cacheMutex.Unlock()
}

// Len returns the length of the cache.
func (c *Cache[ParamsType, ResultType]) Len() int {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()

	return len(c.cache)
}
