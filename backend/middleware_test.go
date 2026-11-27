package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestGenerateRequestID(t *testing.T) {
	first, err := generateRequestID()
	if err != nil {
		t.Fatalf("failed to generate first ID: %v", err)
	}

	second, err := generateRequestID()
	if err != nil {
		t.Fatalf("failed to generate second ID: %v", err)
	}

	pattern := regexp.MustCompile(`^[0-9a-f]{32}$`)

	if !pattern.MatchString(first) {
		t.Errorf("invalid first request ID: %q", first)
	}

	if !pattern.MatchString(second) {
		t.Errorf("invalid second request ID: %q", second)
	}

	if first == second {
		t.Error("two generated request IDs must differ")
	}
}

func TestRequestMiddlewareSetsHeaders(t *testing.T) {
	const expectedID = "0123456789abcdef0123456789abcdef"

	handler := requestMiddlewareWithGenerator(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := requestIDFromContext(r.Context()); got != expectedID {
				t.Errorf("unexpected context request ID: %q", got)
			}

			w.WriteHeader(http.StatusOK)
		}),
		func() (string, error) {
			return expectedID, nil
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	expectedHeaders := map[string]string{
		"X-Request-ID":           expectedID,
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	}

	for name, expected := range expectedHeaders {
		if got := recorder.Header().Get(name); got != expected {
			t.Errorf(
				"header %s: expected %q, got %q",
				name,
				expected,
				got,
			)
		}
	}
}

func TestRequestMiddlewareRejectsClientID(t *testing.T) {
	const serverID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	handler := requestMiddlewareWithGenerator(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := requestIDFromContext(r.Context()); got != serverID {
				t.Errorf("client ID was trusted: %q", got)
			}

			w.WriteHeader(http.StatusOK)
		}),
		func() (string, error) {
			return serverID, nil
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set("X-Request-ID", "client-controlled-id")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Request-ID"); got != serverID {
		t.Errorf("expected server-generated ID, got %q", got)
	}
}

func TestRequestMiddlewareRandomFailure(t *testing.T) {
	called := false

	handler := requestMiddlewareWithGenerator(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}),
		func() (string, error) {
			return "", errors.New("simulated entropy failure")
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if called {
		t.Error("next handler must not execute after ID generation failure")
	}

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("expected HTTP 500, got %d", recorder.Code)
	}

	if got := recorder.Header().Get("X-Request-ID"); got != "" {
		t.Errorf("unexpected request ID after failure: %q", got)
	}

	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("missing security header after failure: %q", got)
	}

	var response APIErrorResponse

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON error: %v", err)
	}

	if response.Error.Code != ErrorCodeInternal {
		t.Errorf("unexpected error code: %q", response.Error.Code)
	}

	if response.Error.RequestID != "" {
		t.Error("failed generator must not produce a fake request ID")
	}

	if response.Error.Message != "Internal server error" {
		t.Errorf("unexpected public message: %q", response.Error.Message)
	}
}

func TestRequestMiddlewareAddsIDToErrors(t *testing.T) {
	const expectedID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	handler := requestMiddlewareWithGenerator(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeAPIError(
				w,
				http.StatusNotFound,
				ErrorCodeNotFound,
				"Resource not found",
				requestIDFromContext(r.Context()),
			)
		}),
		func() (string, error) {
			return expectedID, nil
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404, got %d", recorder.Code)
	}

	var response APIErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	headerID := recorder.Header().Get("X-Request-ID")

	if headerID != expectedID {
		t.Errorf("unexpected header request ID: %q", headerID)
	}

	if response.Error.RequestID != headerID {
		t.Error("JSON request ID does not match response header")
	}
}
