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
	"time"

	"eagle-bank-api/internal/auth"
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

func TestCreateUserThenAuthenticate_Integration(t *testing.T) {
	db, err := sql.Open("pgx", integrationDSN(t))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	repo := postgres.NewRepository(db)
	userHandler := NewUserHandler(repo)
	tokenService := auth.NewTokenService("integration-secret", time.Hour)
	authHandler := NewAuthHandler(repo, tokenService)
	email := "integration-signup-login@example.com"
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM users WHERE email = $1`, email)
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(`{
		"name":"Integration User",
		"address":{"line1":"1 High St","town":"London","county":"Greater London","postcode":"SW1A 1AA"},
		"phoneNumber":"+441234567890",
		"email":"integration-signup-login@example.com",
		"password":"correct horse battery staple"
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	userHandler.ServeHTTP(createRecorder, createReq)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRecorder.Code, http.StatusCreated, createRecorder.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}

	authReq := httptest.NewRequest(http.MethodPost, "/v1/auth/token", strings.NewReader(`{
		"email":"integration-signup-login@example.com",
		"password":"correct horse battery staple"
	}`))
	authReq.Header.Set("Content-Type", "application/json")
	authRecorder := httptest.NewRecorder()
	authHandler.ServeHTTP(authRecorder, authReq)
	if authRecorder.Code != http.StatusOK {
		t.Fatalf("auth status = %d, want %d; body=%s", authRecorder.Code, http.StatusOK, authRecorder.Body.String())
	}

	var authResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(authRecorder.Body.Bytes(), &authResp); err != nil {
		t.Fatalf("unmarshal auth response: %v", err)
	}
	claims, err := tokenService.Verify(authResp.Token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.Subject != created.ID {
		t.Fatalf("token subject = %q, want %q", claims.Subject, created.ID)
	}
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
				"phoneNumber":"+441234567890",
				"password":"correct horse battery staple"
			}`,
			wantMessage: "email is required",
		},
		{
			name: "missing address line1",
			body: `{
				"name":"Integration User",
				"address":{"town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890",
				"email":"integration-user@example.com",
				"password":"correct horse battery staple"
			}`,
			wantMessage: "address.line1 is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(tc.body))
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
