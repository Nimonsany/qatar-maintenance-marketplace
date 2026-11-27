package marketplace

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func input() RequestInput {
	return RequestInput{Category: Plumbing, Description: "Fix leaking water pipe", LocationRef: "test-zone-1", ScheduledAt: time.Now().Add(24 * time.Hour), PhotoRefs: []string{"image-test-1"}}
}
func requireStatus(t *testing.T, got Request, err error, want Status) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != want {
		t.Fatalf("status = %q; want %q", got.Status, want)
	}
}
func TestFullSimulation(t *testing.T) {
	s := NewStore()
	customer := Actor{"c1", Customer}
	provider := Actor{"w1", Worker}
	admin := Actor{"a1", Admin}
	r, err := s.Create(customer, input())
	requireStatus(t, r, err, Requested)
	r, err = s.Quote(provider, r.ID, 1200)
	requireStatus(t, r, err, Quoted)
	r, err = s.Accept(customer, r.ID)
	requireStatus(t, r, err, Accepted)
	r, err = s.Assign(admin, r.ID)
	requireStatus(t, r, err, Assigned)
	r, err = s.Start(provider, r.ID)
	requireStatus(t, r, err, InProgress)
	r, err = s.Complete(provider, r.ID)
	requireStatus(t, r, err, Completed)
	if _, err := s.Cancel(admin, r.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("cancel completed: %v", err)
	}
}
func TestInvalidInputAndRole(t *testing.T) {
	s := NewStore()
	a := Actor{"c1", Customer}
	good := input()
	tests := []struct {
		name   string
		modify func(*RequestInput)
	}{
		{"category", func(i *RequestInput) { i.Category = "unknown" }},
		{"description", func(i *RequestInput) { i.Description = "short" }},
		{"location", func(i *RequestInput) { i.LocationRef = "" }},
		{"schedule", func(i *RequestInput) { i.ScheduledAt = time.Now().Add(-time.Hour) }},
		{"photos", func(i *RequestInput) { i.PhotoRefs = []string{"same", "same"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := good
			tt.modify(&i)
			if _, err := s.Create(a, i); !errors.Is(err, ErrInvalid) {
				t.Fatalf("want ErrInvalid got %v", err)
			}
		})
	}
	if _, err := s.Create(Actor{"worker", Worker}, good); !errors.Is(err, ErrForbidden) {
		t.Fatalf("worker create: %v", err)
	}
	if _, err := s.Create(Actor{"", Customer}, good); !errors.Is(err, ErrInvalid) {
		t.Fatalf("untrusted actor: %v", err)
	}
}
func TestIsolationAndCopies(t *testing.T) {
	s := NewStore()
	owner := Actor{"c1", Customer}
	other := Actor{"c2", Customer}
	worker := Actor{"w1", Worker}
	in := input()
	r, err := s.Create(owner, in)
	if err != nil {
		t.Fatal(err)
	}
	in.PhotoRefs[0] = "changed"
	r.PhotoRefs[0] = "changed"
	if _, err := s.Get(other, r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other read: %v", err)
	}
	if _, err := s.Accept(other, r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other accept: %v", err)
	}
	if _, err := s.Get(worker, r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unquoted worker read: %v", err)
	}
	actual, err := s.Get(owner, r.ID)
	if err != nil || actual.PhotoRefs[0] != "image-test-1" {
		t.Fatalf("copy isolation: %+v %v", actual, err)
	}
	listing, err := s.ListMine(other)
	if err != nil || len(listing) != 0 {
		t.Fatalf("list isolation: %v %v", listing, err)
	}
}
func TestDuplicateQuoteAcceptanceAndPermissions(t *testing.T) {
	s := NewStore()
	c := Actor{"c1", Customer}
	w := Actor{"w1", Worker}
	other := Actor{"w2", Worker}
	admin := Actor{"a1", Admin}
	r, err := s.Create(c, input())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Assign(admin, r.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("premature assign: %v", err)
	}
	if _, err := s.Quote(w, r.ID, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid quote: %v", err)
	}
	if _, err := s.Quote(w, r.ID, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Quote(other, r.ID, 110); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate quote: %v", err)
	}
	if _, err := s.Accept(c, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Accept(c, r.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate accept: %v", err)
	}
	if _, err := s.Assign(w, r.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unauthorized assign: %v", err)
	}
	if _, err := s.Assign(admin, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(other, r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong provider start: %v", err)
	}
}
func TestCancellationAndConcurrentQuotes(t *testing.T) {
	s := NewStore()
	c := Actor{"c1", Customer}
	r, err := s.Create(c, input())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Quote(Actor{"w1", Worker}, r.ID, 100); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			} else if !errors.Is(err, ErrConflict) {
				t.Errorf("quote: %v", err)
			}
		}()
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("successful concurrent quotes = %d; want 1", successes)
	}
	got, err := s.Cancel(c, r.ID)
	requireStatus(t, got, err, Cancelled)
	if _, err := s.Accept(c, r.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("accept cancelled: %v", err)
	}
}
