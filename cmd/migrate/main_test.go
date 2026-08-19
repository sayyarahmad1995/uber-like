package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMigrationsSortsAndParses(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"002_users.sql", "001_extensions_and_types.sql", "003_driver_profiles.sql", "README.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- test\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	migrations, err := loadMigrations(dir)
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if len(migrations) != 3 {
		t.Fatalf("got %d migrations, want 3", len(migrations))
	}

	want := []struct {
		version int64
		name    string
	}{
		{1, "extensions_and_types"},
		{2, "users"},
		{3, "driver_profiles"},
	}
	for i, m := range migrations {
		if m.version != want[i].version || m.name != want[i].name {
			t.Errorf("migration[%d] = {%d, %q}, want {%d, %q}", i, m.version, m.name, want[i].version, want[i].name)
		}
	}
}

func TestLoadMigrationsRejectsSequenceGap(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"001_first.sql", "003_third.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- test\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	_, err := loadMigrations(dir)
	if err == nil {
		t.Fatal("loadMigrations() error = nil, want sequence gap error")
	}
	if !strings.Contains(err.Error(), "migration sequence has gap") {
		t.Fatalf("loadMigrations() error = %q, want sequence gap error", err)
	}
}

func TestChecksumIsStableAndSensitiveToContent(t *testing.T) {
	first := checksum([]byte("CREATE TABLE users (id uuid);\n"))
	second := checksum([]byte("CREATE TABLE users (id uuid);\n"))
	changed := checksum([]byte("CREATE TABLE users (id bigint);\n"))

	if first != second {
		t.Fatal("checksum is not stable")
	}
	if first == changed {
		t.Fatal("checksum did not change when migration content changed")
	}
	if len(first) != 64 {
		t.Fatalf("checksum length = %d, want 64", len(first))
	}
}

func TestValidateAppliedMigrationsRejectsUnknownVersion(t *testing.T) {
	migrations := []migration{{version: 1, name: "first"}}
	applied := map[int64]appliedMigration{
		1: {version: 1, name: "first", checksum: "checksum"},
		2: {version: 2, name: "unknown", checksum: "checksum"},
	}

	err := validateAppliedMigrations(migrations, applied)
	if err == nil {
		t.Fatal("validateAppliedMigrations() error = nil, want unknown migration error")
	}
	if !strings.Contains(err.Error(), "unknown migration 2") {
		t.Fatalf("validateAppliedMigrations() error = %q, want unknown migration error", err)
	}
}
