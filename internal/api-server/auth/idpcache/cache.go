// Package idpcache implements an in-process IdP access token cache (ADR 0002,).
package idpcache

import (
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// DefaultOpaqueMaxTTL is used when IdP access has no expires_in / exp and is not a JWT (ADR 0002 §3.1).
	DefaultOpaqueMaxTTL = 2 * time.Minute
	clockSkew           = 30 * time.Second
)

type entry struct {
	token    []byte
	deadline time.Time
}

// Cache stores IdP access tokens keyed by interactive session_id. Not safe for concurrent use of the same key from multiple goroutines beyond the mutex guarantees of exported methods.
type Cache struct {
	mu  sync.RWMutex
	m   map[string]entry
	now func() time.Time

	// Optional hooks (set once at startup;). Must not log token material.
	onGet        func(hit bool)
	onPut        func()
	onInvalidate func()
}

// New returns an empty cache. If now is nil, time.Now is used.
func New(now func() time.Time) *Cache {
	if now == nil {
		now = time.Now
	}
	return &Cache{
		m:   make(map[string]entry),
		now: now,
	}
}

// SetMetricsHooks registers low-cardinality callbacks for Prometheus (optional). Call before serving traffic.
func (c *Cache) SetMetricsHooks(onGet func(hit bool), onPut, onInvalidate func()) {
	c.onGet = onGet
	c.onPut = onPut
	c.onInvalidate = onInvalidate
}

// Now is the cache clock (for tests).
func (c *Cache) Now() time.Time {
	return c.now()
}

// Get returns a cached token if present and not expired.
func (c *Cache) Get(sessionID string) (token string, ok bool) {
	if sessionID == "" {
		return "", false
	}
	now := c.now()
	c.mu.RLock()
	e, found := c.m[sessionID]
	c.mu.RUnlock()
	hit := found && now.Before(e.deadline)
	if c.onGet != nil {
		c.onGet(hit)
	}
	if !hit {
		return "", false
	}
	return string(e.token), true
}

// Put stores a copy of token until now+ttl. No-op for empty ids, empty token, or non-positive ttl.
func (c *Cache) Put(sessionID, token string, ttl time.Duration) {
	if sessionID == "" || strings.TrimSpace(token) == "" || ttl <= 0 {
		return
	}
	buf := []byte(token)
	deadline := c.now().Add(ttl)
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.m[sessionID]; ok {
		zeroBytes(old.token)
	}
	c.m[sessionID] = entry{token: buf, deadline: deadline}
	if c.onPut != nil {
		c.onPut()
	}
}

// Invalidate removes the session entry and zeroes the previous token buffer.
func (c *Cache) Invalidate(sessionID string) {
	if sessionID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.m[sessionID]; ok {
		zeroBytes(old.token)
		delete(c.m, sessionID)
		if c.onInvalidate != nil {
			c.onInvalidate()
		}
	}
}

// Close clears all entries (e.g. on process shutdown).
func (c *Cache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.m {
		zeroBytes(e.token)
		delete(c.m, k)
	}
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// EntryTTL returns cache TTL per ADR 0002 §3.1: min(remaining our API access, remaining IdP access) minus skew.
// opaqueMax caps remaining IdP time when only an opaque token is available (no expires_in, no JWT exp); if <= 0, DefaultOpaqueMaxTTL is used for that branch.
func EntryTTL(clock, ourAccessExpiry time.Time, idpAccess string, expiresInSec int, opaqueMax time.Duration) (time.Duration, bool) {
	remOur := ourAccessExpiry.Sub(clock) - clockSkew
	if remOur <= 0 {
		return 0, false
	}
	remIdp, idpOk := idpAccessRemaining(clock, strings.TrimSpace(idpAccess), expiresInSec)
	if !idpOk {
		cap := opaqueMax
		if cap <= 0 {
			cap = DefaultOpaqueMaxTTL
		}
		remIdp = cap - clockSkew
	}
	if remIdp <= 0 {
		return 0, false
	}
	ttl := remOur
	if remIdp < ttl {
		ttl = remIdp
	}
	if ttl <= 0 {
		return 0, false
	}
	return ttl, true
}

func idpAccessRemaining(clock time.Time, idpAccess string, expiresInSec int) (time.Duration, bool) {
	if expiresInSec > 0 {
		d := time.Duration(expiresInSec)*time.Second - clockSkew
		return d, d > 0
	}
	p := jwt.NewParser()
	tok, _, err := p.ParseUnverified(idpAccess, jwt.MapClaims{})
	if err != nil {
		return 0, false
	}
	mc, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return 0, false
	}
	exp, err := mc.GetExpirationTime()
	if err != nil || exp == nil {
		return 0, false
	}
	d := exp.Sub(clock) - clockSkew
	return d, d > 0
}
