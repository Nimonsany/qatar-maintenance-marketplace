// Package authboundary declares isolated, non-production authentication trust
// contracts. It creates no tokens, sessions, network listeners or HTTP routes.
package authboundary

import "errors"

var (
	ErrUnauthenticated = errors.New("authentication required")
	ErrForbidden       = errors.New("access denied")
	ErrMisconfigured   = errors.New("authentication boundary unavailable")
)
