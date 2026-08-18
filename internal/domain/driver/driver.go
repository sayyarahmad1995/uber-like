package driver

import "github.com/google/uuid"

type ID = uuid.UUID

type Status string

const (
	StatusActive      Status = "active"
	StatusSuspended   Status = "suspended"
	StatusDeactivated Status = "deactivated"
)

type Eligibility struct {
	Eligible bool
	Reasons  []string
}

type Driver struct {
	ID     ID
	UserID uuid.UUID
	Status Status
}

func (d Driver) IsActive() bool { return d.Status == StatusActive }
