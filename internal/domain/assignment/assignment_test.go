package assignment

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAssignmentLifecycle(t *testing.T) {
	a := New(uuid.New(), uuid.New(), uuid.New())
	if err := a.DriverArrived(); err != nil { t.Fatal(err) }
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	if err := a.Start(start); err != nil { t.Fatal(err) }
	if err := a.Complete(end); err != nil { t.Fatal(err) }
	if !a.IsTerminal() { t.Fatal("completed assignment must be terminal") }
	if a.StartedAt == nil || a.EndedAt == nil { t.Fatal("timestamps must be recorded") }
}

func TestAssignmentCannotCompleteBeforeStart(t *testing.T) {
	a := New(uuid.New(), uuid.New(), uuid.New())
	if err := a.Complete(time.Now()); err != ErrInvalidTransition { t.Fatalf("got %v, want %v", err, ErrInvalidTransition) }
}
