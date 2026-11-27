// Package marketplace implements an in-memory, non-production booking domain.
// It does not authenticate callers, verify worker eligibility, or collect money.
package marketplace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Role string

const (
	Customer Role = "customer"
	Worker   Role = "worker"
	Company  Role = "company"
	Admin    Role = "admin"
)

type Status string

const (
	Requested  Status = "requested"
	Quoted     Status = "quoted"
	Accepted   Status = "accepted"
	Assigned   Status = "assigned"
	InProgress Status = "in_progress"
	Completed  Status = "completed"
	Cancelled  Status = "cancelled"
)

type Category string

const (
	Plumbing   Category = "plumbing"
	Electrical Category = "electrical"
	Painting   Category = "painting"
	Mechanical Category = "mechanical"
	Civil      Category = "civil"
	Welding    Category = "welding"
)

var (
	ErrInvalid   = errors.New("invalid input")
	ErrForbidden = errors.New("forbidden")
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("invalid state transition or duplicate action")
)

type Actor struct {
	ID   string
	Role Role
}
type RequestInput struct {
	Category    Category
	Description string
	LocationRef string // Non-production opaque reference; never a raw precise address.
	ScheduledAt time.Time
	PhotoRefs   []string // Non-production opaque identifiers; no file uploads.
}
type Quote struct {
	ProviderID  string
	AmountMinor int64
}
type Request struct {
	ID                 string
	CustomerID         string
	Category           Category
	Description        string
	LocationRef        string
	ScheduledAt        time.Time
	PhotoRefs          []string
	Status             Status
	Quote              *Quote
	AssignedProviderID string
}

func (r Request) clone() Request {
	r.PhotoRefs = append([]string(nil), r.PhotoRefs...)
	if r.Quote != nil {
		q := *r.Quote
		r.Quote = &q
	}
	return r
}
func validActor(a Actor) bool {
	if strings.TrimSpace(a.ID) == "" || a.ID != strings.TrimSpace(a.ID) {
		return false
	}
	switch a.Role {
	case Customer, Worker, Company, Admin:
		return true
	}
	return false
}
func validCategory(c Category) bool {
	switch c {
	case Plumbing, Electrical, Painting, Mechanical, Civil, Welding:
		return true
	}
	return false
}
func validateInput(in RequestInput, now time.Time) error {
	if !validCategory(in.Category) {
		return fmt.Errorf("%w: category", ErrInvalid)
	}
	d := strings.TrimSpace(in.Description)
	if !utf8.ValidString(d) || utf8.RuneCountInString(d) < 10 || utf8.RuneCountInString(d) > 2000 {
		return fmt.Errorf("%w: description", ErrInvalid)
	}
	loc := strings.TrimSpace(in.LocationRef)
	if loc == "" || len(loc) > 128 || strings.ContainsAny(loc, "\r\n") {
		return fmt.Errorf("%w: location reference", ErrInvalid)
	}
	if in.ScheduledAt.IsZero() || !in.ScheduledAt.After(now) || in.ScheduledAt.After(now.Add(365*24*time.Hour)) {
		return fmt.Errorf("%w: schedule", ErrInvalid)
	}
	if len(in.PhotoRefs) > 5 {
		return fmt.Errorf("%w: photo count", ErrInvalid)
	}
	seen := make(map[string]bool, len(in.PhotoRefs))
	for _, p := range in.PhotoRefs {
		if p == "" || len(p) > 128 || p != strings.TrimSpace(p) || strings.ContainsAny(p, "\r\n") || seen[p] {
			return fmt.Errorf("%w: photo reference", ErrInvalid)
		}
		seen[p] = true
	}
	return nil
}

// Store is an isolated development simulator, not a persistence adapter.
// Every operation takes a trusted Actor from its caller. No public HTTP handler uses it.
type Store struct {
	mu       sync.Mutex
	requests map[string]Request
	now      func() time.Time
}

