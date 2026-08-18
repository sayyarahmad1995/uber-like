package bid

import (
	"testing"

	"github.com/google/uuid"
)

func TestBidLifecycle(t *testing.T) {
	b, err := New(uuid.New(), uuid.New(), uuid.New(), 1500, "PKR")
	if err != nil { t.Fatal(err) }
	if err := b.Activate(); err != nil { t.Fatal(err) }
	if err := b.Select(); err != nil { t.Fatal(err) }
	if !b.IsTerminal() { t.Fatal("selected bid must be terminal") }
}

func TestBidRejectsTerminalTransition(t *testing.T) {
	b, _ := New(uuid.New(), uuid.New(), uuid.New(), 1500, "PKR")
	if err := b.Withdraw(); err != nil { t.Fatal(err) }
	if err := b.Select(); err != ErrInvalidTransition { t.Fatalf("got %v, want %v", err, ErrInvalidTransition) }
}
