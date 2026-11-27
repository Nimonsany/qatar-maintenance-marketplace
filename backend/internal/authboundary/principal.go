package authboundary

import (
	"strings"
	"time"

	"qatar-maintenance/backend/internal/marketplace"
)

// Principal contains claims confirmed by a trusted verifier, not a client DTO.
// SessionVersion is checked against current server-side session state on EACH
// resolution. This package does not establish Qatar worker/legal eligibility.
type Principal struct {
	Subject        string
	Role           marketplace.Role
	Issuer         string
	Audience       string
	SessionID      string
	SessionVersion uint64
	IssuedAt       time.Time
	ExpiresAt      time.Time
}

// Requirements are deliberately caller-owned so a later production provider
// integration must specify an exact issuer and audience.
type Requirements struct {
	Issuer   string
	Audience string
	MaxAge   time.Duration
}

func clean(s string) bool {
	return s != "" && strings.TrimSpace(s) == s &&
		!strings.ContainsAny(s, "\x00\r\n\t") && len(s) <= 256
}

func validRole(role marketplace.Role) bool {
	switch role {
	case marketplace.Customer, marketplace.Worker, marketplace.Company, marketplace.Admin:
		return true
	default:
		return false
	}
}

func (r Requirements) valid() bool {
	return clean(r.Issuer) && clean(r.Audience) && r.MaxAge > 0 && r.MaxAge <= 24*time.Hour
}

// ValidatePrincipal rejects empty, expired, future-dated, wrong-issuer and
// wrong-audience identities. It does NOT cryptographically verify a credential:
// only a trusted Verifier may return a Principal to the Resolver.
func ValidatePrincipal(p Principal, req Requirements, now time.Time) error {
	if !req.valid() || now.IsZero() {
		return ErrMisconfigured
	}
	if !clean(p.Subject) || !validRole(p.Role) || !clean(p.SessionID) ||
		p.SessionVersion == 0 || p.Issuer != req.Issuer || p.Audience != req.Audience ||
		p.IssuedAt.IsZero() || p.ExpiresAt.IsZero() ||
		p.IssuedAt.After(now) || !p.ExpiresAt.After(now) ||
		!p.ExpiresAt.After(p.IssuedAt) ||
		now.Sub(p.IssuedAt) > req.MaxAge || p.ExpiresAt.Sub(p.IssuedAt) > req.MaxAge {
		return ErrUnauthenticated
	}
	return nil
}
