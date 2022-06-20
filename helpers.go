package cache

import (
	"time"
)

type timeOutResult[ResultType any] struct {
	r ResultType
	e error
}

func WithTimeout[ParamsType, ResultType any](f cacheGetterFunc[ParamsType, ResultType], t time.Duration, err error) cacheGetterFunc[ParamsType, ResultType] {
	return func(v ParamsType) (ResultType, error) {
		c := make(chan timeOutResult[ResultType])
		go func() {
			r, e := f(v)
			c <- timeOutResult[ResultType]{r, e}
		}()
		select {
		case r := <-c:
			return r.r, r.e
		case <-time.After(t):
		}
		var zero ResultType
		return zero, err
	}
}

func WithRetry[ParamsType, ResultType any](f cacheGetterFunc[ParamsType, ResultType], n int) cacheGetterFunc[ParamsType, ResultType] {
	return func(v ParamsType) (ResultType, error) {
		var lastError error
		for i := 0; i < n; i++ {
			r, e := f(v)
			if e == nil {
				return r, e
			}
			lastError = e
		}
		var zero ResultType
		return zero, lastError
	}
}
