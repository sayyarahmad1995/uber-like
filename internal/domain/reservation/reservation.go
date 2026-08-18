package reservation

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ID = uuid.UUID

type Status string

const (
	StatusPending   Status = "pending"
	StatusConfirmed Status = "confirmed"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

var ErrInvalidTransition = errors.New("invalid reservation state transition")

type Reservation struct {
	ID        ID
	RideID    uuid.UUID
	BidID     uuid.UUID
	DriverID  uuid.UUID
	Status    Status
	ExpiresAt *time.Time
}

func New(id ID, rideID, bidID, driverID uuid.UUID, expiresAt *time.Time) Reservation {
	return Reservation{ID: id, RideID: rideID, BidID: bidID, DriverID: driverID, Status: StatusPending, ExpiresAt: expiresAt}
}

func (r *Reservation) Confirm() error {
	if r.Status != StatusPending { return ErrInvalidTransition }
	r.Status = StatusConfirmed
	return nil
}

func (r *Reservation) Expire() error {
	if r.Status != StatusPending { return ErrInvalidTransition }
	r.Status = StatusExpired
	return nil
}

func (r *Reservation) Cancel() error {
	if r.Status != StatusPending && r.Status != StatusConfirmed { return ErrInvalidTransition }
	r.Status = StatusCancelled
	return nil
}

func (r Reservation) IsTerminal() bool {
	return r.Status == StatusExpired || r.Status == StatusCancelled
}
