package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteAPIErrorNotFound(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeAPIError(
		recorder,
		http.StatusNotFound,
		ErrorCodeNotFound,
		"Resource not found",
		"test-request-123",
	)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"expected HTTP 404, got %d",
			recorder.Code,
		)
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("unexpected Content-Type: %q", got)
	}

	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("unexpected Cache-Control: %q", got)
	}

	var response APIErrorResponse

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if response.Error.Code != ErrorCodeNotFound {
		t.Errorf(
			"unexpected error code: %q",
			response.Error.Code,
		)
	}

	if response.Error.Message != "Resource not found" {
		t.Errorf(
			"unexpected error message: %q",
			response.Error.Message,
		)
	}

	if response.Error.RequestID != "test-request-123" {
		t.Errorf(
			"unexpected request ID: %q",
			response.Error.RequestID,
		)
	}
}

func TestWriteAPIErrorMethodNotAllowed(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeAPIError(
		recorder,
		http.StatusMethodNotAllowed,
		ErrorCodeMethodNotAllowed,
		"HTTP method not allowed",
		"test-request-456",
	)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected HTTP 405, got %d",
			recorder.Code,
		)
	}

	var response APIErrorResponse

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if response.Error.Code != ErrorCodeMethodNotAllowed {
		t.Errorf("unexpected code: %q", response.Error.Code)
	}

	if response.Error.RequestID != "test-request-456" {
		t.Errorf("unexpected request ID: %q", response.Error.RequestID)
	}
}

func TestWriteAPIErrorDoesNotExposeInternalDetails(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeAPIError(
		recorder,
		http.StatusInternalServerError,
		ErrorCodeInternal,
		"Internal server error",
		"safe-request-id",
	)

	body := recorder.Body.String()

	if strings.Contains(body, "password=") {
		t.Error("response contains sensitive information")
	}

	if strings.Contains(body, "goroutine ") {
		t.Error("response contains stack trace")
	}

	if !strings.Contains(body, "INTERNAL_ERROR") {
		t.Error("expected internal error code")
	}

	if !strings.Contains(body, "safe-request-id") {
		t.Error("request ID missing")
	}
}
