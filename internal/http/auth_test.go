package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
)

type testResolver struct {
	identity application.Identity
	err      error
	token    string
}

func (r testResolver) Resolve(_ context.Context, token string) (application.Identity, error) {
	r.token = token
	return r.identity, r.err
}

func TestAuthMiddlewareRejectsMissingBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	AuthMiddleware{Resolver: testResolver{}}.Middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareSetsIdentity(t *testing.T) {
	id := uuid.New()
	identity := application.Identity{Subject: "subject-1", UserID: id}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := IdentityFromContext(r.Context())
		if !ok {
			t.Fatal("identity missing from context")
		}
		if got != identity {
			t.Fatalf("identity = %#v, want %#v", got, identity)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	AuthMiddleware{Resolver: testResolver{identity: identity}}.Middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestMustIdentityReportsMissingIdentity(t *testing.T) {
	_, err := MustIdentity(context.Background())
	if err != ErrMissingIdentity {
		t.Fatalf("error = %v, want %v", err, ErrMissingIdentity)
	}
}
