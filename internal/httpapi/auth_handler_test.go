package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"eagle-bank-api/internal/auth"
	"eagle-bank-api/internal/users"
	"eagle-bank-api/internal/users/memory"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthenticateUser_ValidCredentials_ReturnsJWT(t *testing.T) {
	repo := memory.NewRepository()
	password := "correct horse battery staple"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	created, err := repo.Create(context.Background(), users.CreateParams{
		ID:           "usr-authsuccess1",
		Name:         "Alice",
		AddressLine1: "1 High St",
		Town:         "London",
		County:       "Greater London",
		Postcode:     "SW1A 1AA",
		PhoneNumber:  "+441234567890",
		Email:        "alice@example.com",
		PasswordHash: string(hash),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	issuer := auth.NewTokenService("test-secret", time.Hour)
	handler := NewAuthHandler(repo, issuer)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/token", strings.NewReader(`{
		"email":"alice@example.com",
		"password":"correct horse battery staple"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rr.Body.String())
	}
	if strings.Count(resp.Token, ".") != 2 {
		t.Fatalf("token = %q, want JWT compact format", resp.Token)
	}
	claims, err := issuer.Verify(resp.Token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.Subject != created.ID {
		t.Fatalf("subject = %q, want %q", claims.Subject, created.ID)
	}
}

func TestAuthenticateUser_InvalidCredentials_ReturnsUnauthorized(t *testing.T) {
	repo := memory.NewRepository()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	_, err = repo.Create(context.Background(), users.CreateParams{
		ID:           "usr-authfailure1",
		Name:         "Alice",
		AddressLine1: "1 High St",
		Town:         "London",
		County:       "Greater London",
		Postcode:     "SW1A 1AA",
		PhoneNumber:  "+441234567890",
		Email:        "alice@example.com",
		PasswordHash: string(hash),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	handler := NewAuthHandler(repo, auth.NewTokenService("test-secret", time.Hour))
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/token", strings.NewReader(`{
		"email":"alice@example.com",
		"password":"wrong password"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestAuthenticateUser_UnknownEmail_ReturnsUnauthorized(t *testing.T) {
	handler := NewAuthHandler(memory.NewRepository(), auth.NewTokenService("test-secret", time.Hour))
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/token", strings.NewReader(`{
		"email":"unknown@example.com",
		"password":"correct horse battery staple"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["message"] != "invalid credentials" {
		t.Fatalf("message = %q, want invalid credentials", resp["message"])
	}
}

func TestAuthenticateUser_MissingRequiredData_ReturnsBadRequest(t *testing.T) {
	handler := NewAuthHandler(memory.NewRepository(), auth.NewTokenService("test-secret", time.Hour))

	tests := []struct {
		name        string
		body        string
		wantMessage string
	}{
		{
			name:        "missing password",
			body:        `{"email":"alice@example.com"}`,
			wantMessage: "password is required",
		},
		{
			name:        "missing email",
			body:        `{"password":"correct horse battery staple"}`,
			wantMessage: "email is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/auth/token", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
			}
			var resp map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if resp["message"] != tc.wantMessage {
				t.Fatalf("message = %q, want %q", resp["message"], tc.wantMessage)
			}
		})
	}
}
