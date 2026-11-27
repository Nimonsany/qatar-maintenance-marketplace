package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
)

// requestIDContextKey prevents collisions with context
// keys defined in other packages.
type requestIDContextKey struct{}

type requestIDGenerator func() (string, error)

// generateRequestID creates 128 bits of cryptographically
// secure randomness, represented as 32 lowercase hex characters.
func generateRequestID() (string, error) {
	var id [16]byte

	if _, err := io.ReadFull(rand.Reader, id[:]); err != nil {
		return "", err
	}

	return hex.EncodeToString(id[:]), nil
}

// requestIDFromContext returns the server-generated request ID.
func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

// requestMiddleware adds a unique request ID and baseline
// security headers to every response.
func requestMiddleware(next http.Handler) http.Handler {
	return requestMiddlewareWithGenerator(next, generateRequestID)
}

// requestMiddlewareWithGenerator allows deterministic tests
// and simulation of random-number generation failures.
func requestMiddlewareWithGenerator(
	next http.Handler,
	generate requestIDGenerator,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")

		// Never trust a client-supplied X-Request-ID.
		w.Header().Del("X-Request-ID")

		requestID, err := generate()
		if err != nil {
			writeAPIError(
				w,
				http.StatusInternalServerError,
				ErrorCodeInternal,
				"Internal server error",
				"",
			)
			return
		}

		w.Header().Set("X-Request-ID", requestID)

		ctx := context.WithValue(
			r.Context(),
			requestIDContextKey{},
			requestID,
		)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
