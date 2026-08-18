package assignment

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ID = uuid.UUID

type Status string

const (
	StatusAssigned      Status = "assigned"
	StatusDriverArrived Status = "driver_arrived"
	StatusInProgress    Status = "in_progress"
	StatusCompleted     Status = "completed"
	StatusCancelled     Status = "cancelled"
	StatusReleased      Status = "released"
)

var ErrInvalidTransition = errors.New("invalid assignment state transition")

type Assignment struct {
	ID        ID
	RideID    uuid.UUID
	DriverID  uuid.UUID
	Status    Status
	StartedAt *time.Time
	EndedAt   *time.Time
}

func New(id ID, rideID, driverID uuid.UUID) Assignment {
	return Assignment{ID: id, RideID: rideID, DriverID: driverID, Status: StatusAssigned}
}

func (a *Assignment) DriverArrived() error {
	if a.Status != StatusAssigned { return ErrInvalidTransition }
	a.Status = StatusDriverArrived
	return nil
}

func (a *Assignment) Start(at time.Time) error {
	if a.Status != StatusDriverArrived { return ErrInvalidTransition }
	a.Status = StatusInProgress
	a.StartedAt = &at
	return nil
}

func (a *Assignment) Complete(at time.Time) error {
	if a.Status != StatusInProgress { return ErrInvalidTransition }
	a.Status = StatusCompleted
	a.EndedAt = &at
	return nil
}

func (a *Assignment) Cancel() error {
	if a.Status != StatusAssigned && a.Status != StatusDriverArrived { return ErrInvalidTransition }
	a.Status = StatusCancelled
	return nil
}

func (a *Assignment) Release() error {
	if a.Status != StatusAssigned { return ErrInvalidTransition }
	a.Status = StatusReleased
	return nil
}

func (a Assignment) IsTerminal() bool {
	return a.Status == StatusCompleted || a.Status == StatusCancelled || a.Status == StatusReleased
}
