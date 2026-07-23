package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Limiter provides per-IP token bucket rate limiting.
type Limiter struct {
	rate    int // requests per window
	window  time.Duration
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens    int
	lastReset time.Time
}

// New creates a rate limiter allowing `rate` requests per `window` per IP.
func New(rate int, window time.Duration) *Limiter {
	return &Limiter{
		rate:    rate,
		window:  window,
		buckets: make(map[string]*bucket),
	}
}

// Allow checks if a request from the given IP is allowed.
func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[ip]
	now := time.Now()

	if !ok || now.Sub(b.lastReset) >= l.window {
		l.buckets[ip] = &bucket{tokens: l.rate - 1, lastReset: now}
		return true
	}

	if b.tokens <= 0 {
		return false
	}

	b.tokens--
	return true
}

// Middleware returns HTTP middleware that rate-limits requests.
// Exempt paths (health, ready, metrics) bypass the limiter.
func (l *Limiter) Middleware() func(http.Handler) http.Handler {
	exempt := map[string]bool{
		"/health":  true,
		"/ready":   true,
		"/metrics": true,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if exempt[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			ip := extractIP(r)
			if !l.Allow(ip) {
				w.Header().Set("Retry-After", "60")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractIP gets the client IP from the request.
func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first (behind reverse proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (client IP)
		if i := len(xff); i > 0 {
			for j := 0; j < len(xff); j++ {
				if xff[j] == ',' {
					return xff[:j]
				}
			}
			return xff
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// MaxBodySize returns middleware that limits request body size.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && r.ContentLength > maxBytes {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				w.Write([]byte(`{"error":"request body too large"}`))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
