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
)

func TestCreateUser_UnauthenticatedValidRequest_ReturnsCreated(t *testing.T) {
	handler := NewUserHandler(memory.NewRepository())

	body := `{
		"name":"Alice",
		"address":{
			"line1":"1 High St",
			"town":"London",
			"county":"Greater London",
			"postcode":"SW1A 1AA"
		},
		"phoneNumber":"+441234567890",
		"email":"alice@example.com",
		"password":"correct horse battery staple"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Address struct {
			Line1    string  `json:"line1"`
			Line2    *string `json:"line2"`
			Line3    *string `json:"line3"`
			Town     string  `json:"town"`
			County   string  `json:"county"`
			Postcode string  `json:"postcode"`
		} `json:"address"`
		PhoneNumber      string `json:"phoneNumber"`
		Email            string `json:"email"`
		CreatedTimestamp string `json:"createdTimestamp"`
		UpdatedTimestamp string `json:"updatedTimestamp"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rr.Body.String())
	}

	if !strings.HasPrefix(resp.ID, "usr-") {
		t.Fatalf("id = %q, want prefix usr-", resp.ID)
	}
	if resp.Name != "Alice" {
		t.Fatalf("name = %q, want Alice", resp.Name)
	}
	if resp.Address.Line1 != "1 High St" || resp.Address.Town != "London" || resp.Address.County != "Greater London" || resp.Address.Postcode != "SW1A 1AA" {
		t.Fatalf("address = %+v", resp.Address)
	}
	if resp.PhoneNumber != "+441234567890" {
		t.Fatalf("phoneNumber = %q", resp.PhoneNumber)
	}
	if resp.Email != "alice@example.com" {
		t.Fatalf("email = %q", resp.Email)
	}
	if strings.Contains(rr.Body.String(), "password") {
		t.Fatalf("response must not include password material: %s", rr.Body.String())
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.CreatedTimestamp); err != nil {
		t.Fatalf("createdTimestamp format: %v (%q)", err, resp.CreatedTimestamp)
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.UpdatedTimestamp); err != nil {
		t.Fatalf("updatedTimestamp format: %v (%q)", err, resp.UpdatedTimestamp)
	}
}

func TestCreateUser_UnauthenticatedMissingRequiredData_ReturnsBadRequest(t *testing.T) {
	handler := NewUserHandler(memory.NewRepository())

	tests := []struct {
		name        string
		body        string
		wantMessage string
	}{
		{
			name: "missing email",
			body: `{
				"name":"Alice",
				"address":{"line1":"1 High St","town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890",
				"password":"correct horse battery staple"
			}`,
			wantMessage: "email is required",
		},
		{
			name: "missing name",
			body: `{
				"address":{"line1":"1 High St","town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890",
				"email":"alice@example.com",
				"password":"correct horse battery staple"
			}`,
			wantMessage: "name is required",
		},
		{
			name: "missing address line1",
			body: `{
				"name":"Alice",
				"address":{"town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890",
				"email":"alice@example.com",
				"password":"correct horse battery staple"
			}`,
			wantMessage: "address.line1 is required",
		},
		{
			name: "missing address town",
			body: `{
				"name":"Alice",
				"address":{"line1":"1 High St","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890",
				"email":"alice@example.com",
				"password":"correct horse battery staple"
			}`,
			wantMessage: "address.town is required",
		},
		{
			name: "missing phone number",
			body: `{
				"name":"Alice",
				"address":{"line1":"1 High St","town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"email":"alice@example.com",
				"password":"correct horse battery staple"
			}`,
			wantMessage: "phoneNumber is required",
		},
		{
			name: "missing password",
			body: `{
				"name":"Alice",
				"address":{"line1":"1 High St","town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890",
				"email":"alice@example.com"
			}`,
			wantMessage: "password is required",
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
				t.Fatalf("unmarshal response: %v", err)
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

func TestCreateUser_DuplicateEmail_ReturnsConflict(t *testing.T) {
	handler := NewUserHandler(memory.NewRepository())
	body := `{
		"name":"Alice",
		"address":{
			"line1":"1 High St",
			"town":"London",
			"county":"Greater London",
			"postcode":"SW1A 1AA"
		},
		"phoneNumber":"+441234567890",
		"email":"alice@example.com",
		"password":"correct horse battery staple"
	}`

	first := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(body))
	first.Header.Set("Content-Type", "application/json")
	firstRecorder := httptest.NewRecorder()
	handler.ServeHTTP(firstRecorder, first)
	if firstRecorder.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d; body=%s", firstRecorder.Code, http.StatusCreated, firstRecorder.Body.String())
	}

	second := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(body))
	second.Header.Set("Content-Type", "application/json")
	secondRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondRecorder, second)
	if secondRecorder.Code != http.StatusConflict {
		t.Fatalf("second status = %d, want %d; body=%s", secondRecorder.Code, http.StatusConflict, secondRecorder.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["message"] != "user already exists" {
		t.Fatalf("message = %q, want user already exists", resp["message"])
	}
}

func TestFetchUser_AuthenticatedUserRequestsOwnDetails_ReturnsUser(t *testing.T) {
	repo := memory.NewRepository()
	created, err := repo.Create(context.Background(), users.CreateParams{
		ID:           "usr-fetchself1",
		Name:         "Alice",
		AddressLine1: "1 High St",
		Town:         "London",
		County:       "Greater London",
		Postcode:     "SW1A 1AA",
		PhoneNumber:  "+441234567890",
		Email:        "alice@example.com",
		PasswordHash: "not-used-in-this-test",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	tokenService := auth.NewTokenService("test-secret", time.Hour)
	token, err := tokenService.Issue(created.ID)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	handler := NewUserHandler(repo, tokenService)
	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Address struct {
			Line1    string `json:"line1"`
			Town     string `json:"town"`
			County   string `json:"county"`
			Postcode string `json:"postcode"`
		} `json:"address"`
		PhoneNumber      string `json:"phoneNumber"`
		Email            string `json:"email"`
		CreatedTimestamp string `json:"createdTimestamp"`
		UpdatedTimestamp string `json:"updatedTimestamp"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rr.Body.String())
	}
	if resp.ID != created.ID {
		t.Fatalf("id = %q, want %q", resp.ID, created.ID)
	}
	if resp.Name != "Alice" || resp.Email != "alice@example.com" || resp.PhoneNumber != "+441234567890" {
		t.Fatalf("unexpected user response: %+v", resp)
	}
	if resp.Address.Line1 != "1 High St" || resp.Address.Town != "London" || resp.Address.County != "Greater London" || resp.Address.Postcode != "SW1A 1AA" {
		t.Fatalf("unexpected address response: %+v", resp.Address)
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.CreatedTimestamp); err != nil {
		t.Fatalf("createdTimestamp format: %v (%q)", err, resp.CreatedTimestamp)
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.UpdatedTimestamp); err != nil {
		t.Fatalf("updatedTimestamp format: %v (%q)", err, resp.UpdatedTimestamp)
	}
}
