package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
)

// DB wraps database/sql for PostgreSQL access. The PostgreSQL driver is wired by
// the application composition root; this package deliberately does not own the
// driver dependency.
type DB struct {
	db *sql.DB
}

func New(db *sql.DB) *DB { return &DB{db: db} }

func (d *DB) Ping(ctx context.Context) error { return d.db.PingContext(ctx) }
func (d *DB) Close() error { return d.db.Close() }

func (d *DB) Begin(ctx context.Context) (application.Transaction, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx}, nil
}

func (d *DB) Users() application.UserRepository { return userRepository{q: d.db} }
func (d *DB) Rides() application.RideRepository { return rideRepository{q: d.db} }
func (d *DB) Bids() application.BidRepository { return bidRepository{q: d.db} }
func (d *DB) Reservations() application.ReservationRepository { return reservationRepository{q: d.db} }
func (d *DB) Assignments() application.AssignmentRepository { return assignmentRepository{q: d.db} }
func (d *DB) Drivers() application.DriverRepository { return driverRepository{q: d.db} }

func (d *DB) IsEligible(ctx context.Context, driverID uuid.UUID) (bool, error) {
	var active bool
	err := d.db.QueryRowContext(ctx, `SELECT status = 'active' FROM driver_profiles WHERE id = $1`, driverID).Scan(&active)
	if err != nil { return false, notFound(err) }
	return active, nil
}

var _ application.UserRepository = (*DB)(nil)
var _ application.TransactionManager = (*DB)(nil)
var _ application.EligibilityChecker = (*DB)(nil)
