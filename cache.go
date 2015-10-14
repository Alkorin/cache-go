package cache

import "sync"

type cachedElement struct {
	value interface{}
}

type cacheGetterFunc func(string) interface{}

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
}

// NewCache returns a new Cache with getter f.
// f will be called to fetch cache-missing data.
func NewCache(f cacheGetterFunc) *Cache {
	return &Cache{
		cache:      make(map[string]*cachedElement),
		cacheQueue: make(map[string]chan bool),
		getter:     f,
	}
}

// Get retrieve a data from the cache which is associated
// to the key 'key'. If data is missing in cache, the
// getter will be called to obtain it and store it in
// the cache
func (c *Cache) Get(key string) interface{} {

	// First try to see if result is already in cache
	c.cacheMutex.RLock()
	if v, ok := c.cache[key]; ok {
		// Result found in cache, return it
		c.cacheMutex.RUnlock()
		return v.value
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
		return v.value
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
	result := c.getter(key)

	// Store result
	c.cacheMutex.Lock()
	c.cache[key] = &cachedElement{value: result}
	c.cacheMutex.Unlock()

	// Clean cacheQueue
	c.cacheQueueMutex.Lock()
	delete(c.cacheQueue, key)
	c.cacheQueueMutex.Unlock()

	// Unlock waiters by closing the chan
	close(wait)

	// Return result
	return result
}
