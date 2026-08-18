package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/domain/ride"
)

type rideRepository struct{ q querier }

func (r rideRepository) Get(ctx context.Context, id ride.ID) (ride.Ride, error) {
	var out ride.Ride
	err := r.q.QueryRowContext(ctx, `
		SELECT id, rider_id, status
		FROM rides
		WHERE id = $1`, id).Scan(&out.ID, &out.RiderID, &out.Status)
	if err != nil {
		return ride.Ride{}, notFound(err)
	}
	return out, nil
}

func (r rideRepository) Create(ctx context.Context, v ride.Ride) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO rides (id, rider_id, status, pickup_latitude, pickup_longitude,
			dropoff_latitude, dropoff_longitude)
		VALUES ($1, $2, $3, 0, 0, 0, 0)`, v.ID, v.RiderID, v.Status)
	return err
}

func (r rideRepository) Save(ctx context.Context, v ride.Ride) error {
	res, err := r.q.ExecContext(ctx, `
		UPDATE rides
		SET status = $2, updated_at = now(),
			bidding_started_at = CASE WHEN $2 = 'bidding' THEN COALESCE(bidding_started_at, now()) ELSE bidding_started_at END,
			reserved_at = CASE WHEN $2 = 'reserved' THEN COALESCE(reserved_at, now()) ELSE reserved_at END,
			started_at = CASE WHEN $2 = 'in_progress' THEN COALESCE(started_at, now()) ELSE started_at END,
			completed_at = CASE WHEN $2 = 'completed' THEN COALESCE(completed_at, now()) ELSE completed_at END,
			cancelled_at = CASE WHEN $2 = 'cancelled' THEN COALESCE(cancelled_at, now()) ELSE cancelled_at END
		WHERE id = $1`, v.ID, v.Status)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}

var _ = uuid.Nil
