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

func TestRetry(t *testing.T) {
	nbCalled := 0
	// Func which returns OK only at the second call
	f := func(v interface{}) (interface{}, error) {
		nbCalled++
		if nbCalled == 2 {
			return "OK", nil
		}
		return nil, errors.New("not second")
	}

	// Try to call func 3 times
	f = WithRetry(f, 3)

	// First try, should call the function 2 times and return success
	if v, e := f(nil); nbCalled != 2 || v.(string) != "OK" || e != nil {
		t.Fatalf("Should have returned 2, OK, nil; received: %v, %v, %v", nbCalled, v, e)
	}

	// Second try, should call the function 3 times and return error
	if v, e := f(nil); nbCalled != 5 || v != nil || e == nil || e.Error() != "not second" {
		t.Fatalf("Should have returned 5, nil, 'not second'; received: %v, %v, %v", nbCalled, v, e)
	}
}
