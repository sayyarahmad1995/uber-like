package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/google/uuid"
	"github.com/sayyarahmad1995/uber-like/internal/application"
)

func TestUserRepositoryIntegration(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	subject := "integration-test-" + uuid.NewString()
	id := uuid.New()
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, oidc_subject, status) VALUES ($1, $2, 'active')`, id, subject)
	if err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)

	r := userRepository{q: db}

	got, err := r.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id || !got.IsActive() {
		t.Fatalf("Get() = %#v, want id %s and active status", got, id)
	}

	got, err = r.GetByOIDCSubject(ctx, subject)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id || !got.IsActive() {
		t.Fatalf("GetByOIDCSubject() = %#v, want id %s and active status", got, id)
	}

	if _, err := r.Get(ctx, uuid.New()); err != application.ErrNotFound {
		t.Fatalf("Get(unknown) error = %v, want %v", err, application.ErrNotFound)
	}

	if _, err := r.GetByOIDCSubject(ctx, "missing-"+uuid.NewString()); err != application.ErrNotFound {
		t.Fatalf("GetByOIDCSubject(unknown) error = %v, want %v", err, application.ErrNotFound)
	}
}
