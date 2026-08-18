package bidapp

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
	domainbid "github.com/sayyarahmad1995/uber-like/internal/domain/bid"
	domaindriver "github.com/sayyarahmad1995/uber-like/internal/domain/driver"
	domainride "github.com/sayyarahmad1995/uber-like/internal/domain/ride"
)

type Create struct {
	Rides       application.RideRepository
	Bids        application.BidRepository
	Drivers     application.DriverRepository
	Eligibility application.EligibilityChecker
}

func (uc Create) Execute(ctx context.Context, rideID domainride.ID, driverID uuid.UUID, amountMinor int64, currency string, expiresAt *time.Time) (domainbid.Bid, error) {
	if driverID == uuid.Nil || rideID == uuid.Nil || amountMinor < 0 || currency == "" {
		return domainbid.Bid{}, application.ErrInvalidArgument
	}
	if _, err := uc.Drivers.Get(ctx, driverID); err != nil {
		return domainbid.Bid{}, err
	}
	eligible, err := uc.Eligibility.IsEligible(ctx, driverID)
	if err != nil {
		return domainbid.Bid{}, err
	}
	if !eligible {
		return domainbid.Bid{}, application.ErrNotEligible
	}

	r, err := uc.Rides.Get(ctx, rideID)
	if err != nil {
		return domainbid.Bid{}, err
	}
	if r.Status != domainride.StatusBidding {
		return domainbid.Bid{}, application.ErrConflict
	}

	active, err := uc.Bids.HasActiveForDriver(ctx, rideID, driverID)
	if err != nil {
		return domainbid.Bid{}, err
	}
	if active {
		return domainbid.Bid{}, application.ErrConflict
	}

	b, err := domainbid.New(uuid.New(), rideID, driverID, amountMinor, currency)
	if err != nil {
		return domainbid.Bid{}, err
	}
	b.ExpiresAt = expiresAt
	if err := b.Activate(); err != nil {
		return domainbid.Bid{}, err
	}
	if err := uc.Bids.Create(ctx, b); err != nil {
		return domainbid.Bid{}, err
	}
	return b, nil
}

var _ = domaindriver.StatusActive
