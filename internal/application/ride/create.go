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

func (uc CreateRide) Execute(ctx context.Context, riderID uuid.UUID) (domainride.Ride, error) {
	if riderID == uuid.Nil {
		return domainride.Ride{}, application.ErrInvalidArgument
	}

	r := domainride.New(uuid.New(), riderID)
	if err := uc.Rides.Create(ctx, r); err != nil {
		return domainride.Ride{}, err
	}
	return r, nil
}
