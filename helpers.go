package cache

import (
	"time"
)

type timeOutResult struct {
	r interface{}
	e error
}

func WithTimeout(f cacheGetterFunc, t time.Duration, err error) cacheGetterFunc {
	return func(v interface{}) (interface{}, error) {
		c := make(chan timeOutResult)
		go func() {
			r, e := f(v)
			c <- timeOutResult{r, e}
		}()
		select {
		case r := <-c:
			return r.r, r.e
		case <-time.After(t):
		}
		return nil, err
	}
}

func WithRetry(f cacheGetterFunc, n int) cacheGetterFunc {
	return func(v interface{}) (interface{}, error) {
		var lastError error
		for i := 0; i < n; i++ {
			r, e := f(v)
			if e == nil {
				return r, e
			}
			lastError = e
		}
		return nil, lastError
	}
}
