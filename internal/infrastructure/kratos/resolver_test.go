package kratos

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
	"github.com/sayyarahmad1995/uber-like/internal/domain/user"
)

type fakeUsers struct {
	user user.User
	err  error
}

func (f fakeUsers) GetByOIDCSubject(context.Context, string) (user.User, error) {
	return f.user, f.err
}

func TestResolverResolvesActiveUser(t *testing.T) {
	userID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/sessions/whoami" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer session-token" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"active":true,"identity":{"id":"kratos-id-1"}}`))
	}))
	defer server.Close()

	resolver := NewResolver(server.URL, fakeUsers{user: user.User{ID: userID, Status: user.StatusActive}})
	got, err := resolver.Resolve(context.Background(), "session-token")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	want := application.Identity{Subject: "kratos-id-1", UserID: userID}
	if got != want {
		t.Fatalf("identity = %#v, want %#v", got, want)
	}
}

func TestResolverRejectsInvalidSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	resolver := NewResolver(server.URL, fakeUsers{})
	_, err := resolver.Resolve(context.Background(), "bad")
	if !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("error = %v, want ErrInvalidSession", err)
	}
}

func TestResolverRejectsUnknownUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"active":true,"identity":{"id":"kratos-id-2"}}`))
	}))
	defer server.Close()

	resolver := NewResolver(server.URL, fakeUsers{err: application.ErrNotFound})
	_, err := resolver.Resolve(context.Background(), "session")
	if !errors.Is(err, ErrUnknownUser) {
		t.Fatalf("error = %v, want ErrUnknownUser", err)
	}
}

func TestResolverRejectsInactiveUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"active":true,"identity":{"id":"kratos-id-3"}}`))
	}))
	defer server.Close()

	resolver := NewResolver(server.URL, fakeUsers{user: user.User{ID: uuid.New(), Status: user.StatusSuspended}})
	_, err := resolver.Resolve(context.Background(), "session")
	if !errors.Is(err, ErrInactiveUser) {
		t.Fatalf("error = %v, want ErrInactiveUser", err)
	}
}
