package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sayyarahmad1995/uber-like/internal/application"
	"github.com/sayyarahmad1995/uber-like/internal/domain/user"
)

type logoutSessionManager struct {
	err       error
	loggedOut bool
}

func (m *logoutSessionManager) ResolveSubject(context.Context, string) (string, error) {
	return "subject", nil
}

func (m *logoutSessionManager) Logout(context.Context, string) error {
	m.loggedOut = true
	return m.err
}

type logoutUsers struct{}

func (logoutUsers) Get(context.Context, user.ID) (user.User, error) {
	return user.User{}, application.ErrNotFound
}

func (logoutUsers) GetByOIDCSubject(context.Context, string) (user.User, error) {
	return user.User{}, application.ErrNotFound
}

func (logoutUsers) ProvisionByOIDCSubject(context.Context, string) (user.User, error) {
	return user.User{}, errors.New("unused")
}

func TestLogoutHandlerRejectsMissingBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	LogoutHandler{Auth: application.AuthService{Sessions: &logoutSessionManager{}}}.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogoutHandlerRevokesSession(t *testing.T) {
	sessions := &logoutSessionManager{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer session-token")
	rec := httptest.NewRecorder()

	LogoutHandler{Auth: application.AuthService{Sessions: sessions}}.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if !sessions.loggedOut {
		t.Fatal("session logout was not invoked")
	}
}

func TestLogoutHandlerRejectsLogoutFailure(t *testing.T) {
	sessions := &logoutSessionManager{err: errors.New("logout failed")}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer session-token")
	rec := httptest.NewRecorder()

	LogoutHandler{Auth: application.AuthService{Sessions: sessions}}.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
