package bidapp

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
	domainbid "github.com/sayyarahmad1995/uber-like/internal/domain/bid"
	"github.com/sayyarahmad1995/uber-like/internal/domain/reservation"
	domainride "github.com/sayyarahmad1995/uber-like/internal/domain/ride"
)

type Select struct {
	Transactions application.TransactionManager
	Clock        application.Clock
}

type SelectionResult struct {
	Ride        domainride.Ride
	Bid         domainbid.Bid
	Reservation reservation.Reservation
}

func (uc Select) Execute(ctx context.Context, bidID domainbid.ID, riderID uuid.UUID) (SelectionResult, error) {
	if bidID == uuid.Nil || riderID == uuid.Nil {
		return SelectionResult{}, application.ErrInvalidArgument
	}

	tx, err := uc.Transactions.Begin(ctx)
	if err != nil {
		return SelectionResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	b, err := tx.Bids().Get(ctx, bidID)
	if err != nil {
		return SelectionResult{}, err
	}
	if b.ExpiresAt != nil && !uc.Clock.Now().Before(*b.ExpiresAt) {
		return SelectionResult{}, domainbid.ErrInvalidTransition
	}

	r, err := tx.Rides().Get(ctx, domainride.ID(b.RideID))
	if err != nil {
		return SelectionResult{}, err
	}
	if r.RiderID != riderID {
		return SelectionResult{}, application.ErrForbidden
	}
	if r.Status != domainride.StatusBidding {
		return SelectionResult{}, application.ErrConflict
	}
	if err := b.Select(); err != nil {
		return SelectionResult{}, err
	}
	if err := r.Reserve(); err != nil {
		return SelectionResult{}, err
	}

	now := uc.Clock.Now()
	rsv := reservation.New(uuid.New(), r.ID, b.ID, b.DriverID, &now)
	if err := tx.Bids().Save(ctx, b); err != nil {
		return SelectionResult{}, err
	}
	if err := tx.Rides().Save(ctx, r); err != nil {
		return SelectionResult{}, err
	}
	if err := tx.Reservations().Create(ctx, rsv); err != nil {
		return SelectionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SelectionResult{}, err
	}

	return SelectionResult{Ride: r, Bid: b, Reservation: rsv}, nil
}

var _ = time.Time{}
