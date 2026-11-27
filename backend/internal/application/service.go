// Package application provides a non-production, transport-independent
// application layer on top of the DEV-005 marketplace domain.
// No HTTP endpoint or authentication mechanism is registered by this package.
package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"qatar-maintenance/backend/internal/marketplace"
)

var ErrUnauthenticated = errors.New("authentication required")

// IdentityResolver is a TRUST BOUNDARY. Only trusted server-side authentication
// infrastructure may implement it. Never derive an Actor from client-provided
// IDs, role headers, query parameters, or unsigned tokens.
// The production authentication integration is deliberately NOT implemented.
type IdentityResolver interface {
	ResolveIdentity(context.Context) (marketplace.Actor, error)
}

// Repository expresses the existing DEV-005 domain operations without imposing
// a database design. The development adapter uses only in-memory storage.
type Repository interface {
	Create(marketplace.Actor, marketplace.RequestInput) (marketplace.Request, error)
	Get(marketplace.Actor, string) (marketplace.Request, error)
	ListMine(marketplace.Actor) ([]marketplace.Request, error)
	Quote(marketplace.Actor, string, int64) (marketplace.Request, error)
	Accept(marketplace.Actor, string) (marketplace.Request, error)
	Assign(marketplace.Actor, string) (marketplace.Request, error)
	Start(marketplace.Actor, string) (marketplace.Request, error)
	Complete(marketplace.Actor, string) (marketplace.Request, error)
	Cancel(marketplace.Actor, string) (marketplace.Request, error)
}

// MemoryRepository is an explicitly non-production DEV-005 adapter.
// All state disappears when its process exits.
type MemoryRepository struct{ *marketplace.Store }

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{Store: marketplace.NewStore()}
}

var _ Repository = (*MemoryRepository)(nil)

type Service struct {
	identities IdentityResolver
	repo       Repository
}

func NewService(identities IdentityResolver, repo Repository) (*Service, error) {
	if identities == nil || repo == nil {
		return nil, fmt.Errorf("%w: identity resolver and repository required", marketplace.ErrInvalid)
	}
	return &Service{identities: identities, repo: repo}, nil
}

func (s *Service) actor(ctx context.Context) (marketplace.Actor, error) {
	if ctx == nil {
		return marketplace.Actor{}, ErrUnauthenticated
	}
	if err := ctx.Err(); err != nil {
		return marketplace.Actor{}, err
	}
	// Identity is never selected from user-supplied API DTOs.
	actor, err := s.identities.ResolveIdentity(ctx)
	if err != nil {
		return marketplace.Actor{}, ErrUnauthenticated
	}
	if actor.ID == "" || actor.ID != strings.TrimSpace(actor.ID) {
		return marketplace.Actor{}, ErrUnauthenticated
	}
	switch actor.Role {
	case marketplace.Customer, marketplace.Worker, marketplace.Company, marketplace.Admin:
		return actor, nil
	default:
		return marketplace.Actor{}, ErrUnauthenticated
	}
}

// RequestDraft is a transport-neutral API input contract. Identity and status
// are intentionally absent; only the trusted resolver supplies an actor.
type RequestDraft struct {
	Category    marketplace.Category `json:"category"`
	Description string               `json:"description"`
	LocationRef string               `json:"location_ref"`
	ScheduledAt time.Time            `json:"scheduled_at"`
	PhotoRefs   []string             `json:"photo_refs,omitempty"`
}

type QuoteDraft struct {
	AmountMinor int64 `json:"amount_minor"`
}

// RequestView is a prospective API response DTO, not a live HTTP response.
type RequestView struct {
	ID                 string               `json:"id"`
	CustomerID         string               `json:"customer_id"`
	Category           marketplace.Category `json:"category"`
	Description        string               `json:"description"`
	LocationRef        string               `json:"location_ref"`
	ScheduledAt        time.Time            `json:"scheduled_at"`
	PhotoRefs          []string             `json:"photo_refs,omitempty"`
	Status             marketplace.Status   `json:"status"`
	Quote              *marketplace.Quote   `json:"quote,omitempty"`
	AssignedProviderID string               `json:"assigned_provider_id,omitempty"`
}

func toView(r marketplace.Request) RequestView {
	view := RequestView{
		ID: r.ID, CustomerID: r.CustomerID, Category: r.Category,
		Description: r.Description, LocationRef: r.LocationRef,
		ScheduledAt: r.ScheduledAt, Status: r.Status,
		PhotoRefs:          append([]string(nil), r.PhotoRefs...),
		AssignedProviderID: r.AssignedProviderID,
	}
	if r.Quote != nil {
		q := *r.Quote
		view.Quote = &q
	}
	return view
}

func (s *Service) Create(ctx context.Context, draft RequestDraft) (RequestView, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return RequestView{}, err
	}
	r, err := s.repo.Create(actor, marketplace.RequestInput{
		Category: draft.Category, Description: draft.Description,
		LocationRef: draft.LocationRef, ScheduledAt: draft.ScheduledAt,
		PhotoRefs: append([]string(nil), draft.PhotoRefs...),
	})
	if err != nil {
		return RequestView{}, err
	}
	return toView(r), nil
}

func (s *Service) Get(ctx context.Context, id string) (RequestView, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return RequestView{}, err
	}
	if id == "" {
		return RequestView{}, marketplace.ErrInvalid
	}
	r, err := s.repo.Get(actor, id)
	if err != nil {
		return RequestView{}, err
	}
	return toView(r), nil
}

func (s *Service) ListMine(ctx context.Context) ([]RequestView, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return nil, err
	}
	results, err := s.repo.ListMine(actor)
	if err != nil {
		return nil, err
	}
	out := make([]RequestView, 0, len(results))
	for _, r := range results {
		out = append(out, toView(r))
	}
	return out, nil
}

func (s *Service) Quote(ctx context.Context, id string, draft QuoteDraft) (RequestView, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return RequestView{}, err
	}
	r, err := s.repo.Quote(actor, id, draft.AmountMinor)
	if err != nil {
		return RequestView{}, err
	}
	return toView(r), nil
}

func (s *Service) Accept(ctx context.Context, id string) (RequestView, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return RequestView{}, err
	}
	r, err := s.repo.Accept(actor, id)
	if err != nil {
		return RequestView{}, err
	}
	return toView(r), nil
}

func (s *Service) Assign(ctx context.Context, id string) (RequestView, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return RequestView{}, err
	}
	r, err := s.repo.Assign(actor, id)
	if err != nil {
		return RequestView{}, err
	}
	return toView(r), nil
}

func (s *Service) Start(ctx context.Context, id string) (RequestView, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return RequestView{}, err
	}
	r, err := s.repo.Start(actor, id)
	if err != nil {
		return RequestView{}, err
	}
	return toView(r), nil
}

func (s *Service) Complete(ctx context.Context, id string) (RequestView, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return RequestView{}, err
	}
	r, err := s.repo.Complete(actor, id)
	if err != nil {
		return RequestView{}, err
	}
	return toView(r), nil
}

func (s *Service) Cancel(ctx context.Context, id string) (RequestView, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return RequestView{}, err
	}
	r, err := s.repo.Cancel(actor, id)
	if err != nil {
		return RequestView{}, err
	}
	return toView(r), nil
}
