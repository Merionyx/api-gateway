package idpcache

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestEntryTTL_minOurAndExpiresIn(t *testing.T) {
	t.Parallel()
	clock := time.Unix(1000, 0)
	ourExp := clock.Add(5 * time.Minute)
	ttl, ok := EntryTTL(clock, ourExp, "opaque", 120, 0)
	if !ok {
		t.Fatal("expected ok")
	}
	// min(5m-skew, 120s-skew) ≈ 90s
	wantMax := 120*time.Second - clockSkew
	if ttl > wantMax+time.Millisecond || ttl < wantMax-time.Second {
		t.Fatalf("ttl %s want ~%s", ttl, wantMax)
	}
}

func TestEntryTTL_respectsOurAccessShorter(t *testing.T) {
	t.Parallel()
	clock := time.Unix(2000, 0)
	ourExp := clock.Add(90 * time.Second)
	ttl, ok := EntryTTL(clock, ourExp, "x", 3600, 0)
	if !ok {
		t.Fatal("expected ok")
	}
	want := 90*time.Second - clockSkew
	if ttl > want+time.Second || ttl < want-time.Second {
		t.Fatalf("ttl %s want ~%s", ttl, want)
	}
}

func TestEntryTTL_jwtExpWithoutExpiresIn(t *testing.T) {
	t.Parallel()
	clock := time.Unix(3000, 0)
	ourExp := clock.Add(10 * time.Minute)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": clock.Add(90 * time.Second).Unix(),
	})
	s, err := tok.SignedString([]byte("unit-test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	ttl, ok := EntryTTL(clock, ourExp, s, 0, 0)
	if !ok {
		t.Fatal("expected ok")
	}
	if ttl > 90*time.Second {
		t.Fatalf("ttl %s", ttl)
	}
}

func TestEntryTTL_opaqueUsesCap(t *testing.T) {
	t.Parallel()
	clock := time.Unix(4000, 0)
	ourExp := clock.Add(30 * time.Minute)
	ttl, ok := EntryTTL(clock, ourExp, "not-a-jwt", 0, 3*time.Minute)
	if !ok {
		t.Fatal("expected ok")
	}
	want := 3*time.Minute - clockSkew
	if ttl > want+time.Millisecond || ttl < want-time.Second {
		t.Fatalf("ttl %s want ~%s", ttl, want)
	}
}

func TestEntryTTL_rejectExpiredOur(t *testing.T) {
	t.Parallel()
	clock := time.Unix(5000, 0)
	ourExp := clock.Add(-time.Second)
	_, ok := EntryTTL(clock, ourExp, "tok", 3600, time.Minute)
	if ok {
		t.Fatal("expected !ok")
	}
}

func TestCache_putGetInvalidate(t *testing.T) {
	t.Parallel()
	start := time.Unix(10_000, 0)
	c := New(func() time.Time { return start })
	const sid = "sess-1"
	c.Put(sid, "secret-at", 5*time.Minute)
	got, ok := c.Get(sid)
	if !ok || got != "secret-at" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	c.Invalidate(sid)
	_, ok = c.Get(sid)
	if ok {
		t.Fatal("expected miss after invalidate")
	}
}

func TestCache_expiry(t *testing.T) {
	t.Parallel()
	cur := []time.Time{time.Unix(20_000, 0)}
	c := New(func() time.Time { return cur[0] })
	const sid = "sess-2"
	c.Put(sid, "tok", time.Minute)
	if _, ok := c.Get(sid); !ok {
		t.Fatal("expected hit")
	}
	cur[0] = cur[0].Add(2 * time.Minute)
	if _, ok := c.Get(sid); ok {
		t.Fatal("expected miss after expiry")
	}
}

func TestCache_closeClears(t *testing.T) {
	t.Parallel()
	c := New(nil)
	c.Put("a", "x", time.Hour)
	c.Close()
	if _, ok := c.Get("a"); ok {
		t.Fatal("expected miss after close")
	}
}

func TestCache_metricsHooks(t *testing.T) {
	t.Parallel()
	var hits, misses, puts, invs int
	c := New(nil)
	c.SetMetricsHooks(
		func(hit bool) {
			if hit {
				hits++
			} else {
				misses++
			}
		},
		func() { puts++ },
		func() { invs++ },
	)
	if _, ok := c.Get("nope"); ok {
		t.Fatal("expected miss")
	}
	if misses != 1 || hits != 0 {
		t.Fatalf("lookup miss: hits=%d misses=%d", hits, misses)
	}
	c.Put("s", "t", time.Minute)
	if puts != 1 {
		t.Fatalf("puts %d", puts)
	}
	if _, ok := c.Get("s"); !ok {
		t.Fatal("expected hit")
	}
	if hits != 1 {
		t.Fatalf("hits %d", hits)
	}
	c.Invalidate("s")
	if invs != 1 {
		t.Fatalf("invs %d", invs)
	}
}
