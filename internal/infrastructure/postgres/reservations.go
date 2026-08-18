package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/domain/reservation"
)

type reservationRepository struct{ q querier }

func (r reservationRepository) Get(ctx context.Context, id reservation.ID) (reservation.Reservation, error) {
	var out reservation.Reservation
	var expires sql.NullTime
	err := r.q.QueryRowContext(ctx, `SELECT id, ride_id, bid_id, driver_id, status, expires_at FROM reservations WHERE id = $1`, id).
		Scan(&out.ID, &out.RideID, &out.BidID, &out.DriverID, &out.Status, &expires)
	if err != nil { return reservation.Reservation{}, notFound(err) }
	if expires.Valid { out.ExpiresAt = &expires.Time }
	return out, nil
}

func (r reservationRepository) Create(ctx context.Context, v reservation.Reservation) error {
	_, err := r.q.ExecContext(ctx, `INSERT INTO reservations (id, ride_id, bid_id, driver_id, status, expires_at) VALUES ($1,$2,$3,$4,$5,$6)`, v.ID, v.RideID, v.BidID, v.DriverID, v.Status, v.ExpiresAt)
	return err
}

func (r reservationRepository) Save(ctx context.Context, v reservation.Reservation) error {
	res, err := r.q.ExecContext(ctx, `UPDATE reservations SET status = $2, confirmed_at = CASE WHEN $2 = 'confirmed' THEN COALESCE(confirmed_at, now()) ELSE confirmed_at END, cancelled_at = CASE WHEN $2 = 'cancelled' THEN COALESCE(cancelled_at, now()) ELSE cancelled_at END WHERE id = $1`, v.ID, v.Status)
	if err != nil { return err }
	n, err := res.RowsAffected()
	if err != nil { return err }
	if n != 1 { return sql.ErrNoRows }
	return nil
}

func (r reservationRepository) GetActiveForRide(ctx context.Context, rideID uuid.UUID) (reservation.Reservation, error) {
	var out reservation.Reservation
	var expires sql.NullTime
	err := r.q.QueryRowContext(ctx, `SELECT id, ride_id, bid_id, driver_id, status, expires_at FROM reservations WHERE ride_id = $1 AND status IN ('pending','confirmed') ORDER BY created_at DESC LIMIT 1`, rideID).
		Scan(&out.ID, &out.RideID, &out.BidID, &out.DriverID, &out.Status, &expires)
	if err != nil { return reservation.Reservation{}, notFound(err) }
	if expires.Valid { out.ExpiresAt = timePtr(expires.Time) }
	return out, nil
}
