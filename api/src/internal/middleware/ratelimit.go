package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/acienciadalua/api/internal/utils"
)

// RateLimiter is an in-memory sliding-window rate limiter, equivalent to the
// @nestjs/throttler configuration (default 10 requests / 60s, with per-route
// overrides on the auth endpoints).
type RateLimiter struct {
	limit  int
	window time.Duration

	mu        sync.Mutex
	hits      map[string][]time.Time
	lastSweep time.Time
	now       func() time.Time
}

// NewRateLimiter creates a limiter. Each instance has its own buckets, which
// reproduces Nest's per-handler throttling keys.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
		hits:   make(map[string][]time.Time),
		now:    time.Now,
	}
}

// Middleware applies the limiter to a single route.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)

		allowed, remaining, reset := l.allow(key)

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

		if !allowed {
			retryAfter := reset - l.now().Unix()
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
			utils.WriteError(w, r, utils.TooManyRequests())
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) allow(key string) (bool, int, int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)

	l.sweep(now)

	timestamps := l.hits[key]
	valid := timestamps[:0]
	for _, timestamp := range timestamps {
		if timestamp.After(cutoff) {
			valid = append(valid, timestamp)
		}
	}

	if len(valid) >= l.limit {
		l.hits[key] = valid
		oldest := valid[0]
		reset := oldest.Add(l.window).Unix()
		return false, 0, reset
	}

	valid = append(valid, now)
	l.hits[key] = valid

	reset := now.Add(l.window).Unix()
	if len(valid) > 0 {
		reset = valid[0].Add(l.window).Unix()
	}

	return true, l.limit - len(valid), reset
}

// sweep drops empty/stale buckets when the map grows, keeping memory bounded.
func (l *RateLimiter) sweep(now time.Time) {
	if len(l.hits) < 10_000 || now.Sub(l.lastSweep) < l.window {
		return
	}

	cutoff := now.Add(-l.window)
	for key, timestamps := range l.hits {
		if len(timestamps) == 0 || !timestamps[len(timestamps)-1].After(cutoff) {
			delete(l.hits, key)
		}
	}
	l.lastSweep = now
}

func clientIP(r *http.Request) string {
	// chi's RealIP middleware (installed globally) rewrites RemoteAddr using
	// X-Forwarded-For / X-Real-IP when present.
	address := r.RemoteAddr
	if index := strings.LastIndex(address, ":"); index != -1 {
		return address[:index]
	}
	return address
}
