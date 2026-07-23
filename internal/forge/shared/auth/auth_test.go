package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if len(token) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("token length = %d, want 64", len(token))
	}

	// Tokens should be unique
	token2, _ := GenerateToken()
	if token == token2 {
		t.Error("two generated tokens should not be equal")
	}
}

func TestMiddleware_RejectsNoToken(t *testing.T) {
	handler := Middleware(Config{Token: "secret123"})(okHandler())
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestMiddleware_RejectsWrongToken(t *testing.T) {
	handler := Middleware(Config{Token: "secret123"})(okHandler())
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestMiddleware_AcceptsValidToken(t *testing.T) {
	handler := Middleware(Config{Token: "secret123"})(okHandler())
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestMiddleware_ExemptHealth(t *testing.T) {
	handler := Middleware(Config{Token: "secret123"})(okHandler())

	for _, path := range []string{"/health", "/ready", "/metrics"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200 (exempt)", path, w.Code)
		}
	}
}

func TestMiddleware_DisabledPassesThrough(t *testing.T) {
	handler := Middleware(Config{Token: "secret123", Disabled: true})(okHandler())
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (disabled)", w.Code)
	}
}

func TestMiddleware_CaseInsensitiveBearer(t *testing.T) {
	handler := Middleware(Config{Token: "mytoken"})(okHandler())
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "BEARER mytoken")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (case-insensitive Bearer)", w.Code)
	}
}

func TestValidateWSToken(t *testing.T) {
	// No token configured — always valid
	req := httptest.NewRequest("GET", "/ws/chat", nil)
	if !ValidateWSToken(req, "") {
		t.Error("empty token should always pass")
	}

	// Valid token
	req = httptest.NewRequest("GET", "/ws/chat?token=abc123", nil)
	if !ValidateWSToken(req, "abc123") {
		t.Error("valid token should pass")
	}

	// Invalid token
	req = httptest.NewRequest("GET", "/ws/chat?token=wrong", nil)
	if ValidateWSToken(req, "abc123") {
		t.Error("wrong token should fail")
	}

	// Missing token
	req = httptest.NewRequest("GET", "/ws/chat", nil)
	if ValidateWSToken(req, "abc123") {
		t.Error("missing token should fail")
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
