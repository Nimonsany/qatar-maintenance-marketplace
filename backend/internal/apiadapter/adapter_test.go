package apiadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"qatar-maintenance/backend/internal/application"
	"qatar-maintenance/backend/internal/marketplace"
)

type identityKey struct{}
type trustedResolver struct{}

func (trustedResolver) ResolveIdentity(ctx context.Context) (marketplace.Actor, error) {
	actor, ok := ctx.Value(identityKey{}).(marketplace.Actor)
	if !ok {
		return marketplace.Actor{}, errors.New("missing trusted test fixture")
	}
	return actor, nil
}
func fixture(t *testing.T) *Adapter {
	t.Helper()
	svc, err := application.NewService(trustedResolver{}, application.NewMemoryRepository())
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewIsolatedAdapter(svc)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func request(a *Adapter, method, path, body string, actor *marketplace.Actor, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	if actor != nil {
		r = r.WithContext(context.WithValue(r.Context(), identityKey{}, *actor))
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func actor(id string, role marketplace.Role) *marketplace.Actor {
	return &marketplace.Actor{ID: id, Role: role}
}
func draftJSON() string {
	b, _ := json.Marshal(application.RequestDraft{Category: marketplace.Plumbing, Description: "Fix leaking kitchen water pipe", LocationRef: "test-zone", ScheduledAt: time.Now().Add(time.Hour)})
	return string(b)
}
func newRequest(t *testing.T, a *Adapter) string {
	t.Helper()
	w := request(a, "POST", "/v1/requests", draftJSON(), actor("c1", marketplace.Customer), nil)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var response struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.ID) != 32 {
		t.Fatal("invalid id")
	}
	return response.Data.ID
}
func TestNoServiceIsRejected(t *testing.T) {
	if _, err := NewIsolatedAdapter(nil); err == nil {
		t.Fatal("nil service accepted")
	}
}
func TestUnauthenticatedAndHeaderSpoofing(t *testing.T) {
	a := fixture(t)
	h := map[string]string{"X-User-ID": "admin", "X-Role": "admin", "Authorization": "Bearer forged"}
	for _, tc := range []struct{ method, path, body string }{{"GET", "/v1/requests", ""}, {"POST", "/v1/requests", draftJSON()}} {
		w := request(a, tc.method, tc.path, tc.body, nil, h)
		if w.Code != 401 {
			t.Fatalf("spoofed auth %s -> %d %s", tc.path, w.Code, w.Body.String())
		}
	}
}
func TestLifecycleIsolationAndAuthorization(t *testing.T) {
	a := fixture(t)
	id := newRequest(t, a)
	path := "/v1/requests/" + id
	if w := request(a, "GET", path, "", actor("other", marketplace.Customer), nil); w.Code != 404 {
		t.Fatalf("other read: %d", w.Code)
	}
	if w := request(a, "GET", path, "", actor("w1", marketplace.Worker), nil); w.Code != 404 {
		t.Fatalf("unquoted provider read: %d", w.Code)
	}
	if w := request(a, "POST", path+"/accept", "", actor("other", marketplace.Customer), nil); w.Code != 404 {
		t.Fatalf("other accept: %d", w.Code)
	}
	actions := []struct {
		suffix, body string
		who          *marketplace.Actor
		want         int
	}{
		{"/quote", `{"amount_minor":1500}`, actor("w1", marketplace.Worker), 200},
		{"/accept", "", actor("c1", marketplace.Customer), 200},
		{"/assign", "", actor("w1", marketplace.Worker), 403},
		{"/assign", "", actor("a1", marketplace.Admin), 200},
		{"/start", "", actor("w2", marketplace.Worker), 404},
		{"/start", "", actor("w1", marketplace.Worker), 200},
		{"/complete", "", actor("w1", marketplace.Worker), 200},
		{"/cancel", "", actor("a1", marketplace.Admin), 409},
	}
	for _, step := range actions {
		w := request(a, "POST", path+step.suffix, step.body, step.who, nil)
		if w.Code != step.want {
			t.Fatalf("%s: got %d want %d body %s", step.suffix, w.Code, step.want, w.Body.String())
		}
	}
	if w := request(a, "GET", "/v1/requests", "", actor("other", marketplace.Customer), nil); w.Code != 200 || strings.Contains(w.Body.String(), id) {
		t.Fatalf("list leak: %d %s", w.Code, w.Body.String())
	}
}
func TestStrictValidationAndSafeErrors(t *testing.T) {
	a := fixture(t)
	owner := actor("c1", marketplace.Customer)
	cases := []string{"", `[]`, `{"unknown":"field"}`, `{"category":"plumbing","category":"painting"}`, `{"description":"test"} {"extra":1}`, strings.Repeat(" ", int(MaxBodyBytes)+1)}
	for _, body := range cases {
		w := request(a, "POST", "/v1/requests", body, owner, nil)
		if w.Code != 400 {
			t.Errorf("invalid payload -> %d: %s", w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct {
		method, path string
		want         int
	}{{"PUT", "/v1/requests", 405}, {"GET", "/v1/requests/nope", 404}, {"POST", "/v1/requests/" + strings.Repeat("a", 32) + "/bogus", 404}, {"GET", "/other", 404}} {
		w := request(a, tc.method, tc.path, "", owner, nil)
		if w.Code != tc.want {
			t.Errorf("%s %s: got %d", tc.method, tc.path, w.Code)
		}
		if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
			t.Error("missing json type")
		}
	}
}
func TestConcurrentDuplicateQuoteOnlyOneSuccess(t *testing.T) {
	a := fixture(t)
	id := newRequest(t, a)
	path := "/v1/requests/" + id + "/quote"
	const count = 32
	var wg sync.WaitGroup
	out := make(chan int, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := request(a, "POST", path, `{"amount_minor":500}`, actor("w1", marketplace.Worker), nil)
			out <- w.Code
		}()
	}
	wg.Wait()
	close(out)
	success := 0
	for code := range out {
		if code == 200 {
			success++
		} else if code != 409 {
			t.Errorf("unexpected code %d", code)
		}
	}
	if success != 1 {
		t.Fatalf("got %d quotes, want one", success)
	}
}
func TestResponseDoesNotEchoUntrustedHeaders(t *testing.T) {
	a := fixture(t)
	id := newRequest(t, a)
	w := request(a, "GET", "/v1/requests/"+id, "", actor("c1", marketplace.Customer), map[string]string{"X-Request-ID": "attacker-controlled", "X-Role": "admin"})
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("attacker-controlled")) {
		t.Fatal("injected header reflected")
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatal(got)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatal(got)
	}
}
