package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

// Config holds authentication settings.
type Config struct {
	Token    string // bearer token (from flag, env, or auto-generated)
	Disabled bool   // true when --no-auth is set
}

// GenerateToken creates a cryptographically random 32-byte hex token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Middleware returns HTTP middleware that validates bearer tokens.
// Exempt paths (health, ready, metrics) bypass authentication.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	exempt := map[string]bool{
		"/health":  true,
		"/ready":   true,
		"/metrics": true,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.Disabled {
				next.ServeHTTP(w, r)
				return
			}

			// Exempt endpoints (k8s probes, prometheus)
			if exempt[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			token := extractBearerToken(r)
			if token == "" {
				writeUnauthorized(w, "missing Authorization header")
				return
			}

			if !secureCompare(token, cfg.Token) {
				writeUnauthorized(w, "invalid token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ValidateWSToken checks the token query parameter for WebSocket upgrade requests.
func ValidateWSToken(r *http.Request, token string) bool {
	if token == "" {
		return true // no auth configured
	}
	qToken := r.URL.Query().Get("token")
	return secureCompare(qToken, token)
}

// extractBearerToken pulls the token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

// secureCompare performs constant-time string comparison to prevent timing attacks.
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
