package cache

import (
	"time"
)

type CachingStrategy interface {
	CleanupTick() time.Duration
	IsExpired(*CachedElement) bool
	IsCleanable(*CachedElement) bool
	NewCachedElement(*CachedElement, interface{}, error) (*CachedElement, error)
	ShouldPropagateError(error) bool
}

type DefaultCachingStrategy struct {
	expiration time.Duration
	cleanup    time.Duration
}

func NewDefaultCachingStrategy(expiration time.Duration, cleanup time.Duration) *DefaultCachingStrategy {
	return &DefaultCachingStrategy{expiration: expiration, cleanup: cleanup}
}

func (cs *DefaultCachingStrategy) CleanupTick() time.Duration {
	return cs.cleanup
}

func (cs *DefaultCachingStrategy) IsExpired(e *CachedElement) bool {
	if cs.expiration != 0 {
		return time.Since(e.Timestamp) >= cs.expiration
	}
	return false
}

func (cs *DefaultCachingStrategy) IsCleanable(e *CachedElement) bool {
	return cs.IsExpired(e)
}

func (cs *DefaultCachingStrategy) NewCachedElement(old *CachedElement, v interface{}, e error) (*CachedElement, error) {
	return &CachedElement{Value: v, Timestamp: time.Now()}, e
}

func (cs *DefaultCachingStrategy) ShouldPropagateError(err error) bool {
	return true
}
