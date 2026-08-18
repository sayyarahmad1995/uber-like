package reservationapp

import (
	"context"
	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
	domainreservation "github.com/sayyarahmad1995/uber-like/internal/domain/reservation"
)

type Confirm struct {
	Transactions application.TransactionManager
}

func (uc Confirm) Execute(ctx context.Context, reservationID domainreservation.ID, driverID uuid.UUID) (domainreservation.Reservation, error) {
	if reservationID == uuid.Nil || driverID == uuid.Nil {
		return domainreservation.Reservation{}, application.ErrInvalidArgument
	}

	tx, err := uc.Transactions.Begin(ctx)
	if err != nil {
		return domainreservation.Reservation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	r, err := tx.Reservations().Get(ctx, reservationID)
	if err != nil {
		return domainreservation.Reservation{}, err
	}
	if r.DriverID != driverID {
		return domainreservation.Reservation{}, application.ErrForbidden
	}
	if r.ExpiresAt != nil && !timeNowUTC().Before(*r.ExpiresAt) {
		return domainreservation.Reservation{}, domainreservation.ErrInvalidTransition
	}
	if err := r.Confirm(); err != nil {
		return domainreservation.Reservation{}, err
	}
	if err := tx.Reservations().Save(ctx, r); err != nil {
		return domainreservation.Reservation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domainreservation.Reservation{}, err
	}
	return r, nil
}

func timeNowUTC() time.Time { return time.Now().UTC() }
