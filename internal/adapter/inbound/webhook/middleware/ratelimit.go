package middleware

import (
	"net/http"
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

// rateLimiter manages per-IP token buckets.
type rateLimiter struct {
	mu                 sync.Mutex
	buckets            map[string]*tokenBucket
	requestsPerMinute  int
}

func newRateLimiter(requestsPerMinute int) *rateLimiter {
	return &rateLimiter{
		buckets:           make(map[string]*tokenBucket),
		requestsPerMinute: requestsPerMinute,
	}
}

func (rl *rateLimiter) getBucket(ip string) *tokenBucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[ip]
	if !ok {
		bucket = newTokenBucket(rl.requestsPerMinute)
		rl.buckets[ip] = bucket
	}
	return bucket
}

// NewRateLimiter returns a middleware that limits requests per minute per remote IP
// using a token bucket algorithm (stdlib only, no external dependencies).
func NewRateLimiter(requestsPerMinute int) func(http.Handler) http.Handler {
	rl := newRateLimiter(requestsPerMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := remoteIP(r)
			bucket := rl.getBucket(ip)
			if !bucket.allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// remoteIP extracts the client IP from the request, checking X-Forwarded-For first.
func remoteIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
