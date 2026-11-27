package application

import (
	"context"
	"errors"

	"qatar-maintenance/backend/internal/marketplace"
)

// ErrorCode maps a domain/application error to a future API error category.
// No HTTP handlers are created or registered in DEV-006.
type ErrorCode string

const (
	CodeUnauthenticated ErrorCode = "unauthenticated"
	CodeInvalid         ErrorCode = "invalid_input"
	CodeForbidden       ErrorCode = "forbidden"
	CodeNotFound        ErrorCode = "not_found"
	CodeConflict        ErrorCode = "conflict"
	CodeUnavailable     ErrorCode = "unavailable"
	CodeInternal        ErrorCode = "internal"
)

type ErrorContract struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// MapError intentionally avoids exposing internal error strings.
func MapError(err error) ErrorContract {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ErrorContract{CodeUnavailable, "Request interrupted"}
	case errors.Is(err, ErrUnauthenticated):
		return ErrorContract{CodeUnauthenticated, "Authentication required"}
	case errors.Is(err, marketplace.ErrInvalid):
		return ErrorContract{CodeInvalid, "Invalid input"}
	case errors.Is(err, marketplace.ErrForbidden):
		return ErrorContract{CodeForbidden, "Access denied"}
	case errors.Is(err, marketplace.ErrNotFound):
		return ErrorContract{CodeNotFound, "Resource not found"}
	case errors.Is(err, marketplace.ErrConflict):
		return ErrorContract{CodeConflict, "Conflicting operation"}
	default:
		return ErrorContract{CodeInternal, "Internal error"}
	}
}
