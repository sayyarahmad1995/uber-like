package reservation

import (
	"testing"

	"github.com/google/uuid"
)

func TestReservationLifecycle(t *testing.T) {
	r := New(uuid.New(), uuid.New(), uuid.New(), uuid.New(), nil)
	if err := r.Confirm(); err != nil { t.Fatal(err) }
	if err := r.Cancel(); err != nil { t.Fatal(err) }
	if r.Status != StatusCancelled { t.Fatalf("got %q, want %q", r.Status, StatusCancelled) }
}

func TestReservationRejectsExpiredConfirmation(t *testing.T) {
	r := New(uuid.New(), uuid.New(), uuid.New(), uuid.New(), nil)
	if err := r.Expire(); err != nil { t.Fatal(err) }
	if err := r.Confirm(); err != ErrInvalidTransition { t.Fatalf("got %v, want %v", err, ErrInvalidTransition) }
}
