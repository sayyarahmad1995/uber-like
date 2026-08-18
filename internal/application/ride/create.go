package rideapp

import (
	"context"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
	domainride "github.com/sayyarahmad1995/uber-like/internal/domain/ride"
)

type CreateRide struct {
	Rides application.RideRepository
}

func (uc CreateRide) Execute(ctx context.Context, riderID uuid.UUID, pickup, dropoff domainride.Coordinate) (domainride.Ride, error) {
	if riderID == uuid.Nil || !pickup.Valid() || !dropoff.Valid() {
		return domainride.Ride{}, application.ErrInvalidArgument
	}

	r, err := domainride.New(uuid.New(), riderID, pickup, dropoff)
	if err != nil {
		return domainride.Ride{}, application.ErrInvalidArgument
	}
	if err := uc.Rides.Create(ctx, r); err != nil {
		return domainride.Ride{}, err
	}
	return r, nil
}
