package middleware

import (
	"log"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// NewLoggingMiddleware returns middleware that logs each incoming request with
// method, path, status code, and elapsed duration.
func NewLoggingMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := newResponseWriter(w)

			next.ServeHTTP(wrapped, r)

			elapsed := time.Since(start)
			logger.Printf(
				"method=%s path=%s status=%d duration=%s remote=%s",
				r.Method,
				r.URL.Path,
				wrapped.statusCode,
				elapsed.Round(time.Millisecond),
				remoteIP(r, false),
			)
		})
	}
}
