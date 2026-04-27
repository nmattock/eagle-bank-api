//go:build integration

package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"eagle-bank-api/internal/users/postgres"
)

func integrationDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("integration tests: set TEST_DATABASE_URL (e.g. postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable)")
	}
	return dsn
}

func TestCreateUser_MissingRequiredFields_Integration(t *testing.T) {
	db, err := sql.Open("pgx", integrationDSN(t))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	handler := NewUserHandler(postgres.NewRepository(db))

	tests := []struct {
		name        string
		body        string
		wantMessage string
	}{
		{
			name: "missing email",
			body: `{
				"name":"Integration User",
				"address":{"line1":"1 High St","town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890"
			}`,
			wantMessage: "email is required",
		},
		{
			name: "missing address line1",
			body: `{
				"name":"Integration User",
				"address":{"town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890",
				"email":"integration-user@example.com"
			}`,
			wantMessage: "address.line1 is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(tc.body))
			// Auth is currently only a bearer-token presence check; token validation will be added later.
			req.Header.Set("Authorization", placeholderBearerToken)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
			}

			var resp map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal response: %v body=%s", err, rr.Body.String())
			}
			if strings.TrimSpace(resp["message"]) == "" {
				t.Fatalf("expected non-empty error message, got %q", resp["message"])
			}
			if resp["message"] != tc.wantMessage {
				t.Fatalf("message = %q, want %q", resp["message"], tc.wantMessage)
			}
		})
	}
}

