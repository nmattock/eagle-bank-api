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
	handler := AuthMiddleware(tokenService)(NewAccountHandler(memory.NewRepository()))
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
		AccountType:   accounts.AccountTypePersonal,
	})
	ownAccount2 := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000002",
		UserID:        owner.ID,
		Name:          "Savings Account",
		AccountType:   accounts.AccountTypePersonal,
	})
	createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000003",
		UserID:        otherUser.ID,
		Name:          "Other User Account",
		AccountType:   accounts.AccountTypePersonal,
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
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
		AccountType:   accounts.AccountTypePersonal,
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
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
		AccountType:   accounts.AccountTypePersonal,
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
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
	handler := AuthMiddleware(tokenService)(NewAccountHandler(memory.NewRepository()))
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/01999999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())
}

func TestCreateTransaction_AuthenticatedOwnerDepositsMoney_ReturnsTransactionAndUpdatesBalance(t *testing.T) {
	owner, tokenService, token := testAuthenticatedUser(t, "usr-depositowner1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000101",
		UserID:        owner.ID,
		Name:          "Personal Bank Account",
		AccountType:   accounts.AccountTypePersonal,
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
	req := httptest.NewRequest(http.MethodPost, "/v1/accounts/"+account.AccountNumber+"/transactions", strings.NewReader(`{
		"amount":25.50,
		"currency":"GBP",
		"type":"deposit",
		"reference":"Payday"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	assertTransactionResponse(t, rr.Body.Bytes(), "deposit", 25.50, "Payday", owner.ID)

	updatedAccount, err := repo.GetByAccountNumber(context.Background(), account.AccountNumber)
	if err != nil {
		t.Fatalf("get updated account: %v", err)
	}
	if updatedAccount.Balance != 2550 {
		t.Fatalf("balance = %d, want 2550", updatedAccount.Balance)
	}
}

func TestCreateTransaction_AuthenticatedOwnerWithdrawsMoney_ReturnsTransactionAndUpdatesBalance(t *testing.T) {
	owner, tokenService, token := testAuthenticatedUser(t, "usr-withdrawowner1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000102",
		UserID:        owner.ID,
		Name:          "Personal Bank Account",
		AccountType:   accounts.AccountTypePersonal,
	})
	_, err := repo.CreateTransaction(context.Background(), accounts.CreateTransactionParams{
		ID:            "tan-seedfunds1",
		AccountNumber: account.AccountNumber,
		UserID:        owner.ID,
		Amount:        10000,
		Currency:      "GBP",
		Type:          accounts.TransactionTypeDeposit,
		Reference:     "Seed funds",
	})
	if err != nil {
		t.Fatalf("seed deposit: %v", err)
	}

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
	req := httptest.NewRequest(http.MethodPost, "/v1/accounts/"+account.AccountNumber+"/transactions", strings.NewReader(`{
		"amount":30.25,
		"currency":"GBP",
		"type":"withdrawal",
		"reference":"Groceries"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	assertTransactionResponse(t, rr.Body.Bytes(), "withdrawal", 30.25, "Groceries", owner.ID)

	updatedAccount, err := repo.GetByAccountNumber(context.Background(), account.AccountNumber)
	if err != nil {
		t.Fatalf("get updated account: %v", err)
	}
	if updatedAccount.Balance != 6975 {
		t.Fatalf("balance = %d, want 6975", updatedAccount.Balance)
	}
}

func TestCreateTransaction_AuthenticatedOwnerWithdrawsWithInsufficientFunds_ReturnsUnprocessableEntity(t *testing.T) {
	owner, tokenService, token := testAuthenticatedUser(t, "usr-insufficientfunds1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000103",
		UserID:        owner.ID,
		Name:          "Personal Bank Account",
		AccountType:   accounts.AccountTypePersonal,
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
	req := httptest.NewRequest(http.MethodPost, "/v1/accounts/"+account.AccountNumber+"/transactions", strings.NewReader(`{
		"amount":30.25,
		"currency":"GBP",
		"type":"withdrawal",
		"reference":"Groceries"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusUnprocessableEntity, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())

	updatedAccount, err := repo.GetByAccountNumber(context.Background(), account.AccountNumber)
	if err != nil {
		t.Fatalf("get updated account: %v", err)
	}
	if updatedAccount.Balance != 0 {
		t.Fatalf("balance = %v, want unchanged 0", updatedAccount.Balance)
	}
}

func TestCreateTransaction_AuthenticatedNonOwner_ReturnsForbidden(t *testing.T) {
	owner, _, _ := testAuthenticatedUser(t, "usr-transactionowner1")
	_, tokenService, token := testAuthenticatedUser(t, "usr-transactionintruder1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000104",
		UserID:        owner.ID,
		Name:          "Owner Account",
		AccountType:   accounts.AccountTypePersonal,
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
	req := httptest.NewRequest(http.MethodPost, "/v1/accounts/"+account.AccountNumber+"/transactions", strings.NewReader(`{
		"amount":10.00,
		"currency":"GBP",
		"type":"deposit"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())

	updatedAccount, err := repo.GetByAccountNumber(context.Background(), account.AccountNumber)
	if err != nil {
		t.Fatalf("get updated account: %v", err)
	}
	if updatedAccount.Balance != 0 {
		t.Fatalf("balance = %v, want unchanged 0", updatedAccount.Balance)
	}
}

func TestCreateTransaction_AuthenticatedUserRequestsNonExistentAccount_ReturnsNotFound(t *testing.T) {
	_, tokenService, token := testAuthenticatedUser(t, "usr-missingtransactionaccount1")
	handler := AuthMiddleware(tokenService)(NewAccountHandler(memory.NewRepository()))
	req := httptest.NewRequest(http.MethodPost, "/v1/accounts/01999998/transactions", strings.NewReader(`{
		"amount":10.00,
		"currency":"GBP",
		"type":"deposit"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())
}

func TestCreateTransaction_AuthenticatedUserMissingRequiredData_ReturnsBadRequest(t *testing.T) {
	owner, tokenService, token := testAuthenticatedUser(t, "usr-missingtransactiondata1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000105",
		UserID:        owner.ID,
		Name:          "Personal Bank Account",
		AccountType:   accounts.AccountTypePersonal,
	})
	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))

	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing amount",
			body: `{"currency":"GBP","type":"deposit"}`,
		},
		{
			name: "missing currency",
			body: `{"amount":10.00,"type":"deposit"}`,
		},
		{
			name: "missing type",
			body: `{"amount":10.00,"currency":"GBP"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/accounts/"+account.AccountNumber+"/transactions", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
			}
			assertErrorMessage(t, rr.Body.Bytes())
		})
	}
}

func TestListTransactions_AuthenticatedOwner_ReturnsTransactions(t *testing.T) {
	owner, tokenService, token := testAuthenticatedUser(t, "usr-listtransactions1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000106",
		UserID:        owner.ID,
		Name:          "Personal Bank Account",
		AccountType:   accounts.AccountTypePersonal,
	})
	deposit := createTestTransaction(t, repo, accounts.CreateTransactionParams{
		ID:            "tan-listdeposit1",
		AccountNumber: account.AccountNumber,
		UserID:        owner.ID,
		Amount:        10000,
		Currency:      "GBP",
		Type:          accounts.TransactionTypeDeposit,
		Reference:     "Salary",
	})
	withdrawal := createTestTransaction(t, repo, accounts.CreateTransactionParams{
		ID:            "tan-listwithdraw1",
		AccountNumber: account.AccountNumber,
		UserID:        owner.ID,
		Amount:        2500,
		Currency:      "GBP",
		Type:          accounts.TransactionTypeWithdrawal,
		Reference:     "Lunch",
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/"+account.AccountNumber+"/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var resp struct {
		Transactions []transactionResponse `json:"transactions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rr.Body.String())
	}
	if len(resp.Transactions) != 2 {
		t.Fatalf("transactions len = %d, want 2; body=%s", len(resp.Transactions), rr.Body.String())
	}
	assertTransactionInList(t, resp.Transactions, deposit.ID, "deposit", 100.00, "Salary", owner.ID)
	assertTransactionInList(t, resp.Transactions, withdrawal.ID, "withdrawal", 25.00, "Lunch", owner.ID)
}

func TestListTransactions_AuthenticatedNonOwner_ReturnsForbidden(t *testing.T) {
	owner, _, _ := testAuthenticatedUser(t, "usr-listtransactionsowner1")
	_, tokenService, token := testAuthenticatedUser(t, "usr-listtransactionsintruder1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000107",
		UserID:        owner.ID,
		Name:          "Owner Account",
		AccountType:   accounts.AccountTypePersonal,
	})
	createTestTransaction(t, repo, accounts.CreateTransactionParams{
		ID:            "tan-private1",
		AccountNumber: account.AccountNumber,
		UserID:        owner.ID,
		Amount:        5000,
		Currency:      "GBP",
		Type:          accounts.TransactionTypeDeposit,
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/"+account.AccountNumber+"/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())
}

func TestListTransactions_AuthenticatedUserRequestsNonExistentAccount_ReturnsNotFound(t *testing.T) {
	_, tokenService, token := testAuthenticatedUser(t, "usr-listmissingtransactions1")
	handler := AuthMiddleware(tokenService)(NewAccountHandler(memory.NewRepository()))
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/01999997/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())
}

func TestFetchTransaction_AuthenticatedOwner_ReturnsTransaction(t *testing.T) {
	owner, tokenService, token := testAuthenticatedUser(t, "usr-fetchtransaction1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000108",
		UserID:        owner.ID,
		Name:          "Personal Bank Account",
		AccountType:   accounts.AccountTypePersonal,
	})
	transaction := createTestTransaction(t, repo, accounts.CreateTransactionParams{
		ID:            "tan-fetch1",
		AccountNumber: account.AccountNumber,
		UserID:        owner.ID,
		Amount:        4000,
		Currency:      "GBP",
		Type:          accounts.TransactionTypeDeposit,
		Reference:     "Gift",
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/"+account.AccountNumber+"/transactions/"+transaction.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	assertTransactionResponseWithID(t, rr.Body.Bytes(), transaction.ID, "deposit", 40.00, "Gift", owner.ID)
}

func TestFetchTransaction_AuthenticatedNonOwner_ReturnsForbidden(t *testing.T) {
	owner, _, _ := testAuthenticatedUser(t, "usr-fetchtransactionowner1")
	_, tokenService, token := testAuthenticatedUser(t, "usr-fetchtransactionintruder1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000109",
		UserID:        owner.ID,
		Name:          "Owner Account",
		AccountType:   accounts.AccountTypePersonal,
	})
	transaction := createTestTransaction(t, repo, accounts.CreateTransactionParams{
		ID:            "tan-forbidden1",
		AccountNumber: account.AccountNumber,
		UserID:        owner.ID,
		Amount:        5000,
		Currency:      "GBP",
		Type:          accounts.TransactionTypeDeposit,
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/"+account.AccountNumber+"/transactions/"+transaction.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusForbidden, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())
}

func TestFetchTransaction_AuthenticatedUserRequestsNonExistentAccount_ReturnsNotFound(t *testing.T) {
	_, tokenService, token := testAuthenticatedUser(t, "usr-fetchmissingaccount1")
	handler := AuthMiddleware(tokenService)(NewAccountHandler(memory.NewRepository()))
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/01999996/transactions/tan-missingaccount1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())
}

func TestFetchTransaction_AuthenticatedOwnerRequestsNonExistentTransaction_ReturnsNotFound(t *testing.T) {
	owner, tokenService, token := testAuthenticatedUser(t, "usr-fetchmissingtransaction1")
	repo := memory.NewRepository()
	account := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000110",
		UserID:        owner.ID,
		Name:          "Personal Bank Account",
		AccountType:   accounts.AccountTypePersonal,
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/"+account.AccountNumber+"/transactions/tan-doesnotexist1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())
}

func TestFetchTransaction_AuthenticatedOwnerRequestsTransactionForWrongAccount_ReturnsNotFound(t *testing.T) {
	owner, tokenService, token := testAuthenticatedUser(t, "usr-fetchwrongaccount1")
	repo := memory.NewRepository()
	requestedAccount := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000111",
		UserID:        owner.ID,
		Name:          "Current Account",
		AccountType:   accounts.AccountTypePersonal,
	})
	otherAccount := createTestAccount(t, repo, accounts.CreateParams{
		AccountNumber: "01000112",
		UserID:        owner.ID,
		Name:          "Savings Account",
		AccountType:   accounts.AccountTypePersonal,
	})
	transaction := createTestTransaction(t, repo, accounts.CreateTransactionParams{
		ID:            "tan-wrongaccount1",
		AccountNumber: otherAccount.AccountNumber,
		UserID:        owner.ID,
		Amount:        7500,
		Currency:      "GBP",
		Type:          accounts.TransactionTypeDeposit,
	})

	handler := AuthMiddleware(tokenService)(NewAccountHandler(repo))
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/"+requestedAccount.AccountNumber+"/transactions/"+transaction.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes())
}

func TestPoundsToPence(t *testing.T) {
	tests := []struct {
		name   string
		pounds float64
		want   int64
	}{
		{name: "zero", pounds: 0, want: 0},
		{name: "whole pounds", pounds: 1, want: 100},
		{name: "pence only", pounds: 0.99, want: 99},
		{name: "pounds and pence", pounds: 25.50, want: 2550},
		{name: "single penny", pounds: 0.01, want: 1},
		{name: "rounds half-penny up", pounds: 0.005, want: 1},
		{name: "truncates sub-half-penny", pounds: 0.004, want: 0},
		{name: "large value", pounds: 9999.99, want: 999999},
		{name: "floating point classic 0.1+0.2", pounds: 0.1 + 0.2, want: 30},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := poundsToPence(tc.pounds)
			if got != tc.want {
				t.Fatalf("poundsToPence(%v) = %d, want %d", tc.pounds, got, tc.want)
			}
		})
	}
}

func TestPenceToPounds(t *testing.T) {
	tests := []struct {
		name  string
		pence int64
		want  float64
	}{
		{name: "zero", pence: 0, want: 0},
		{name: "whole pound", pence: 100, want: 1},
		{name: "pence only", pence: 99, want: 0.99},
		{name: "pounds and pence", pence: 2550, want: 25.50},
		{name: "single penny", pence: 1, want: 0.01},
		{name: "large value", pence: 999999, want: 9999.99},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := penceToPounds(tc.pence)
			if got != tc.want {
				t.Fatalf("penceToPounds(%d) = %v, want %v", tc.pence, got, tc.want)
			}
		})
	}
}

func TestPenceConversion_RoundTrip(t *testing.T) {
	t.Run("pence to pounds to pence", func(t *testing.T) {
		values := []int64{0, 1, 10, 50, 99, 100, 101, 150, 2550, 9999, 100000, 999999, 1000000}
		for _, pence := range values {
			got := poundsToPence(penceToPounds(pence))
			if got != pence {
				t.Fatalf("round-trip failed for %d pence: got %d", pence, got)
			}
		}
	})

	t.Run("pounds to pence to pounds", func(t *testing.T) {
		values := []float64{0, 0.01, 0.10, 0.50, 0.99, 1.00, 1.01, 1.50, 25.50, 99.99, 1000.00, 9999.99, 10000.00}
		for _, pounds := range values {
			got := penceToPounds(poundsToPence(pounds))
			if got != pounds {
				t.Fatalf("round-trip failed for %.2f pounds: got %v", pounds, got)
			}
		}
	})

	t.Run("all whole-penny amounts under one pound", func(t *testing.T) {
		for pence := int64(0); pence <= 100; pence++ {
			got := poundsToPence(penceToPounds(pence))
			if got != pence {
				t.Fatalf("round-trip failed for %d pence: got %d", pence, got)
			}
		}
	})
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

func createTestTransaction(t *testing.T, repo accounts.Repository, params accounts.CreateTransactionParams) *accounts.Transaction {
	t.Helper()
	transaction, err := repo.CreateTransaction(context.Background(), params)
	if err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	return transaction
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

func assertTransactionResponse(t *testing.T, body []byte, wantType string, wantAmount float64, wantReference string, wantUserID string) {
	t.Helper()
	var resp struct {
		ID               string  `json:"id"`
		Amount           float64 `json:"amount"`
		Currency         string  `json:"currency"`
		Type             string  `json:"type"`
		Reference        string  `json:"reference"`
		UserID           string  `json:"userId"`
		CreatedTimestamp string  `json:"createdTimestamp"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, string(body))
	}
	if !strings.HasPrefix(resp.ID, "tan-") {
		t.Fatalf("id = %q, want tan- prefix", resp.ID)
	}
	if resp.Amount != wantAmount {
		t.Fatalf("amount = %v, want %v", resp.Amount, wantAmount)
	}
	if resp.Currency != "GBP" {
		t.Fatalf("currency = %q, want GBP", resp.Currency)
	}
	if resp.Type != wantType {
		t.Fatalf("type = %q, want %q", resp.Type, wantType)
	}
	if resp.Reference != wantReference {
		t.Fatalf("reference = %q, want %q", resp.Reference, wantReference)
	}
	if resp.UserID != wantUserID {
		t.Fatalf("userId = %q, want %q", resp.UserID, wantUserID)
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.CreatedTimestamp); err != nil {
		t.Fatalf("createdTimestamp format: %v (%q)", err, resp.CreatedTimestamp)
	}
}

func assertTransactionResponseWithID(t *testing.T, body []byte, wantID string, wantType string, wantAmount float64, wantReference string, wantUserID string) {
	t.Helper()
	var resp transactionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, string(body))
	}
	if resp.ID != wantID {
		t.Fatalf("id = %q, want %q", resp.ID, wantID)
	}
	if resp.Type != wantType || resp.Amount != wantAmount || resp.Currency != "GBP" || resp.UserID != wantUserID {
		t.Fatalf("unexpected transaction response: %+v", resp)
	}
	if resp.Reference == nil || *resp.Reference != wantReference {
		t.Fatalf("reference = %+v, want %q", resp.Reference, wantReference)
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.CreatedTimestamp); err != nil {
		t.Fatalf("createdTimestamp format: %v (%q)", err, resp.CreatedTimestamp)
	}
}

func assertTransactionInList(t *testing.T, transactions []transactionResponse, wantID string, wantType string, wantAmount float64, wantReference string, wantUserID string) {
	t.Helper()
	for _, transaction := range transactions {
		if transaction.ID != wantID {
			continue
		}
		if transaction.Type != wantType || transaction.Amount != wantAmount || transaction.Currency != "GBP" || transaction.UserID != wantUserID {
			t.Fatalf("unexpected transaction response: %+v", transaction)
		}
		if transaction.Reference == nil || *transaction.Reference != wantReference {
			t.Fatalf("reference = %+v, want %q", transaction.Reference, wantReference)
		}
		if _, err := time.Parse(time.RFC3339Nano, transaction.CreatedTimestamp); err != nil {
			t.Fatalf("createdTimestamp format: %v (%q)", err, transaction.CreatedTimestamp)
		}
		return
	}
	t.Fatalf("transaction %q not found in %+v", wantID, transactions)
}
