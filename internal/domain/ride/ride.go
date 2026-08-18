package ride

import (
	"errors"
	"github.com/google/uuid"
)

type ID = uuid.UUID

type Status string

const (
	StatusRequested     Status = "requested"
	StatusBidding       Status = "bidding"
	StatusReserved      Status = "reserved"
	StatusAssigned      Status = "assigned"
	StatusDriverArrived Status = "driver_arrived"
	StatusInProgress    Status = "in_progress"
	StatusCompleted     Status = "completed"
	StatusCancelled     Status = "cancelled"
)

var ErrInvalidTransition = errors.New("invalid ride state transition")

type Ride struct {
	ID     ID
	RiderID uuid.UUID
	Status Status
}

func New(id ID, riderID uuid.UUID) Ride {
	return Ride{ID: id, RiderID: riderID, Status: StatusRequested}
}

func (r *Ride) StartBidding() error {
	if r.Status != StatusRequested { return ErrInvalidTransition }
	r.Status = StatusBidding
	return nil
}

func (r *Ride) Reserve() error {
	if r.Status != StatusBidding { return ErrInvalidTransition }
	r.Status = StatusReserved
	return nil
}

func (r *Ride) Assign() error {
	if r.Status != StatusReserved { return ErrInvalidTransition }
	r.Status = StatusAssigned
	return nil
}

func (r *Ride) DriverArrived() error {
	if r.Status != StatusAssigned { return ErrInvalidTransition }
	r.Status = StatusDriverArrived
	return nil
}

func (r *Ride) Start() error {
	if r.Status != StatusDriverArrived { return ErrInvalidTransition }
	r.Status = StatusInProgress
	return nil
}

func (r *Ride) Complete() error {
	if r.Status != StatusInProgress { return ErrInvalidTransition }
	r.Status = StatusCompleted
	return nil
}

func (r *Ride) Cancel() error {
	switch r.Status {
	case StatusRequested, StatusBidding, StatusReserved, StatusAssigned, StatusDriverArrived:
		r.Status = StatusCancelled
		return nil
	default:
		return ErrInvalidTransition
	}
}

func (r Ride) IsTerminal() bool {
	return r.Status == StatusCompleted || r.Status == StatusCancelled
}
