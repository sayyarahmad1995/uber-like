package ride

import (
	"testing"

	"github.com/google/uuid"
)

func newTestRide(t *testing.T) Ride {
	t.Helper()
	r, err := New(uuid.New(), uuid.New(), Coordinate{Latitude: 1, Longitude: 2}, Coordinate{Latitude: 3, Longitude: 4})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRideLifecycle(t *testing.T) {
	r := newTestRide(t)

	steps := []struct {
		name string
		fn   func() error
		want Status
	}{
		{"start bidding", r.StartBidding, StatusBidding},
		{"reserve", r.Reserve, StatusReserved},
		{"assign", r.Assign, StatusAssigned},
		{"driver arrived", r.DriverArrived, StatusDriverArrived},
		{"start", r.Start, StatusInProgress},
		{"complete", r.Complete, StatusCompleted},
	}

	for _, step := range steps {
		if err := step.fn(); err != nil {
			t.Fatalf("%s: unexpected error: %v", step.name, err)
		}
		if r.Status != step.want {
			t.Fatalf("%s: got %q, want %q", step.name, r.Status, step.want)
		}
	}

	if !r.IsTerminal() {
		t.Fatal("completed ride must be terminal")
	}
}

func TestRideRejectsInvalidTransition(t *testing.T) {
	r := newTestRide(t)

	if err := r.Complete(); err != ErrInvalidTransition {
		t.Fatalf("got %v, want %v", err, ErrInvalidTransition)
	}
}

func TestRideCanCancelBeforeTripStarts(t *testing.T) {
	r := newTestRide(t)

	if err := r.StartBidding(); err != nil {
		t.Fatal(err)
	}
	if err := r.Cancel(); err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusCancelled {
		t.Fatalf("got %q, want %q", r.Status, StatusCancelled)
	}
}

func TestRideCannotCancelAfterCompletion(t *testing.T) {
	r := newTestRide(t)
	_ = r.StartBidding()
	_ = r.Reserve()
	_ = r.Assign()
	_ = r.DriverArrived()
	_ = r.Start()
	_ = r.Complete()

	if err := r.Cancel(); err != ErrInvalidTransition {
		t.Fatalf("got %v, want %v", err, ErrInvalidTransition)
	}
}
