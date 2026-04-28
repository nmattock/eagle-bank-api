package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"eagle-bank-api/internal/accounts"
	"eagle-bank-api/internal/accounts/memory"
	"eagle-bank-api/internal/auth"
	"eagle-bank-api/internal/users"
	usersmemory "eagle-bank-api/internal/users/memory"
)

func TestCreateBankAccount_AuthenticatedValidRequest_ReturnsCreatedAccount(t *testing.T) {
	_, tokenService, token := testAuthenticatedUser(t, "usr-accountowner1")
	handler := NewAccountHandler(memory.NewRepository(), tokenService)
	req := httptest.NewRequest(http.MethodPost, "/v1/accounts", strings.NewReader(`{
		"name":"Personal Bank Account",
		"accountType":"personal"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp struct {
		AccountNumber    string  `json:"accountNumber"`
		SortCode         string  `json:"sortCode"`
		Name             string  `json:"name"`
		AccountType      string  `json:"accountType"`
		Balance          float64 `json:"balance"`
		Currency         string  `json:"currency"`
		CreatedTimestamp string  `json:"createdTimestamp"`
		UpdatedTimestamp string  `json:"updatedTimestamp"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rr.Body.String())
	}
	if !regexp.MustCompile(`^01\d{6}$`).MatchString(resp.AccountNumber) {
		t.Fatalf("accountNumber = %q, want format ^01\\d{6}$", resp.AccountNumber)
	}
	if resp.SortCode != "10-10-10" {
		t.Fatalf("sortCode = %q, want 10-10-10", resp.SortCode)
	}
	if resp.Name != "Personal Bank Account" {
		t.Fatalf("name = %q, want Personal Bank Account", resp.Name)
	}
	if resp.AccountType != "personal" {
		t.Fatalf("accountType = %q, want personal", resp.AccountType)
	}
	if resp.Balance != 0 {
		t.Fatalf("balance = %v, want 0", resp.Balance)
	}
	if resp.Currency != "GBP" {
		t.Fatalf("currency = %q, want GBP", resp.Currency)
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.CreatedTimestamp); err != nil {
		t.Fatalf("createdTimestamp format: %v (%q)", err, resp.CreatedTimestamp)
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.UpdatedTimestamp); err != nil {
		t.Fatalf("updatedTimestamp format: %v (%q)", err, resp.UpdatedTimestamp)
	}
}

func TestListBankAccounts_AuthenticatedUser_ReturnsTheirAccounts(t *testing.T) {
	owner, tokenService, token := testAuthenticatedUser(t, "usr-listowner1")
	otherUser, _, _ := testAuthenticatedUser(t, "usr-listother1")
	repo := memory.NewRepository()
	ownAccount1 := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000001",
		UserID:        owner.ID,
		Name:          "Everyday Account",
		AccountType:   "personal",
	})
	ownAccount2 := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000002",
		UserID:        owner.ID,
		Name:          "Savings Account",
		AccountType:   "personal",
	})
	createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000003",
		UserID:        otherUser.ID,
		Name:          "Other User Account",
		AccountType:   "personal",
	})

	handler := NewAccountHandler(repo, tokenService)
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var resp struct {
		Accounts []bankAccountResponse `json:"accounts"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rr.Body.String())
	}
	if len(resp.Accounts) != 2 {
		t.Fatalf("accounts len = %d, want 2; body=%s", len(resp.Accounts), rr.Body.String())
	}
	gotNumbers := []string{resp.Accounts[0].AccountNumber, resp.Accounts[1].AccountNumber}
	sort.Strings(gotNumbers)
	wantNumbers := []string{ownAccount1.AccountNumber, ownAccount2.AccountNumber}
	sort.Strings(wantNumbers)
	if gotNumbers[0] != wantNumbers[0] || gotNumbers[1] != wantNumbers[1] {
		t.Fatalf("account numbers = %v, want %v", gotNumbers, wantNumbers)
	}
}

func TestFetchBankAccount_AuthenticatedOwner_ReturnsAccount(t *testing.T) {
	owner, tokenService, token := testAuthenticatedUser(t, "usr-fetchaccount1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000011",
		UserID:        owner.ID,
		Name:          "Personal Bank Account",
		AccountType:   "personal",
	})

	handler := NewAccountHandler(repo, tokenService)
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/"+account.AccountNumber, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	assertBankAccountResponse(t, rr.Body.Bytes(), account)
}

func TestFetchBankAccount_AuthenticatedNonOwner_ReturnsForbidden(t *testing.T) {
	owner, _, _ := testAuthenticatedUser(t, "usr-accountowner2")
	_, tokenService, token := testAuthenticatedUser(t, "usr-accountintruder1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000021",
		UserID:        owner.ID,
		Name:          "Owner Account",
		AccountType:   "personal",
	})

	handler := NewAccountHandler(repo, tokenService)
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/"+account.AccountNumber, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())
}

func TestFetchBankAccount_AuthenticatedUserRequestsNonExistentAccount_ReturnsNotFound(t *testing.T) {
	_, tokenService, token := testAuthenticatedUser(t, "usr-missingaccount1")
	handler := NewAccountHandler(memory.NewRepository(), tokenService)
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/01999999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())
}

func testAuthenticatedUser(t *testing.T, id string) (*users.User, *auth.TokenService, string) {
	t.Helper()
	userRepo := usersmemory.NewRepository()
	createdUser, err := userRepo.Create(context.Background(), users.CreateParams{
		ID:           id,
		Name:         "Alice",
		AddressLine1: "1 High St",
		Town:         "London",
		County:       "Greater London",
		Postcode:     "SW1A 1AA",
		PhoneNumber:  "+441234567890",
		Email:        id + "@example.com",
		PasswordHash: "not-used-in-this-test",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	tokenService := auth.NewTokenService("test-secret", time.Hour)
	token, err := tokenService.Issue(createdUser.ID)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return createdUser, tokenService, token
}

func createTestAccount(t *testing.T, repo accounts.Repository, params accounts.CreateParams) *accounts.BankAccount {
	t.Helper()
	account, err := repo.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return account
}

func assertBankAccountResponse(t *testing.T, body []byte, want *accounts.BankAccount) {
	t.Helper()
	var resp bankAccountResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, string(body))
	}
	if resp.AccountNumber != want.AccountNumber {
		t.Fatalf("accountNumber = %q, want %q", resp.AccountNumber, want.AccountNumber)
	}
	if resp.SortCode != "10-10-10" || resp.Name != want.Name || resp.AccountType != "personal" || resp.Balance != 0 || resp.Currency != "GBP" {
		t.Fatalf("unexpected account response: %+v", resp)
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.CreatedTimestamp); err != nil {
		t.Fatalf("createdTimestamp format: %v (%q)", err, resp.CreatedTimestamp)
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.UpdatedTimestamp); err != nil {
		t.Fatalf("updatedTimestamp format: %v (%q)", err, resp.UpdatedTimestamp)
	}
}

func assertErrorMessage(t *testing.T, body []byte) {
	t.Helper()
	var resp map[string]string
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, string(body))
	}
	if strings.TrimSpace(resp["message"]) == "" {
		t.Fatalf("expected non-empty error message")
	}
}
