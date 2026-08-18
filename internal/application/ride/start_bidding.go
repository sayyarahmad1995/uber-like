package rideapp

import (
	"context"

	"github.com/sayyarahmad1995/uber-like/internal/application"
	domainride "github.com/sayyarahmad1995/uber-like/internal/domain/ride"
)

type StartBidding struct {
	Rides application.RideRepository
}

func (uc StartBidding) Execute(ctx context.Context, rideID domainride.ID, riderID domainride.ID) (domainride.Ride, error) {
	r, err := uc.Rides.Get(ctx, rideID)
	if err != nil {
		return domainride.Ride{}, err
	}
	if r.RiderID != riderID {
		return domainride.Ride{}, application.ErrForbidden
	}
	if err := r.StartBidding(); err != nil {
		return domainride.Ride{}, err
	}
	if err := uc.Rides.Save(ctx, r); err != nil {
		return domainride.Ride{}, err
	}
	return r, nil
}
