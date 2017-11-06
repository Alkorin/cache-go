package cache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func Test(t *testing.T) {

	nbHit := int64(0)

	f := func(v interface{}) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt64(&nbHit, int64(1))
		return v, nil
	}

	cache := NewCache(f, NewDefaultCachingStrategy(200*time.Millisecond, 500*time.Millisecond))

	var foo1, foo2, foo3, foo4, foo5, bar1, bar2 interface{}
	var wg sync.WaitGroup

	// Sequential get before expiration
	wg.Add(2)
	go func() { foo1, _ = cache.Get("foo", "Cached-foo"); wg.Done() }()
	go func() { bar1, _ = cache.Get("bar", "Cached-bar"); wg.Done() }()
	wg.Wait()
	wg.Add(2)
	go func() { foo2, _ = cache.Get("foo", "Cached-foo2"); wg.Done() }()
	go func() { bar2, _ = cache.Get("bar", "Cached-bar2"); wg.Done() }()
	wg.Wait()

	if foo2 != foo1 || bar2 != bar1 {
		t.Errorf("foo2/bar2 got new values, cache was not used: foo1=%q, foo2=%q, bar1=%q, bar2=%q", foo1, foo2, bar1, bar2)
	}

	if nbHit != 2 { // foo1, bar1
		t.Errorf("Should have 2 hits on cache getter func after foo1/bar1, got %d", nbHit)
	}
	cacheLen := cache.Len()
	if cacheLen != 2 {
		t.Errorf("expected cache length to be 2, got %d", cacheLen)
	}

	time.Sleep(1 * time.Second) // Sleep to force expiration
	// Concurrent get (cache queue)
	wg.Add(2)
	go func() { foo3, _ = cache.Get("foo", "Cached-foo3"); wg.Done() }()
	time.Sleep(10 * time.Millisecond) // Sleep to force goroutine scheduling order
	go func() { foo4, _ = cache.Get("foo", "Cached-foo4"); wg.Done() }()
	wg.Wait()

	if foo4 != foo3 {
		t.Errorf("foo4 got new value, cacheQueue error: foo3=%q, foo4=%q", foo3, foo4)
	}

	if nbHit != 3 { // foo1, bar1, foo3
		t.Errorf("Should have 3 hits on cache getter func after foo3, got %d", nbHit)
	}

	// Delete key
	if ok := cache.Delete("foo"); !ok {
		t.Errorf("foo key should have been deleted, got %t", ok)
	}
	wg.Add(1)
	go func() { foo5, _ = cache.Get("foo", "Cached-foo5"); wg.Done() }()
	wg.Wait()

	if foo5 != "Cached-foo5" {
		t.Errorf("foo5 got an invalid value: foo5=%q", foo5)
	}

	if nbHit != 4 { // foo1, bar1, foo3, foo5
		t.Errorf("Should have 4 hits on cache getter func after foo5, got %d", nbHit)
	}
	time.Sleep(1 * time.Second)

	cacheLen = cache.Len()
	if cacheLen != 0 {
		t.Errorf("expected cache length to be 0, got %d", cacheLen)
	}
}

func TestError(t *testing.T) {

	errStr := "test recover"

	var nbHit int64

	f := func(v interface{}) (interface{}, error) {
		if atomic.LoadInt64(&nbHit) == 0 {
			atomic.AddInt64(&nbHit, 1)
			panic(errStr)
		}
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt64(&nbHit, 1)
		return v, nil
	}

	cache := NewCache(f, nil)

	var foo1, foo2, foo3 interface{}
	var err error
	var wg sync.WaitGroup

	// 3 sequential gets
	// first should error
	// 2nd & 3rd should be queued and auto-retry
	wg.Add(3)
	go func() { foo1, err = cache.Get("foo", "Cached-foo1"); wg.Done() }()
	time.Sleep(10 * time.Millisecond)
	go func() { foo2, _ = cache.Get("foo", "Cached-foo2"); wg.Done() }()
	time.Sleep(10 * time.Millisecond)
	go func() { foo3, _ = cache.Get("foo", "Cached-foo3"); wg.Done() }()
	wg.Wait()

	if err == nil {
		t.Errorf("foo1 should have produced an error, got nil, foo1=%v", foo1)
	} else if err.Error() != errStr {
		t.Errorf("foo1 error message is not consistent, expected '%s' got '%s'", errStr, err.Error())
	}

	if nbHit != 2 {
		t.Errorf("Should have 2 hits on cache getter func after foo3, got %d", nbHit)
	}

	if foo2 != foo3 {
		t.Errorf("foo3 got an invalid value: foo2=%q, foo3=%q", foo2, foo3)
	}
}

func TestSet(t *testing.T) {

	key := "foo"
	normalFlow := "normal"
	forceSet := "forceset"

	f := func(v interface{}) (interface{}, error) {
		return normalFlow, nil
	}

	cache := NewCache(f, nil)

	c1, err := cache.Get(key, key)
	if err != nil {
		t.Error(err)
	}
	if c1 != normalFlow {
		t.Errorf("Expected '%s', got '%s'", normalFlow, c1)
	}

	cache.Set(key, forceSet)

	c2, err := cache.Get(key, key)
	if err != nil {
		t.Error(err)
	}
	if c2 != forceSet {
		t.Errorf("Expected '%s', got '%s'", forceSet, c2)
	}

	cacheLen := cache.Len()
	if cacheLen != 1 {
		t.Errorf("expected cache length to be 1, got %d", cacheLen)
	}
}
