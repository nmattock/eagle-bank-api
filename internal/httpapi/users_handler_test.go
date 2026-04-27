package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"eagle-bank-api/internal/users/memory"
)

const placeholderBearerToken = "Bearer present-but-not-validated"

func TestCreateUser_AuthenticatedValidRequest_ReturnsCreated(t *testing.T) {
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
		"email":"alice@example.com"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(body))
	// Auth is currently only a bearer-token presence check; token validation will be added later.
	req.Header.Set("Authorization", placeholderBearerToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		Address          struct {
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
	if _, err := time.Parse(time.RFC3339Nano, resp.CreatedTimestamp); err != nil {
		t.Fatalf("createdTimestamp format: %v (%q)", err, resp.CreatedTimestamp)
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.UpdatedTimestamp); err != nil {
		t.Fatalf("updatedTimestamp format: %v (%q)", err, resp.UpdatedTimestamp)
	}
}

func TestCreateUser_AuthenticatedMissingRequiredData_ReturnsBadRequest(t *testing.T) {
	handler := NewUserHandler(memory.NewRepository())

	tests := []struct {
		name         string
		body         string
		wantMessage  string
	}{
		{
			name: "missing email",
			body: `{
				"name":"Alice",
				"address":{"line1":"1 High St","town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890"
			}`,
			wantMessage: "email is required",
		},
		{
			name: "missing name",
			body: `{
				"address":{"line1":"1 High St","town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890",
				"email":"alice@example.com"
			}`,
			wantMessage: "name is required",
		},
		{
			name: "missing address line1",
			body: `{
				"name":"Alice",
				"address":{"town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890",
				"email":"alice@example.com"
			}`,
			wantMessage: "address.line1 is required",
		},
		{
			name: "missing address town",
			body: `{
				"name":"Alice",
				"address":{"line1":"1 High St","county":"Greater London","postcode":"SW1A 1AA"},
				"phoneNumber":"+441234567890",
				"email":"alice@example.com"
			}`,
			wantMessage: "address.town is required",
		},
		{
			name: "missing phone number",
			body: `{
				"name":"Alice",
				"address":{"line1":"1 High St","town":"London","county":"Greater London","postcode":"SW1A 1AA"},
				"email":"alice@example.com"
			}`,
			wantMessage: "phoneNumber is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(tc.body))
			req.Header.Set("Authorization", placeholderBearerToken)
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

