package postgres

import (
	"context"
	"database/sql"
)

// DB wraps database/sql for PostgreSQL access. The PostgreSQL driver is wired by
// the application composition root; this package deliberately does not own the
// driver dependency.
type DB struct {
	db *sql.DB
}

func New(db *sql.DB) *DB {
	return &DB{db: db}
}

func (d *DB) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

func (d *DB) Begin(ctx context.Context) (*Tx, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}
