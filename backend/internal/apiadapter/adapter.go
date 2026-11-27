// Package apiadapter models an isolated HTTP contract adapter for DEV-007.
// Nothing in this package registers routes with the running application.
// This adapter is for isolated contract/security tests only, not a public API.
package apiadapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"qatar-maintenance/backend/internal/application"
	"qatar-maintenance/backend/internal/marketplace"
)

const MaxBodyBytes int64 = 16 * 1024
const prefix = "/v1/requests"

type envelope struct {
	Data  any                        `json:"data,omitempty"`
	Error *application.ErrorContract `json:"error,omitempty"`
}

// Adapter is NOT attached to the production mux. It relies on application's
// injected trusted IdentityResolver, which must never read request-supplied
// IDs, role headers or unsigned claims. Do not register in production.
type Adapter struct{ svc *application.Service }

func NewIsolatedAdapter(svc *application.Service) (*Adapter, error) {
	if svc == nil {
		return nil, errors.New("application service required")
	}
	return &Adapter{svc: svc}, nil
}

func writeJSON(w http.ResponseWriter, status int, body envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func statusFor(err error) int {
	switch application.MapError(err).Code {
	case application.CodeUnauthenticated:
		return http.StatusUnauthorized
	case application.CodeInvalid:
		return http.StatusBadRequest
	case application.CodeForbidden:
		return http.StatusForbidden
	case application.CodeNotFound:
		return http.StatusNotFound
	case application.CodeConflict:
		return http.StatusConflict
	case application.CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
func fail(w http.ResponseWriter, status int, code application.ErrorCode, msg string) {
	writeJSON(w, status, envelope{Error: &application.ErrorContract{Code: code, Message: msg}})
}
func mapped(w http.ResponseWriter, err error) {
	c := application.MapError(err)
	writeJSON(w, statusFor(err), envelope{Error: &c})
}

// duplicateKeys checks object keys recursively before decoding into DTOs.
func duplicateKeys(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]bool)
		for dec.More() {
			keyToken, e := dec.Token()
			if e != nil {
				return e
			}
			key, ok := keyToken.(string)
			if !ok || seen[key] {
				return marketplace.ErrInvalid
			}
			seen[key] = true
			if e := duplicateKeys(dec); e != nil {
				return e
			}
		}
	case '[':
		for dec.More() {
			if e := duplicateKeys(dec); e != nil {
				return e
			}
		}
	default:
		return marketplace.ErrInvalid
	}
	_, err = dec.Token()
	return err
}

// decodeStrict requires one JSON object, no duplicate/unknown keys, no trailing
// JSON values, and a bounded body. Credentials are never DTO fields.
func decodeStrict(w http.ResponseWriter, r *http.Request, v any) error {
	if r.Body == nil {
		return marketplace.ErrInvalid
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return marketplace.ErrInvalid
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return marketplace.ErrInvalid
	}
	scan := json.NewDecoder(bytes.NewReader(raw))
	if err := duplicateKeys(scan); err != nil {
		return marketplace.ErrInvalid
	}
	var extra any
	if err := scan.Decode(&extra); err != io.EOF {
		return marketplace.ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return marketplace.ErrInvalid
	}
	return nil
}
func idValid(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func onlyGetOrPost(r *http.Request, method string) bool { return r.Method == method }
func methodNotAllowed(w http.ResponseWriter, method string) {
	w.Header().Set("Allow", method)
	fail(w, http.StatusMethodNotAllowed, application.CodeInvalid, "Method not allowed")
}

// ServeHTTP exposes a test-only virtual contract. There is intentionally no
// registration in main.go, buildApplication, or the production HTTP server.
func (a *Adapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a == nil || a.svc == nil {
		fail(w, 500, application.CodeInternal, "Internal error")
		return
	}
	if r.URL.Path != prefix && !strings.HasPrefix(r.URL.Path, prefix+"/") {
		fail(w, 404, application.CodeNotFound, "Resource not found")
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	if rest == "" {
		switch r.Method {
		case http.MethodGet:
			data, err := a.svc.ListMine(r.Context())
			if err != nil {
				mapped(w, err)
				return
			}
			writeJSON(w, 200, envelope{Data: data})
		case http.MethodPost:
			var input application.RequestDraft
			if err := decodeStrict(w, r, &input); err != nil {
				mapped(w, err)
				return
			}
			data, err := a.svc.Create(r.Context(), input)
			if err != nil {
				mapped(w, err)
				return
			}
			writeJSON(w, 201, envelope{Data: data})
		default:
			w.Header().Set("Allow", "GET, POST")
			fail(w, 405, application.CodeInvalid, "Method not allowed")
		}
		return
	}
	parts := strings.Split(strings.TrimPrefix(rest, "/"), "/")
	if len(parts) < 1 || len(parts) > 2 || !idValid(parts[0]) {
		fail(w, 404, application.CodeNotFound, "Resource not found")
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		if !onlyGetOrPost(r, http.MethodGet) {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		data, err := a.svc.Get(r.Context(), id)
		if err != nil {
			mapped(w, err)
			return
		}
		writeJSON(w, 200, envelope{Data: data})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var data application.RequestView
	var err error
	switch parts[1] {
	case "quote":
		var draft application.QuoteDraft
		if e := decodeStrict(w, r, &draft); e != nil {
			mapped(w, e)
			return
		}
		data, err = a.svc.Quote(r.Context(), id, draft)
	case "accept":
		data, err = a.svc.Accept(r.Context(), id)
	case "assign":
		data, err = a.svc.Assign(r.Context(), id)
	case "start":
		data, err = a.svc.Start(r.Context(), id)
	case "complete":
		data, err = a.svc.Complete(r.Context(), id)
	case "cancel":
		data, err = a.svc.Cancel(r.Context(), id)
	default:
		fail(w, 404, application.CodeNotFound, "Resource not found")
		return
	}
	if err != nil {
		mapped(w, err)
		return
	}
	writeJSON(w, 200, envelope{Data: data})
}
