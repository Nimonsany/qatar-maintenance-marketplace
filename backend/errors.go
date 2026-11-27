package main

import (
	"encoding/json"
	"net/http"
)

// APIError represents a safe, structured API error.
type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// APIErrorResponse is the public JSON error envelope.
type APIErrorResponse struct {
	Error APIError `json:"error"`
}

const (
	ErrorCodeNotFound         = "NOT_FOUND"
	ErrorCodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	ErrorCodeInternal         = "INTERNAL_ERROR"
)

// writeAPIError writes a consistent JSON error response.
//
// The caller must supply an approved public error message.
// Internal errors, secrets and stack traces must never be
// passed into this function as public messages.
func writeAPIError(
	w http.ResponseWriter,
	statusCode int,
	code string,
	message string,
	requestID string,
) {
	response := APIErrorResponse{
		Error: APIError{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	}

	payload, err := json.Marshal(response)
	if err != nil {
		http.Error(
			w,
			"Internal server error",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	w.WriteHeader(statusCode)

	// Encoding is complete before headers are committed.
	// ResponseWriter errors cannot be recovered by sending
	// a second HTTP response.
	_, _ = w.Write(append(payload, '\n'))
}
