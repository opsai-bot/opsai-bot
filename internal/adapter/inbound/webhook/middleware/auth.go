package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// BearerAuth returns middleware that validates a Bearer token in the Authorization header.
func BearerAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}

			token := strings.TrimSpace(parts[1])
			if !hmac.Equal([]byte(token), []byte(secret)) {
				http.Error(w, "invalid bearer token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// HMACAuth returns middleware that validates an HMAC-SHA256 signature.
// The signature is expected in the X-Hub-Signature-256 header as "sha256=<hex>".
func HMACAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sigHeader := r.Header.Get("X-Hub-Signature-256")
			if sigHeader == "" {
				http.Error(w, "missing signature header", http.StatusUnauthorized)
				return
			}

			const prefix = "sha256="
			if !strings.HasPrefix(sigHeader, prefix) {
				http.Error(w, "invalid signature format", http.StatusUnauthorized)
				return
			}

			providedSig, err := hex.DecodeString(strings.TrimPrefix(sigHeader, prefix))
			if err != nil {
				http.Error(w, "invalid signature encoding", http.StatusUnauthorized)
				return
			}

			body, ok := r.Context().Value(rawBodyKey{}).([]byte)
			if !ok {
				http.Error(w, "request body not available for signature verification", http.StatusInternalServerError)
				return
			}

			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expectedSig := mac.Sum(nil)

			if !hmac.Equal(expectedSig, providedSig) {
				http.Error(w, "invalid HMAC signature", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// rawBodyKey is used to store the raw request body in context (set by BodyReader middleware).
type rawBodyKey struct{}
