package cache

import (
	"time"
)

type CachingStrategy[T any] interface {
	CleanupTick() time.Duration
	IsExpired(*CachedElement[T]) bool
	IsCleanable(*CachedElement[T]) bool
	NewCachedElement(*CachedElement[T], T, error) (*CachedElement[T], error)
	ShouldPropagateError(error) bool
}

type DefaultCachingStrategy[T any] struct {
	expiration time.Duration
	cleanup    time.Duration
}

func NewDefaultCachingStrategy[T any](expiration time.Duration, cleanup time.Duration) *DefaultCachingStrategy[T] {
	return &DefaultCachingStrategy[T]{expiration: expiration, cleanup: cleanup}
}

func (cs *DefaultCachingStrategy[T]) CleanupTick() time.Duration {
	return cs.cleanup
}

func (cs *DefaultCachingStrategy[T]) IsExpired(e *CachedElement[T]) bool {
	if cs.expiration != 0 {
		return time.Since(e.Timestamp) >= cs.expiration
	}
	return false
}

func (cs *DefaultCachingStrategy[T]) IsCleanable(e *CachedElement[T]) bool {
	return cs.IsExpired(e)
}

func (cs *DefaultCachingStrategy[T]) NewCachedElement(old *CachedElement[T], v T, e error) (*CachedElement[T], error) {
	return &CachedElement[T]{Value: v, Timestamp: time.Now()}, e
}

func (cs *DefaultCachingStrategy[T]) ShouldPropagateError(err error) bool {
	return true
}
