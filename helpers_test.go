package cache

import (
	"errors"
	"testing"
	"time"
)

func TestTimeout(t *testing.T) {
	// Func which waits v and returns OK
	f := func(v interface{}) (interface{}, error) {
		time.Sleep(v.(time.Duration))
		return "OK", nil
	}

	// Returns err if call is more than 100ms
	f = WithTimeout(f, 100*time.Millisecond, errors.New("timed out"))

	// Call with 50ms, should return 'OK'
	if v, e := f(50 * time.Millisecond); v.(string) != "OK" || e != nil {
		t.Fatalf("Should have returned OK, nil; received: %v, %v", v, e)
	}

	// Call with 200ms, should return 'timed out'
	if v, e := f(200 * time.Millisecond); v != nil || e == nil || e.Error() != "timed out" {
		t.Fatalf("Should have returned nil, 'timed out'; received: %v, %v", v, e)
	}
}
