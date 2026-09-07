// Package pgtest creates disposable databases only when explicitly configured.
package pgtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"chakuchuri/backend/internal/platform/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func New(t *testing.T) *sql.DB {
	t.Helper()
	raw := os.Getenv("TEST_POSTGRES_ADMIN_URL")
	if raw == "" {
		t.Skip("TEST_POSTGRES_ADMIN_URL is not set; PostgreSQL integration test not run")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("invalid TEST_POSTGRES_ADMIN_URL")
	}
	admin, err := sql.Open("pgx", raw)
	if err != nil {
		t.Fatal("open test database admin connection")
	}
	t.Cleanup(func() { _ = admin.Close() })
	var entropy [10]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		t.Fatal(err)
	}
	name := "cc_test_" + hex.EncodeToString(entropy[:])
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
		t.Fatalf("create disposable test database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		// name is generated locally, and this cleanup is registered only after creation.
		if _, err := admin.ExecContext(ctx, `DROP DATABASE "`+name+`" WITH (FORCE)`); err != nil {
			t.Errorf("cleanup disposable database %s: %v", name, err)
		}
	})
	u.Path = "/" + name
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal("open disposable database")
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(2)
	t.Cleanup(func() { _ = db.Close() })
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
	if _, err := migrations.Apply(ctx, db, dir); err != nil {
		t.Fatalf("apply test migrations: %v", err)
	}
	return db
}
