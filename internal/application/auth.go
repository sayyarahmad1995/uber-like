package application

import (
	"context"
	"strings"
)

type AuthService struct {
	Sessions SessionManager
	Users    UserRepository
}

func (s AuthService) Provision(ctx context.Context, token string) (Identity, error) {
	if s.Sessions == nil || s.Users == nil {
		return Identity{}, ErrInvalidArgument
	}

	subject, err := s.Sessions.ResolveSubject(ctx, token)
	if err != nil {
		return Identity{}, err
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return Identity{}, ErrInvalidArgument
	}

	localUser, err := s.Users.ProvisionByOIDCSubject(ctx, subject)
	if err != nil {
		return Identity{}, err
	}
	if !localUser.IsActive() {
		return Identity{}, ErrForbidden
	}

	return Identity{Subject: subject, UserID: localUser.ID}, nil
}

func (s AuthService) Logout(ctx context.Context, token string) error {
	if s.Sessions == nil {
		return ErrInvalidArgument
	}
	if strings.TrimSpace(token) == "" {
		return ErrInvalidArgument
	}
	return s.Sessions.Logout(ctx, token)
}
