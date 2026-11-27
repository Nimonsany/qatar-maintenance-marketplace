package authboundary

import (
	"context"
	"time"

	"qatar-maintenance/backend/internal/application"
	"qatar-maintenance/backend/internal/marketplace"
)

// CredentialSource is trusted server-side infrastructure. It MUST NOT extract
// X-User-ID, X-Role or other unsigned client claims as identity. No concrete
// source or HTTP adapter is provided by this package.
type CredentialSource interface {
	Credential(context.Context) (string, error)
}

// Verifier must cryptographically authenticate credentials in any future
// production integration. There is deliberately no implementation here.
type Verifier interface {
	Verify(context.Context, string) (Principal, error)
}

// SessionState is current authoritative server-side identity/session state.
// Revoked sessions, changed roles or changed versions are always rejected.
// If state is unavailable, access fails closed.
type SessionState struct {
	Subject        string
	Role           marketplace.Role
	SessionVersion uint64
	Revoked        bool
}

type SessionChecker interface {
	CheckSession(context.Context, string) (SessionState, error)
}

// Resolver implements application.IdentityResolver but is not installed in
// main.go or the DEV-007 HTTP adapter. All three dependencies are required;
// there is no permissive or anonymous fallback.
type Resolver struct {
	source   CredentialSource
	verifier Verifier
	sessions SessionChecker
	req      Requirements
	now      func() time.Time
}

var _ application.IdentityResolver = (*Resolver)(nil)

func NewResolver(source CredentialSource, verifier Verifier, sessions SessionChecker, req Requirements) (*Resolver, error) {
	if source == nil || verifier == nil || sessions == nil || !req.valid() {
		return nil, ErrMisconfigured
	}
	return &Resolver{source: source, verifier: verifier, sessions: sessions, req: req, now: time.Now}, nil
}

// ResolveIdentity is designed for server-side wiring after a separate approval.
// Neither credentials nor underlying errors are exposed to consumers.
func (r *Resolver) ResolveIdentity(ctx context.Context) (marketplace.Actor, error) {
	if r == nil || r.source == nil || r.verifier == nil || r.sessions == nil || !r.req.valid() {
		return marketplace.Actor{}, ErrMisconfigured
	}
	if ctx == nil {
		return marketplace.Actor{}, ErrUnauthenticated
	}
	if err := ctx.Err(); err != nil {
		return marketplace.Actor{}, err
	}
	credential, err := r.source.Credential(ctx)
	if err != nil || !clean(credential) || len(credential) > 256 {
		return marketplace.Actor{}, ErrUnauthenticated
	}
	p, err := r.verifier.Verify(ctx, credential)
	if err != nil || ValidatePrincipal(p, r.req, r.now()) != nil {
		return marketplace.Actor{}, ErrUnauthenticated
	}
	state, err := r.sessions.CheckSession(ctx, p.SessionID)
	if err != nil || state.Revoked || state.Subject != p.Subject ||
		state.Role != p.Role || state.SessionVersion != p.SessionVersion {
		return marketplace.Actor{}, ErrUnauthenticated
	}
	if err := ctx.Err(); err != nil {
		return marketplace.Actor{}, err
	}
	return marketplace.Actor{ID: p.Subject, Role: p.Role}, nil
}
