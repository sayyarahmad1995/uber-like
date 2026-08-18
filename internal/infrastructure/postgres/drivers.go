package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/domain/driver"
)

type driverRepository struct{ q querier }

func (r driverRepository) Get(ctx context.Context, id uuid.UUID) (driver.Driver, error) {
	var out driver.Driver
	err := r.q.QueryRowContext(ctx, `SELECT id, user_id, status FROM driver_profiles WHERE id = $1`, id).
		Scan(&out.ID, &out.UserID, &out.Status)
	if err != nil { return driver.Driver{}, notFound(err) }
	return out, nil
}
