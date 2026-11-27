package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"qatar-maintenance/backend/internal/marketplace"
)

type contextKey struct{}
type testResolver struct{}

func (testResolver) ResolveIdentity(ctx context.Context) (marketplace.Actor, error) {
	actor, ok := ctx.Value(contextKey{}).(marketplace.Actor)
	if !ok {
		return marketplace.Actor{}, errors.New("no trusted identity")
	}
	return actor, nil
}
func identity(id string, role marketplace.Role) context.Context {
	return context.WithValue(context.Background(), contextKey{}, marketplace.Actor{ID: id, Role: role})
}
func setup(t *testing.T) *Service {
	t.Helper()
	s, err := NewService(testResolver{}, NewMemoryRepository())
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func draft() RequestDraft {
	return RequestDraft{Category: marketplace.Plumbing,
		Description: "Fix leaking pipe in kitchen",
		LocationRef: "nonprod-zone-a", ScheduledAt: time.Now().Add(24 * time.Hour),
		PhotoRefs: []string{"test-photo"}}
}
func requireStatus(t *testing.T, got RequestView, err error, expected marketplace.Status) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != expected {
		t.Fatalf("status %q, want %q", got.Status, expected)
	}
}

func TestServiceLifecycleAndIsolation(t *testing.T) {
	s := setup(t)
	owner := identity("customer-1", marketplace.Customer)
	other := identity("customer-2", marketplace.Customer)
	provider := identity("worker-1", marketplace.Worker)
	wrongProvider := identity("worker-2", marketplace.Worker)
	admin := identity("admin-1", marketplace.Admin)

	request, err := s.Create(owner, draft())
	requireStatus(t, request, err, marketplace.Requested)
	if _, err := s.Get(other, request.ID); !errors.Is(err, marketplace.ErrNotFound) {
		t.Fatalf("other customer saw request: %v", err)
	}
	if _, err := s.Get(provider, request.ID); !errors.Is(err, marketplace.ErrNotFound) {
		t.Fatalf("unquoted provider saw request: %v", err)
	}
	if _, err := s.Accept(other, request.ID); !errors.Is(err, marketplace.ErrNotFound) {
		t.Fatalf("other customer accepted: %v", err)
	}
	quote, err := s.Quote(provider, request.ID, QuoteDraft{AmountMinor: 4500})
	requireStatus(t, quote, err, marketplace.Quoted)
	if _, err := s.Quote(wrongProvider, request.ID, QuoteDraft{AmountMinor: 5000}); !errors.Is(err, marketplace.ErrConflict) {
		t.Fatalf("duplicate quotation: %v", err)
	}
	accepted, err := s.Accept(owner, request.ID)
	requireStatus(t, accepted, err, marketplace.Accepted)
	if _, err := s.Accept(owner, request.ID); !errors.Is(err, marketplace.ErrConflict) {
		t.Fatalf("duplicate acceptance: %v", err)
	}
	if _, err := s.Assign(provider, request.ID); !errors.Is(err, marketplace.ErrForbidden) {
		t.Fatalf("provider assigned request: %v", err)
	}
	assigned, err := s.Assign(admin, request.ID)
	requireStatus(t, assigned, err, marketplace.Assigned)
	if _, err := s.Start(wrongProvider, request.ID); !errors.Is(err, marketplace.ErrNotFound) {
		t.Fatalf("wrong provider started: %v", err)
	}
	started, err := s.Start(provider, request.ID)
	requireStatus(t, started, err, marketplace.InProgress)
	finished, err := s.Complete(provider, request.ID)
	requireStatus(t, finished, err, marketplace.Completed)
	if _, err := s.Cancel(admin, request.ID); !errors.Is(err, marketplace.ErrConflict) {
		t.Fatalf("cancelled completed request: %v", err)
	}
	listings, err := s.ListMine(other)
	if err != nil || len(listings) != 0 {
		t.Fatalf("ownership listing leaked: %v, %v", listings, err)
	}
}

