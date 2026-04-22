package idempotency

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

func TestHashBundleSyncRequest_Stable(t *testing.T) {
	t.Parallel()
	a := apiserver.BundleSyncRequest{Repository: "r", Ref: "main", Bundle: "b"}
	b := apiserver.BundleSyncRequest{Repository: "r", Ref: "main", Bundle: "b"}
	if HashBundleSyncRequest(a) != HashBundleSyncRequest(b) {
		t.Fatal("hash mismatch for equal payloads")
	}
}

func TestExecute_bypassesWhenKeyOrHashEmpty(t *testing.T) {
	t.Parallel()
	s := NewStore(time.Hour)
	var runs int
	res1, err := s.Execute(context.Background(), "", "ab", func() (*HTTPResult, error) {
		runs++
		return &HTTPResult{StatusCode: 201}, nil
	})
	if err != nil || res1.StatusCode != 201 || runs != 1 {
		t.Fatalf("empty key: err=%v res=%v runs=%d", err, res1, runs)
	}
	var runs2 int
	_, _ = s.Execute(context.Background(), "akey", "", func() (*HTTPResult, error) {
		runs2++
		return &HTTPResult{StatusCode: 202}, nil
	})
	if runs2 != 1 {
		t.Fatalf("empty body hash: runs=%d", runs2)
	}
}

func TestExecute_ConflictDifferentBody(t *testing.T) {
	t.Parallel()
	s := NewStore(time.Hour)
	key := "0123456789abcdef0123456789abcdef"
	h1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	h2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	_, err := s.Execute(context.Background(), key, h1, func() (*HTTPResult, error) {
		return &HTTPResult{StatusCode: 200, ContentType: "application/json", Body: []byte(`{}`)}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Execute(context.Background(), key, h2, func() (*HTTPResult, error) {
		return nil, errors.New("second body must not run")
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestExecute_ConcurrentDedup(t *testing.T) {
	s := NewStore(time.Hour)
	key := "fedcba9876543210fedcba9876543210"
	h := HashBundleSyncRequest(apiserver.BundleSyncRequest{Repository: "r", Ref: "m", Bundle: "x"})

	var run int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Execute(context.Background(), key, h, func() (*HTTPResult, error) {
				atomic.AddInt32(&run, 1)
				time.Sleep(30 * time.Millisecond)
				return &HTTPResult{StatusCode: 200, ContentType: "application/json", Body: []byte(`{}`)}, nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if atomic.LoadInt32(&run) != 1 {
		t.Fatalf("want fn once, got %d", run)
	}
}

func TestExecute_fnErrorReplayed(t *testing.T) {
	t.Parallel()
	s := NewStore(time.Hour)
	key := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	h := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	fnErr := errors.New("pipeline failed")
	_, err := s.Execute(context.Background(), key, h, func() (*HTTPResult, error) {
		return nil, fnErr
	})
	if err == nil || err.Error() != fnErr.Error() {
		t.Fatalf("first: %v", err)
	}
	var secondRan bool
	_, err2 := s.Execute(context.Background(), key, h, func() (*HTTPResult, error) {
		secondRan = true
		return &HTTPResult{StatusCode: 200}, nil
	})
	if secondRan {
		t.Fatal("second call must not run fn")
	}
	if err2 == nil || err2.Error() != fnErr.Error() {
		t.Fatalf("second: %v", err2)
	}
}

func TestExecute_ttlEvictionRunsFnAgain(t *testing.T) {
	s := NewStore(8 * time.Millisecond)
	key := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	h := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	var runs int32
	for i := 0; i < 3; i++ {
		if i > 0 {
			time.Sleep(10 * time.Millisecond)
		}
		_, err := s.Execute(context.Background(), key, h, func() (*HTTPResult, error) {
			atomic.AddInt32(&runs, 1)
			return &HTTPResult{StatusCode: 200, ContentType: "application/json", Body: []byte(`{}`)}, nil
		})
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	if atomic.LoadInt32(&runs) != 3 {
		t.Fatalf("want 3 fn runs after eviction, got %d", runs)
	}
}
