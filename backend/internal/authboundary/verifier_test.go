package authboundary

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"qatar-maintenance/backend/internal/application"
	"qatar-maintenance/backend/internal/marketplace"
)

// Test fixtures are deliberately compiled ONLY in _test.go, never shipped.
type fixtureSource struct {
	credential string
	err        error
}

func (s fixtureSource) Credential(context.Context) (string, error) { return s.credential, s.err }

type fixtureVerifier struct {
	principal Principal
	err       error
}

func (v fixtureVerifier) Verify(context.Context, string) (Principal, error) {
	return v.principal, v.err
}

type fixtureSession struct {
	mu    sync.RWMutex
	state SessionState
	err   error
}

func (s *fixtureSession) CheckSession(context.Context, string) (SessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state, s.err
}
func (s *fixtureSession) change(f func(*SessionState)) { s.mu.Lock(); defer s.mu.Unlock(); f(&s.state) }

func base(t *testing.T) (*Resolver, *fixtureSession, Principal) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	p := Principal{Subject: "customer-1", Role: marketplace.Customer, Issuer: "test-issuer", Audience: "test-backend", SessionID: "session-1", SessionVersion: 1, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	state := &fixtureSession{state: SessionState{Subject: p.Subject, Role: p.Role, SessionVersion: p.SessionVersion}}
	r, err := NewResolver(fixtureSource{credential: "opaque-test-reference"}, fixtureVerifier{principal: p}, state, Requirements{Issuer: p.Issuer, Audience: p.Audience, MaxAge: 2 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	r.now = func() time.Time { return now }
	return r, state, p
}
func TestTrustedResolverAndIntegration(t *testing.T) {
	r, _, _ := base(t)
	a, err := r.ResolveIdentity(context.Background())
	if err != nil || a.ID != "customer-1" || a.Role != marketplace.Customer {
		t.Fatalf("actor=%+v err=%v", a, err)
	}
	svc, err := application.NewService(r, application.NewMemoryRepository())
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Create(context.Background(), application.RequestDraft{Category: marketplace.Plumbing, Description: "Fix the leaking kitchen pipe", LocationRef: "test-zone", ScheduledAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
}
func TestResolverDenyByDefaultAndRevocation(t *testing.T) {
	r, state, _ := base(t)
	for _, modify := range []func(*SessionState){
		func(s *SessionState) { s.Revoked = true },
		func(s *SessionState) { s.Role = marketplace.Admin },
		func(s *SessionState) { s.SessionVersion++ },
		func(s *SessionState) { s.Subject = "other" },
	} {
		original := state.state
		state.change(modify)
		if _, err := r.ResolveIdentity(context.Background()); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("state change accepted: %v", err)
		}
		state.change(func(s *SessionState) { *s = original })
	}
	state.mu.Lock()
	state.err = errors.New("internal secret")
	state.mu.Unlock()
	if _, err := r.ResolveIdentity(context.Background()); !errors.Is(err, ErrUnauthenticated) || err.Error() != "authentication required" {
		t.Fatalf("unsanitized session failure: %v", err)
	}
}
func TestPrincipalValidation(t *testing.T) {
	_, _, p := base(t)
	now := p.IssuedAt.Add(time.Minute)
	req := Requirements{Issuer: p.Issuer, Audience: p.Audience, MaxAge: 2 * time.Hour}
	cases := []struct {
		name   string
		change func(*Principal)
	}{
		{"expired", func(p *Principal) { p.ExpiresAt = now }},
		{"future-issued", func(p *Principal) { p.IssuedAt = now.Add(time.Second) }},
		{"wrong-issuer", func(p *Principal) { p.Issuer = "other" }},
		{"wrong-audience", func(p *Principal) { p.Audience = "other" }},
		{"unknown-role", func(p *Principal) { p.Role = "superadmin" }},
		{"blank-subject", func(p *Principal) { p.Subject = "" }},
		{"invalid-subject", func(p *Principal) { p.Subject = " user " }},
		{"missing-session", func(p *Principal) { p.SessionID = "" }},
		{"missing-version", func(p *Principal) { p.SessionVersion = 0 }},
		{"too-long-lifetime", func(p *Principal) { p.ExpiresAt = p.IssuedAt.Add(3 * time.Hour) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cp := p
			tc.change(&cp)
			if err := ValidatePrincipal(cp, req, now); !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if err := ValidatePrincipal(p, Requirements{}, now); !errors.Is(err, ErrMisconfigured) {
		t.Fatal(err)
	}
}
func TestResolverRejectsMissingAndForgedInputs(t *testing.T) {
	r, _, p := base(t)
	if _, err := r.ResolveIdentity(nil); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.ResolveIdentity(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := NewResolver(nil, fixtureVerifier{principal: p}, &fixtureSession{}, r.req); !errors.Is(err, ErrMisconfigured) {
		t.Fatal(err)
	}
	bad := []struct {
		source   fixtureSource
		verifier fixtureVerifier
	}{
		{fixtureSource{credential: ""}, fixtureVerifier{principal: p}},
		{fixtureSource{credential: "X-Role: admin"}, fixtureVerifier{err: errors.New("invalid credential")}},
		{fixtureSource{err: errors.New("secret")}, fixtureVerifier{principal: p}},
	}
	for _, b := range bad {
		x, err := NewResolver(b.source, b.verifier, &fixtureSession{}, r.req)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := x.ResolveIdentity(context.Background()); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("accepted bad input: %v", err)
		}
	}
}
func TestConcurrentRevocationFailClosed(t *testing.T) {
	r, state, _ := base(t)
	const n = 40
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := r.ResolveIdentity(context.Background())
			if err != nil && !errors.Is(err, ErrUnauthenticated) {
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	state.change(func(s *SessionState) { s.Revoked = true })
	wg.Wait()
	for i := 0; i < n; i++ {
		if _, err := r.ResolveIdentity(context.Background()); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("revoked identity accepted: %v", err)
		}
	}
}
