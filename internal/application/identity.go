package application

import (
	"context"
	"github.com/google/uuid"
)

type Identity struct {
	Subject string
	UserID  uuid.UUID
}

type IdentityResolver interface {
	Resolve(ctx context.Context, token string) (Identity, error)
}
