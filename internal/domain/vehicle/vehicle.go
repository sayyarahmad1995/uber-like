package vehicle

import "github.com/google/uuid"

type ID = uuid.UUID

type Status string

const (
	StatusActive    Status = "active"
	StatusInactive  Status = "inactive"
	StatusSuspended Status = "suspended"
)

type Vehicle struct {
	ID                 ID
	DriverID           uuid.UUID
	Type               string
	RegistrationNumber string
	Status             Status
}

func (v Vehicle) IsActive() bool { return v.Status == StatusActive }