func NewStore() *Store { return &Store{requests: make(map[string]Request), now: time.Now} }
func (s *Store) Create(a Actor, in RequestInput) (Request, error) {
	if !validActor(a) {
		return Request{}, ErrInvalid
	}
	if a.Role != Customer {
		return Request{}, ErrForbidden
	}
	if err := validateInput(in, s.now()); err != nil {
		return Request{}, err
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Request{}, fmt.Errorf("generate request id: %w", err)
	}
	r := Request{ID: hex.EncodeToString(raw[:]), CustomerID: a.ID, Category: in.Category, Description: strings.TrimSpace(in.Description), LocationRef: strings.TrimSpace(in.LocationRef), ScheduledAt: in.ScheduledAt.UTC(), PhotoRefs: append([]string(nil), in.PhotoRefs...), Status: Requested}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests[r.ID] = r
	return r.clone(), nil
}
func (s *Store) Get(a Actor, id string) (Request, error) {
	if !validActor(a) {
		return Request{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return Request{}, ErrNotFound
	}
	// A provider can view a request only after becoming its quoted/assigned provider.
	if a.Role != Admin && !(a.Role == Customer && a.ID == r.CustomerID) && !((a.Role == Worker || a.Role == Company) && r.Quote != nil && a.ID == r.Quote.ProviderID) {
		return Request{}, ErrNotFound
	}
	return r.clone(), nil
}
func (s *Store) ListMine(a Actor) ([]Request, error) {
	if !validActor(a) {
		return nil, ErrInvalid
	}
	if a.Role != Customer && a.Role != Admin {
		return nil, ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, 0)
	for _, r := range s.requests {
		if a.Role == Admin || a.ID == r.CustomerID {
			out = append(out, r.clone())
		}
	}
	return out, nil
}
func (s *Store) Quote(a Actor, id string, amountMinor int64) (Request, error) {
	if !validActor(a) {
		return Request{}, ErrInvalid
	}
	if a.Role != Worker && a.Role != Company {
		return Request{}, ErrForbidden
	}
	if amountMinor <= 0 || amountMinor > 100_000_000_00 {
		return Request{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return Request{}, ErrNotFound
	}
	if r.Status != Requested {
		return Request{}, ErrConflict
	}
	// Eligibility is not established here. No live quotation may be sent.
	r.Quote = &Quote{ProviderID: a.ID, AmountMinor: amountMinor}
	r.Status = Quoted
	s.requests[id] = r
	return r.clone(), nil
}
func (s *Store) Accept(a Actor, id string) (Request, error) {
	if !validActor(a) {
		return Request{}, ErrInvalid
	}
	if a.Role != Customer {
		return Request{}, ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok || r.CustomerID != a.ID {
		return Request{}, ErrNotFound
	}
	if r.Status != Quoted || r.Quote == nil {
		return Request{}, ErrConflict
	}
	r.Status = Accepted
	s.requests[id] = r
	return r.clone(), nil
}
func (s *Store) Assign(a Actor, id string) (Request, error) {
	if !validActor(a) {
		return Request{}, ErrInvalid
	}
	if a.Role != Admin {
		return Request{}, ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return Request{}, ErrNotFound
	}
	if r.Status != Accepted || r.Quote == nil {
		return Request{}, ErrConflict
	}
	r.AssignedProviderID = r.Quote.ProviderID
	r.Status = Assigned
	s.requests[id] = r
	return r.clone(), nil
}
func (s *Store) Start(a Actor, id string) (Request, error) {
	if !validActor(a) {
		return Request{}, ErrInvalid
	}
	if a.Role != Worker && a.Role != Company {
		return Request{}, ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return Request{}, ErrNotFound
	}
	if r.AssignedProviderID != a.ID {
		return Request{}, ErrNotFound
	}
	if r.Status != Assigned {
		return Request{}, ErrConflict
	}
	r.Status = InProgress
	s.requests[id] = r
	return r.clone(), nil
}
func (s *Store) Complete(a Actor, id string) (Request, error) {
	if !validActor(a) {
		return Request{}, ErrInvalid
	}
	if a.Role != Worker && a.Role != Company {
		return Request{}, ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return Request{}, ErrNotFound
	}
	if r.AssignedProviderID != a.ID {
		return Request{}, ErrNotFound
	}
	if r.Status != InProgress {
		return Request{}, ErrConflict
	}
	r.Status = Completed
	s.requests[id] = r
	return r.clone(), nil
}
func (s *Store) Cancel(a Actor, id string) (Request, error) {
	if !validActor(a) {
		return Request{}, ErrInvalid
	}
	if a.Role != Customer && a.Role != Admin {
		return Request{}, ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return Request{}, ErrNotFound
	}
	if a.Role == Customer && r.CustomerID != a.ID {
		return Request{}, ErrNotFound
	}
	switch r.Status {
	case Requested, Quoted, Accepted, Assigned:
	default:
		return Request{}, ErrConflict
	}
	r.Status = Cancelled
	s.requests[id] = r
	return r.clone(), nil
}
