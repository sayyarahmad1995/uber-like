package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/domain/assignment"
)

type assignmentRepository struct{ q querier }

func (r assignmentRepository) Get(ctx context.Context, id assignment.ID) (assignment.Assignment, error) {
	var out assignment.Assignment
	var started, ended sql.NullTime
	err := r.q.QueryRowContext(ctx, `SELECT id, ride_id, driver_id, status, started_at, ended_at FROM assignments WHERE id = $1`, id).
		Scan(&out.ID, &out.RideID, &out.DriverID, &out.Status, &started, &ended)
	if err != nil { return assignment.Assignment{}, notFound(err) }
	if started.Valid { out.StartedAt = timePtr(started.Time) }
	if ended.Valid { out.EndedAt = timePtr(ended.Time) }
	return out, nil
}

func (r assignmentRepository) Create(ctx context.Context, v assignment.Assignment) error {
	_, err := r.q.ExecContext(ctx, `INSERT INTO assignments (id, ride_id, driver_id, status, started_at, ended_at) VALUES ($1,$2,$3,$4,$5,$6)`, v.ID, v.RideID, v.DriverID, v.Status, v.StartedAt, v.EndedAt)
	return err
}

func (r assignmentRepository) Save(ctx context.Context, v assignment.Assignment) error {
	res, err := r.q.ExecContext(ctx, `UPDATE assignments SET status = $2, started_at = $3, ended_at = $4 WHERE id = $1`, v.ID, v.Status, v.StartedAt, v.EndedAt)
	if err != nil { return err }
	n, err := res.RowsAffected()
	if err != nil { return err }
	if n != 1 { return sql.ErrNoRows }
	return nil
}

func (r assignmentRepository) HasActiveForDriver(ctx context.Context, driverID uuid.UUID) (bool, error) {
	var exists bool
	err := r.q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM assignments WHERE driver_id = $1 AND status IN ('assigned','driver_arrived','in_progress'))`, driverID).Scan(&exists)
	return exists, err
}

var _ time.Time