func TestUntrustedIdentityAndDTOValidation(t *testing.T) {
	s := setup(t)
	if _, err := s.Create(context.Background(), draft()); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("missing identity: %v", err)
	}
	for _, actor := range []marketplace.Actor{
		{ID: "", Role: marketplace.Customer},
		{ID: " customer ", Role: marketplace.Customer},
		{ID: "customer", Role: "superuser"},
	} {
		ctx := context.WithValue(context.Background(), contextKey{}, actor)
		if _, err := s.Create(ctx, draft()); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("invalid actor %+v accepted: %v", actor, err)
		}
	}
	if _, err := s.Create(identity("worker-1", marketplace.Worker), draft()); !errors.Is(err, marketplace.ErrForbidden) {
		t.Fatalf("worker create: %v", err)
	}
	bad := draft()
	bad.Description = "short"
	if _, err := s.Create(identity("customer-1", marketplace.Customer), bad); !errors.Is(err, marketplace.ErrInvalid) {
		t.Fatalf("invalid request: %v", err)
	}
	ctx, cancel := context.WithCancel(identity("customer-1", marketplace.Customer))
	cancel()
	if _, err := s.Create(ctx, draft()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context: %v", err)
	}
}

func TestDTOCopyIsolationAndCancellation(t *testing.T) {
	s := setup(t)
	owner := identity("customer-1", marketplace.Customer)
	input := draft()
	request, err := s.Create(owner, input)
	if err != nil {
		t.Fatal(err)
	}
	input.PhotoRefs[0] = "modified"
	request.PhotoRefs[0] = "modified"
	got, err := s.Get(owner, request.ID)
	if err != nil || got.PhotoRefs[0] != "test-photo" {
		t.Fatalf("mutated persisted data: %+v %v", got, err)
	}
	got, err = s.Cancel(owner, request.ID)
	requireStatus(t, got, err, marketplace.Cancelled)
	if _, err := s.Quote(identity("worker-1", marketplace.Worker), request.ID, QuoteDraft{AmountMinor: 100}); !errors.Is(err, marketplace.ErrConflict) {
		t.Fatalf("quoted cancelled request: %v", err)
	}
}

func TestConcurrentQuotationOnlyOneWinner(t *testing.T) {
	s := setup(t)
	request, err := s.Create(identity("customer-1", marketplace.Customer), draft())
	if err != nil {
		t.Fatal(err)
	}
	const attempts = 40
	var wg sync.WaitGroup
	results := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Quote(identity("worker-1", marketplace.Worker), request.ID, QuoteDraft{AmountMinor: 100})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, marketplace.ErrConflict) {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful quotes=%d, want 1", successes)
	}
}

func TestErrorMappingIsSanitized(t *testing.T) {
	cases := []struct {
		err  error
		code ErrorCode
	}{
		{ErrUnauthenticated, CodeUnauthenticated},
		{marketplace.ErrInvalid, CodeInvalid},
		{marketplace.ErrForbidden, CodeForbidden},
		{marketplace.ErrNotFound, CodeNotFound},
		{marketplace.ErrConflict, CodeConflict},
		{context.Canceled, CodeUnavailable},
		{errors.New("SECRET database password"), CodeInternal},
	}
	for _, tc := range cases {
		got := MapError(tc.err)
		if got.Code != tc.code {
			t.Fatalf("%v -> %s want %s", tc.err, got.Code, tc.code)
		}
		if got.Message == "SECRET database password" {
			t.Fatal("sensitive error leaked")
		}
	}
}

func TestServiceRequiresDependencies(t *testing.T) {
	if _, err := NewService(nil, NewMemoryRepository()); !errors.Is(err, marketplace.ErrInvalid) {
		t.Fatalf("missing resolver: %v", err)
	}
	if _, err := NewService(testResolver{}, nil); !errors.Is(err, marketplace.ErrInvalid) {
		t.Fatalf("missing repo: %v", err)
	}
}
