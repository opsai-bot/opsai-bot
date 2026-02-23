package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// tokenBucket implements a simple token bucket rate limiter per remote IP.
type tokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per nanosecond
	lastRefill time.Time
}

func newTokenBucket(requestsPerMinute int) *tokenBucket {
	max := float64(requestsPerMinute)
	return &tokenBucket{
		tokens:     max,
		maxTokens:  max,
		refillRate: max / float64(time.Minute),
		lastRefill: time.Now(),
	}
}

func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	tb.tokens += float64(elapsed) * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now

	if tb.tokens < 1 {
		return false
	}
	tb.tokens--
	return true
}

// rateLimiter manages per-IP token buckets with automatic eviction of stale entries.
type rateLimiter struct {
	mu                sync.Mutex
	buckets           map[string]*tokenBucket
	requestsPerMinute int
	maxBuckets        int
	trustProxy        bool
}

func newRateLimiter(requestsPerMinute int, trustProxy bool) *rateLimiter {
	rl := &rateLimiter{
		buckets:           make(map[string]*tokenBucket),
		requestsPerMinute: requestsPerMinute,
		maxBuckets:        10000,
		trustProxy:        trustProxy,
	}
	go rl.evictionLoop()
	return rl
}

// evictionLoop periodically removes stale buckets to prevent unbounded memory growth.
func (rl *rateLimiter) evictionLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.evictStale(10 * time.Minute)
	}
}

// evictStale removes buckets not accessed within maxAge.
func (rl *rateLimiter) evictStale(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for ip, bucket := range rl.buckets {
		bucket.mu.Lock()
		stale := bucket.lastRefill.Before(cutoff)
		bucket.mu.Unlock()
		if stale {
			delete(rl.buckets, ip)
		}
	}
}

func (rl *rateLimiter) getBucket(ip string) *tokenBucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// If we've hit maxBuckets, reject new IPs to prevent memory exhaustion.
	bucket, ok := rl.buckets[ip]
	if !ok {
		if len(rl.buckets) >= rl.maxBuckets {
			return nil
		}
		bucket = newTokenBucket(rl.requestsPerMinute)
		rl.buckets[ip] = bucket
	}
	return bucket
}

// NewRateLimiter returns a middleware that limits requests per minute per remote IP
// using a token bucket algorithm (stdlib only, no external dependencies).
// trustProxy controls whether X-Forwarded-For is used for IP extraction.
func NewRateLimiter(requestsPerMinute int) func(http.Handler) http.Handler {
	rl := newRateLimiter(requestsPerMinute, false)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := remoteIP(r, rl.trustProxy)
			bucket := rl.getBucket(ip)
			if bucket == nil || !bucket.allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// remoteIP extracts the client IP from the request.
// Only trusts X-Forwarded-For when trustProxy is true (i.e., behind a known reverse proxy).
func remoteIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the first (client) IP in the list.
			if idx := strings.IndexByte(xff, ','); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
	}
	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
