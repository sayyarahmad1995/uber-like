package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
	"github.com/sayyarahmad1995/uber-like/internal/domain/user"
)

type userRepository struct{ q querier }

func (r userRepository) Get(ctx context.Context, id user.ID) (user.User, error) {
	var out user.User
	err := r.q.QueryRowContext(ctx, `SELECT id, status FROM users WHERE id = $1`, id).
		Scan(&out.ID, &out.Status)
	if err != nil {
		return user.User{}, notFound(err)
	}
	return out, nil
}

func (r userRepository) GetByOIDCSubject(ctx context.Context, subject string) (user.User, error) {
	var out user.User
	err := r.q.QueryRowContext(ctx, `SELECT id, status FROM users WHERE oidc_subject = $1`, subject).
		Scan(&out.ID, &out.Status)
	if err != nil {
		return user.User{}, notFound(err)
	}
	return out, nil
}

var _ application.UserRepository = userRepository{}
var _ = uuid.Nil
