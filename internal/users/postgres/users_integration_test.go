//go:build integration

package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"eagle-bank-api/internal/users"
)

func integrationDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("integration tests: set TEST_DATABASE_URL (e.g. postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable)")
	}
	return dsn
}

// randomUserID returns a value matching DB/OpenAPI: ^usr-[A-Za-z0-9]+$
// (only one hyphen, immediately after "usr"; the rest must be alphanumeric—no "usr-itest-..." second hyphen).
func randomUserID(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return "usr-itest" + hex.EncodeToString(b[:])
}

func TestRepository_Create_integration(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("pgx", integrationDSN(t))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	repo := NewRepository(db)
	id := randomUserID(t)
	email := id + "@example.com" // unique per run

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})

	u, err := repo.Create(ctx, users.CreateParams{
		ID:           id,
		Name:         "Integration User",
		AddressLine1: "10 Test St",
		Town:         "London",
		County:       "Greater London",
		Postcode:     "EC1A 1BB",
		PhoneNumber:  "+441234567890",
		Email:        email,
		PasswordHash: "test-password-hash",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID != id || u.Email != email {
		t.Fatalf("Create returned %+v", u)
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Integration User" {
		t.Fatalf("GetByID name = %q", got.Name)
	}
}
