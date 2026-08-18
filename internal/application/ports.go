package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/domain/bid"
	"github.com/sayyarahmad1995/uber-like/internal/domain/driver"
	"github.com/sayyarahmad1995/uber-like/internal/domain/reservation"
	"github.com/sayyarahmad1995/uber-like/internal/domain/ride"
)

var (
	ErrNotFound        = errors.New("resource not found")
	ErrForbidden       = errors.New("forbidden")
	ErrConflict        = errors.New("resource conflict")
	ErrNotEligible     = errors.New("driver is not eligible")
	ErrInvalidArgument = errors.New("invalid argument")
)

type RideRepository interface {
	Get(ctx context.Context, id ride.ID) (ride.Ride, error)
	Create(ctx context.Context, r ride.Ride) error
	Save(ctx context.Context, r ride.Ride) error
}

type BidRepository interface {
	Get(ctx context.Context, id bid.ID) (bid.Bid, error)
	Create(ctx context.Context, b bid.Bid) error
	Save(ctx context.Context, b bid.Bid) error
	HasActiveForDriver(ctx context.Context, rideID, driverID uuid.UUID) (bool, error)
}

type ReservationRepository interface {
	Create(ctx context.Context, r reservation.Reservation) error
	GetActiveForRide(ctx context.Context, rideID uuid.UUID) (reservation.Reservation, error)
}

type DriverRepository interface {
	Get(ctx context.Context, id uuid.UUID) (driver.Driver, error)
}

type EligibilityChecker interface {
	IsEligible(ctx context.Context, driverID uuid.UUID) (bool, error)
}

type Transaction interface {
	Rides() RideRepository
	Bids() BidRepository
	Reservations() ReservationRepository
	Drivers() DriverRepository
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type TransactionManager interface {
	Begin(ctx context.Context) (Transaction, error)
}

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }
