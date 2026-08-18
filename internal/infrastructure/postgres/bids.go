package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/domain/bid"
)

type bidRepository struct{ q querier }

func (r bidRepository) Get(ctx context.Context, id bid.ID) (bid.Bid, error) {
	var out bid.Bid
	var expires sql.NullTime
	err := r.q.QueryRowContext(ctx, `SELECT id, ride_id, driver_id, amount_minor, currency, status, expires_at FROM bids WHERE id = $1`, id).
		Scan(&out.ID, &out.RideID, &out.DriverID, &out.AmountMinor, &out.Currency, &out.Status, &expires)
	if err != nil { return bid.Bid{}, notFound(err) }
	if expires.Valid { out.ExpiresAt = timePtr(expires.Time) }
	return out, nil
}

func (r bidRepository) Create(ctx context.Context, v bid.Bid) error {
	_, err := r.q.ExecContext(ctx, `INSERT INTO bids (id, ride_id, driver_id, amount_minor, currency, status, expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, v.ID, v.RideID, v.DriverID, v.AmountMinor, v.Currency, v.Status, v.ExpiresAt)
	return err
}

func (r bidRepository) Save(ctx context.Context, v bid.Bid) error {
	res, err := r.q.ExecContext(ctx, `UPDATE bids SET status = $2, updated_at = now() WHERE id = $1`, v.ID, v.Status)
	if err != nil { return err }
	n, err := res.RowsAffected()
	if err != nil { return err }
	if n != 1 { return sql.ErrNoRows }
	return nil
}

func (r bidRepository) HasActiveForDriver(ctx context.Context, rideID, driverID uuid.UUID) (bool, error) {
	var exists bool
	err := r.q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM bids WHERE ride_id = $1 AND driver_id = $2 AND status IN ('submitted','active'))`, rideID, driverID).Scan(&exists)
	return exists, err
}

func timePtr(t time.Time) *time.Time { return &t }
