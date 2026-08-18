package postgres

import (
	"context"
	"database/sql"

	"github.com/sayyarahmad1995/uber-like/internal/application"
)

type Tx struct {
	tx *sql.Tx
}

func (t *Tx) Commit(ctx context.Context) error {
	return t.tx.Commit()
}

func (t *Tx) Rollback(ctx context.Context) error {
	return t.tx.Rollback()
}

func (t *Tx) Rides() application.RideRepository { return rideRepository{q: t.tx} }
func (t *Tx) Bids() application.BidRepository { return bidRepository{q: t.tx} }
func (t *Tx) Reservations() application.ReservationRepository {
	return reservationRepository{q: t.tx}
}
func (t *Tx) Assignments() application.AssignmentRepository { return assignmentRepository{q: t.tx} }
func (t *Tx) Drivers() application.DriverRepository { return driverRepository{q: t.tx} }

type querier interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func notFound(err error) error {
	if err == sql.ErrNoRows {
		return application.ErrNotFound
	}
	return err
}
