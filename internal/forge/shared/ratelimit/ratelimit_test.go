package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLimiterAllow(t *testing.T) {
	l := New(3, time.Minute)

	for i := range 3 {
		if !l.Allow("1.2.3.4") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	if l.Allow("1.2.3.4") {
		t.Error("4th request should be blocked")
	}

	// Different IP should still be allowed
	if !l.Allow("5.6.7.8") {
		t.Error("different IP should be allowed")
	}
}

func TestLimiterMiddleware(t *testing.T) {
	l := New(2, time.Minute)
	handler := l.Middleware()(okHandler())

	// First two requests pass
	for range 2 {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	}

	// Third request blocked
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header")
	}
}

func TestLimiterExemptPaths(t *testing.T) {
	l := New(1, time.Minute)
	handler := l.Middleware()(okHandler())

	// Exhaust the limit
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Exempt paths should still work
	for _, path := range []string{"/health", "/ready", "/metrics"} {
		req := httptest.NewRequest("GET", path, nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", path, w.Code)
		}
	}
}

func TestMaxBodySize(t *testing.T) {
	handler := MaxBodySize(10)(okHandler())

	// Small body passes
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader("small"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("small body: expected 200, got %d", w.Code)
	}

	// Large body with Content-Length header gets rejected early
	req = httptest.NewRequest("POST", "/api/test", strings.NewReader(strings.Repeat("x", 100)))
	req.ContentLength = 100
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("large body: expected 413, got %d", w.Code)
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
