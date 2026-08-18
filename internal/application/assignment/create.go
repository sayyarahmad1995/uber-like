package assignmentapp

import (
	"context"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
	domainassignment "github.com/sayyarahmad1995/uber-like/internal/domain/assignment"
	domainreservation "github.com/sayyarahmad1995/uber-like/internal/domain/reservation"
	domainride "github.com/sayyarahmad1995/uber-like/internal/domain/ride"
)

type Create struct {
	Transactions application.TransactionManager
	Eligibility  application.EligibilityChecker
	Clock        application.Clock
}

type Result struct {
	Ride       domainride.Ride
	Assignment domainassignment.Assignment
}

func (uc Create) Execute(ctx context.Context, reservationID domainreservation.ID) (Result, error) {
	if reservationID == uuid.Nil {
		return Result{}, application.ErrInvalidArgument
	}

	tx, err := uc.Transactions.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rsv, err := tx.Reservations().Get(ctx, reservationID)
	if err != nil {
		return Result{}, err
	}
	if rsv.Status != domainreservation.StatusConfirmed {
		return Result{}, application.ErrConflict
	}
	if rsv.ExpiresAt != nil && !uc.Clock.Now().Before(*rsv.ExpiresAt) {
		return Result{}, application.ErrConflict
	}
	eligible, err := uc.Eligibility.IsEligible(ctx, rsv.DriverID)
	if err != nil {
		return Result{}, err
	}
	if !eligible {
		return Result{}, application.ErrNotEligible
	}
	if active, err := tx.Assignments().HasActiveForDriver(ctx, rsv.DriverID); err != nil {
		return Result{}, err
	} else if active {
		return Result{}, application.ErrConflict
	}

	r, err := tx.Rides().Get(ctx, domainride.ID(rsv.RideID))
	if err != nil {
		return Result{}, err
	}
	if err := r.Assign(); err != nil {
		return Result{}, err
	}

	a := domainassignment.New(uuid.New(), r.ID, rsv.DriverID)
	if err := tx.Rides().Save(ctx, r); err != nil {
		return Result{}, err
	}
	if err := tx.Assignments().Create(ctx, a); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{Ride: r, Assignment: a}, nil
}
