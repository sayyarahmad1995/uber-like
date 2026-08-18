package bid

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ID = uuid.UUID

type Status string

const (
	StatusSubmitted Status = "submitted"
	StatusActive    Status = "active"
	StatusWithdrawn Status = "withdrawn"
	StatusRejected  Status = "rejected"
	StatusSelected  Status = "selected"
	StatusExpired   Status = "expired"
)

var ErrInvalidTransition = errors.New("invalid bid state transition")

type Bid struct {
	ID        ID
	RideID    uuid.UUID
	DriverID  uuid.UUID
	AmountMinor int64
	Currency  string
	Status    Status
	ExpiresAt *time.Time
}

func New(id ID, rideID, driverID uuid.UUID, amountMinor int64, currency string) (Bid, error) {
	if amountMinor < 0 || currency == "" {
		return Bid{}, errors.New("invalid bid amount or currency")
	}
	return Bid{ID: id, RideID: rideID, DriverID: driverID, AmountMinor: amountMinor, Currency: currency, Status: StatusSubmitted}, nil
}

func (b *Bid) Activate() error {
	if b.Status != StatusSubmitted { return ErrInvalidTransition }
	b.Status = StatusActive
	return nil
}

func (b *Bid) Withdraw() error {
	if b.Status != StatusSubmitted && b.Status != StatusActive { return ErrInvalidTransition }
	b.Status = StatusWithdrawn
	return nil
}

func (b *Bid) Reject() error {
	if b.Status != StatusSubmitted && b.Status != StatusActive { return ErrInvalidTransition }
	b.Status = StatusRejected
	return nil
}

func (b *Bid) Select() error {
	if b.Status != StatusSubmitted && b.Status != StatusActive { return ErrInvalidTransition }
	b.Status = StatusSelected
	return nil
}

func (b *Bid) Expire() error {
	if b.Status != StatusSubmitted && b.Status != StatusActive { return ErrInvalidTransition }
	b.Status = StatusExpired
	return nil
}

func (b Bid) IsTerminal() bool {
	return b.Status == StatusWithdrawn || b.Status == StatusRejected || b.Status == StatusSelected || b.Status == StatusExpired
}
