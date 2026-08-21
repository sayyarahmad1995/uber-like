package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
	"github.com/sayyarahmad1995/uber-like/internal/domain/user"
)

type provisionSessionResolver struct{}

func (provisionSessionResolver) ResolveSubject(context.Context, string) (string, error) {
	return "kratos-id", nil
}

type provisionUsers struct{}

func (provisionUsers) Get(context.Context, user.ID) (user.User, error) { return user.User{}, application.ErrNotFound }
func (provisionUsers) GetByOIDCSubject(context.Context, string) (user.User, error) {
	return user.User{}, application.ErrNotFound
}
func (provisionUsers) ProvisionByOIDCSubject(context.Context, string) (user.User, error) {
	return user.New(uuid.MustParse("11111111-1111-1111-1111-111111111111")), nil
}

func TestProvisionHandlerRejectsMissingBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/provision", nil)
	rec := httptest.NewRecorder()

	ProvisionHandler{Auth: application.AuthService{Sessions: provisionSessionResolver{}, Users: provisionUsers{}}}.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestProvisionHandlerReturnsLocalIdentity(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/provision", nil)
	req.Header.Set("Authorization", "Bearer session-token")
	rec := httptest.NewRecorder()

	h := ProvisionHandler{Auth: application.AuthService{Sessions: provisionSessionResolver{}, Users: provisionUsers{}}}
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got == "" || got == "{}\n" {
		t.Fatalf("response body = %q, want identity JSON", got)
	}
}
