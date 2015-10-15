package cache

import (
	"sync"
	"time"
)

type cachedElement struct {
	value      interface{}
	error      error
	expiration *time.Time
}

func (e *cachedElement) Expired() bool {
	if e.expiration == nil || e.expiration.After(time.Now()) {
		return false
	}
	return true
}

type cacheGetterFunc func(interface{}) (interface{}, error)

// Cache implements a thread-safe cache where
// getting the real data is expensive.
//
// If two goroutines ask the same key and the
// data is not already in the cache, the getter
// getter sub will only be called once.
//
// All goroutines will waits this result.
type Cache struct {
	cache      map[string]*cachedElement
	cacheMutex sync.RWMutex

	cacheQueue      map[string]chan bool
	cacheQueueMutex sync.RWMutex

	getter cacheGetterFunc

	expiration time.Duration
}

func (c *Cache) cleanup(interval time.Duration) {

	ticker := time.Tick(interval)

	for {
		select {
		case <-ticker:
			// Do cleanup
			c.cacheMutex.Lock()
			for k, v := range c.cache {
				if v.Expired() {
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
func NewCache(f cacheGetterFunc, expiration time.Duration, cleanup time.Duration) *Cache {

	c := Cache{
		cache:      make(map[string]*cachedElement),
		cacheQueue: make(map[string]chan bool),
		getter:     f,
		expiration: expiration,
	}

	if cleanup != 0 {
		go c.cleanup(cleanup)
	}

	return &c
}

// Get retrieve a data from the cache which is associated
// to the key 'key'. If data is missing in cache, the
// getter will be called to obtain it and store it in
// the cache
func (c *Cache) Get(key string, data interface{}) (interface{}, error) {

	// First try to see if result is already in cache
	c.cacheMutex.RLock()
	if v, ok := c.cache[key]; ok && v.error == nil && !v.Expired() {
		// Result found in cache, return it
		c.cacheMutex.RUnlock()
		return v.value, nil
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
	if wait, ok := c.cacheQueue[key]; ok {
		// Someone is already fetching this value, wait it's answer
		c.cacheQueueMutex.Unlock()
		<-wait

		// Result should be in cache
		c.cacheMutex.RLock()
		v := c.cache[key]
		c.cacheMutex.RUnlock()
		return v.value, v.error
	}

	// Nobody is fetching this key, so we will insert
	// wait lock and do the real job. The wait lock
	// is made by a simple chan, as a read in a chan
	// is a blocking operation, unblocked when the chan
	// is closed.
	wait := make(chan bool)
	c.cacheQueue[key] = wait
	c.cacheQueueMutex.Unlock()

	// Do Real Call which may be time consuming
	result, err := c.getter(data)

	// Store result if callee said it's ok
	c.cacheMutex.Lock()
	e := cachedElement{value: result, error: err}
	if c.expiration != 0 {
		expi := time.Now().Add(c.expiration)
		e.expiration = &expi
	}
	c.cache[key] = &e
	c.cacheMutex.Unlock()

	// Clean cacheQueue
	c.cacheQueueMutex.Lock()
	delete(c.cacheQueue, key)
	c.cacheQueueMutex.Unlock()

	// Unlock waiters by closing the chan
	close(wait)

	// Return result
	return result, err
}
