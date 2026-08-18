package user

import "github.com/google/uuid"

type ID = uuid.UUID

type Status string

const (
	StatusActive      Status = "active"
	StatusSuspended   Status = "suspended"
	StatusDeactivated Status = "deactivated"
)

type User struct {
	ID     ID
	Status Status
}

func New(id ID) User {
	return User{ID: id, Status: StatusActive}
}

func (u User) IsActive() bool { return u.Status == StatusActive }
