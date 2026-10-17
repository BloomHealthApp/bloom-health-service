// Package ratelimit provides per-key token-bucket limiters.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Keyed keeps one token bucket per key and evicts idle keys.
type Keyed struct {
	mu   sync.Mutex
	m    map[string]*entry
	r    rate.Limit
	b    int
	last time.Time
}

type entry struct {
	l    *rate.Limiter
	seen time.Time
}

// NewKeyed creates a limiter allowing r events per second with burst b per key.
func NewKeyed(r rate.Limit, b int) *Keyed { return &Keyed{m: map[string]*entry{}, r: r, b: b} }

// Allow consumes one token for key.
func (ls *Keyed) Allow(key string) bool {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	now := time.Now()
	if now.Sub(ls.last) > 5*time.Minute {
		for k, e := range ls.m {
			if now.Sub(e.seen) > 30*time.Minute {
				delete(ls.m, k)
			}
		}
		ls.last = now
	}
	e, ok := ls.m[key]
	if !ok {
		e = &entry{l: rate.NewLimiter(ls.r, ls.b)}
		ls.m[key] = e
	}
	e.seen = now
	return e.l.Allow()
}

// Set groups the limiters used by the HTTP layer.
type Set struct {
	IP    *Keyed // every request, per client IP
	Auth  *Keyed // /v1/auth/*, per client IP
	Email *Keyed // OTP request/verify, per email
}

// NewSet builds the production limits.
func NewSet() *Set {
	return &Set{
		IP:    NewKeyed(rate.Limit(30), 90),
		Auth:  NewKeyed(rate.Every(6*time.Second), 10),
		Email: NewKeyed(rate.Every(2*time.Minute), 3),
	}
}
