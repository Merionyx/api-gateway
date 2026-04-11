package idempotency

import (
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

func TestExecute_ConflictDifferentBody(t *testing.T) {
	t.Parallel()
	s := NewStore(time.Hour)
	key := "0123456789abcdef0123456789abcdef"
	h1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	h2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	_, err := s.Execute(key, h1, func() (*HTTPResult, error) {
		return &HTTPResult{StatusCode: 200, ContentType: "application/json", Body: []byte(`{}`)}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Execute(key, h2, func() (*HTTPResult, error) {
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
			_, err := s.Execute(key, h, func() (*HTTPResult, error) {
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
