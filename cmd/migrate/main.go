package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const lockKey int64 = 74627381942

var migrationFile = regexp.MustCompile(`^(\d+)_.*\.sql$`)

type migration struct {
	version int64
	name    string
	path    string
}

type appliedMigration struct {
	version  int64
	name     string
	checksum string
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fatal("DATABASE_URL is required")
	}

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "/app/migrations"
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fatal("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fatal("ping database: %v", err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		fatal("acquire migration connection: %v", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		fatal("acquire migration lock: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockKey)
	}()

	if _, err := conn.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    checksum TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`); err != nil {
		fatal("create schema_migrations table: %v", err)
	}

	migrations, err := loadMigrations(migrationsDir)
	if err != nil {
		fatal("load migrations: %v", err)
	}

	applied, err := loadAppliedMigrations(ctx, conn)
	if err != nil {
		fatal("load applied migrations: %v", err)
	}

	validateAppliedMigrations(migrations, applied)

	for _, m := range migrations {
		contents, err := os.ReadFile(m.path)
		if err != nil {
			fatal("read migration %s: %v", m.name, err)
		}
		checksum := checksum(contents)

		appliedMigration, ok := applied[m.version]
		if ok {
			if appliedMigration.name != m.name {
				fatal("migration %d name changed: database has %q, filesystem has %q", m.version, appliedMigration.name, m.name)
			}
			if appliedMigration.checksum != checksum {
				fatal("migration %d (%s) checksum changed", m.version, m.name)
			}
			continue
		}

		if err := applyMigration(ctx, conn, m, contents, checksum); err != nil {
			fatal("apply migration %d (%s): %v", m.version, m.name, err)
		}
		fmt.Printf("applied migration %03d_%s\n", m.version, m.name)
	}

	fmt.Printf("database schema is up to date (%d migrations)\n", len(migrations))
}

func loadMigrations(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := migrationFile.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		name := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), match[1]+"_"), ".sql")
		migrations = append(migrations, migration{version: version, name: name, path: filepath.Join(dir, entry.Name())})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	for i, m := range migrations {
		expected := int64(i + 1)
		if m.version != expected {
			return nil, fmt.Errorf("migration sequence has gap: expected %d, found %d", expected, m.version)
		}
	}
	return migrations, nil
}

func loadAppliedMigrations(ctx context.Context, conn *pgxpool.Conn) (map[int64]appliedMigration, error) {
	rows, err := conn.Query(ctx, "SELECT version, name, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int64]appliedMigration)
	for rows.Next() {
		var m appliedMigration
		if err := rows.Scan(&m.version, &m.name, &m.checksum); err != nil {
			return nil, err
		}
		applied[m.version] = m
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return applied, nil
}

func validateAppliedMigrations(migrations []migration, applied map[int64]appliedMigration) {
	known := make(map[int64]migration, len(migrations))
	for _, m := range migrations {
		known[m.version] = m
	}
	for version, m := range applied {
		if _, ok := known[version]; !ok {
			fatal("database contains unknown migration %d (%s)", version, m.name)
		}
	}
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, m migration, contents []byte, checksum string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, string(contents)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)", m.version, m.name, checksum); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func checksum(contents []byte) string {
	h := sha256.Sum256(contents)
	return hex.EncodeToString(h[:])
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "migration error: "+format+"\n", args...)
	os.Exit(1)
}
