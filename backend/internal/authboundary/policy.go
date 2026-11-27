package authboundary

import "qatar-maintenance/backend/internal/marketplace"

type Action string

const (
	CreateRequest Action = "create_request"
	ListRequests  Action = "list_requests"
	ViewRequest   Action = "view_request"
	QuoteRequest  Action = "quote_request"
	AcceptQuote   Action = "accept_quote"
	AssignWorker  Action = "assign_worker"
	StartWork     Action = "start_work"
	CompleteWork  Action = "complete_work"
	CancelRequest Action = "cancel_request"
)

// PermitRole is only a coarse, deny-by-default gate. It is NOT ownership
// authorization. The DEV-005 domain still performs request-specific checks.
func PermitRole(role marketplace.Role, action Action) error {
	if !validRole(role) {
		return ErrForbidden
	}
	switch action {
	case CreateRequest, AcceptQuote:
		if role == marketplace.Customer {
			return nil
		}
	case ListRequests:
		if role == marketplace.Customer || role == marketplace.Admin {
			return nil
		}
	case ViewRequest:
		return nil // Ownership/assignment check MUST occur in marketplace.Store.Get.
	case QuoteRequest, StartWork, CompleteWork:
		if role == marketplace.Worker || role == marketplace.Company {
			return nil
		}
	case AssignWorker:
		if role == marketplace.Admin {
			return nil
		}
	case CancelRequest:
		if role == marketplace.Customer || role == marketplace.Admin {
			return nil
		}
	}
	return ErrForbidden
}
