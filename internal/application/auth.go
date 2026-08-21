package application

import (
	"context"
	"strings"
)

type AuthService struct {
	Sessions SessionResolver
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

	return Identity{Subject: subject, UserID: localUser.ID}, nil
}
