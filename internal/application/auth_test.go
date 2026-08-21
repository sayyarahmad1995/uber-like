package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/domain/user"
)

type testSessionResolver struct {
	subject   string
	err       error
	loggedOut bool
}

func (r *testSessionResolver) ResolveSubject(context.Context, string) (string, error) {
	return r.subject, r.err
}

func (r *testSessionResolver) Logout(context.Context, string) error {
	r.loggedOut = true
	return r.err
}

type testUserRepository struct {
	user    user.User
	subject string
	err     error
}

func (r *testUserRepository) Get(context.Context, user.ID) (user.User, error) {
	return r.user, nil
}

func (r *testUserRepository) GetByOIDCSubject(context.Context, string) (user.User, error) {
	return r.user, nil
}

func (r *testUserRepository) ProvisionByOIDCSubject(_ context.Context, subject string) (user.User, error) {
	r.subject = subject
	return r.user, r.err
}

func TestAuthServiceProvision(t *testing.T) {
	id := uuid.New()
	users := &testUserRepository{user: user.New(id)}
	sessions := &testSessionResolver{subject: "kratos-subject"}
	service := AuthService{
		Sessions: sessions,
		Users:    users,
	}

	got, err := service.Provision(context.Background(), "session-token")
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != id || got.Subject != "kratos-subject" {
		t.Fatalf("identity = %#v, want user %s and subject kratos-subject", got, id)
	}
	if users.subject != "kratos-subject" {
		t.Fatalf("provision subject = %q, want kratos-subject", users.subject)
	}
}

func TestAuthServiceLogout(t *testing.T) {
	sessions := &testSessionResolver{}
	service := AuthService{Sessions: sessions}

	if err := service.Logout(context.Background(), "session-token"); err != nil {
		t.Fatal(err)
	}
	if !sessions.loggedOut {
		t.Fatal("session logout was not invoked")
	}
}
