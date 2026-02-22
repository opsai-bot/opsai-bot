package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

// BodyReader reads and buffers the request body so it can be accessed multiple
// times (e.g. for HMAC validation and then JSON parsing). The raw bytes are
// stored in the request context under rawBodyKey{}.
func BodyReader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10 MB limit
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		// Restore body so downstream handlers can read it again
		r.Body = io.NopCloser(bytes.NewReader(body))

		ctx := context.WithValue(r.Context(), rawBodyKey{}, body)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
