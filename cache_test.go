package cache

import (
	"sync"
	"testing"
	"time"
)

func Test(t *testing.T) {

	f := func(s string) interface{} {
		time.Sleep(100 * time.Millisecond)
		return "Cached-" + s
	}

	cache := NewCache(f, 200*time.Millisecond, 500*time.Millisecond)

	var foo1, foo2, foo3, foo4, bar1, bar2, bar3, bar4 string
	var wg sync.WaitGroup

	// Concurrent get
	wg.Add(4)
	go func() { foo1 = cache.Get("foo").(string); wg.Done() }()
	go func() { bar1 = cache.Get("bar").(string); wg.Done() }()
	go func() { foo2 = cache.Get("foo").(string); wg.Done() }()
	go func() { bar2 = cache.Get("bar").(string); wg.Done() }()
	wg.Wait()

	// Sequential get from cache
	wg.Add(2)
	go func() { foo3 = cache.Get("foo").(string); wg.Done() }()
	go func() { bar3 = cache.Get("bar").(string); wg.Done() }()
	wg.Wait()

	// Wait cache to expire and retry
	time.Sleep(300 * time.Millisecond)
	wg.Add(1)
	go func() { foo4 = cache.Get("foo").(string); wg.Done() }()
	wg.Wait()

	// Wait cleanup and retry
	time.Sleep(1 * time.Second)
	wg.Add(1)
	go func() { bar4 = cache.Get("bar").(string); wg.Done() }()
	wg.Wait()

	if foo1 != "Cached-foo" || bar1 != "Cached-bar" || foo2 != "Cached-foo" || bar2 != "Cached-bar" || foo3 != "Cached-foo" || bar3 != "Cached-bar" || foo4 != "Cached-foo" || bar4 != "Cached-bar" {
		t.Fatalf("Invalid return, foo1 = %q, bar1 = %q, bar2 = %q, foo2 = %q, bar3 = %q, foo3 = %q", foo1, bar1, foo2, bar2, foo3, bar3)
	}

}
