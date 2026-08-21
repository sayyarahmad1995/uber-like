package application

import (
	"context"
	"strings"

	"github.com/sayyarahmad1995/uber-like/internal/domain/user"
)

type AuthService struct {
	Sessions SessionResolver
	Users    UserRepository
}

func (s AuthService) Provision(ctx context.Context, token string) (user.User, error) {
	if s.Sessions == nil || s.Users == nil {
		return user.User{}, ErrInvalidArgument
	}

	subject, err := s.Sessions.ResolveSubject(ctx, token)
	if err != nil {
		return user.User{}, err
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return user.User{}, ErrInvalidArgument
	}

	return s.Users.ProvisionByOIDCSubject(ctx, subject)
}
