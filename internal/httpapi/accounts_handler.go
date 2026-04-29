package httpapi

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"

	"eagle-bank-api/internal/accounts"
	"eagle-bank-api/internal/auth"
)

type AccountHandler struct {
	repo   accounts.Repository
	tokens *auth.TokenService
}

func NewAccountHandler(repo accounts.Repository, tokens *auth.TokenService) *AccountHandler {
	return &AccountHandler{repo: repo, tokens: tokens}
}

type createBankAccountRequest struct {
	Name        string `json:"name"`
	AccountType string `json:"accountType"`
}

type bankAccountResponse struct {
	AccountNumber    string  `json:"accountNumber"`
	SortCode         string  `json:"sortCode"`
	Name             string  `json:"name"`
	AccountType      string  `json:"accountType"`
	Balance          float64 `json:"balance"`
	Currency         string  `json:"currency"`
	CreatedTimestamp string  `json:"createdTimestamp"`
	UpdatedTimestamp string  `json:"updatedTimestamp"`
}

type listBankAccountsResponse struct {
	Accounts []bankAccountResponse `json:"accounts"`
}

type createTransactionRequest struct {
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Type      string  `json:"type"`
	Reference string  `json:"reference"`
}

type transactionResponse struct {
	ID               string  `json:"id"`
	Amount           float64 `json:"amount"`
	Currency         string  `json:"currency"`
	Type             string  `json:"type"`
	Reference        *string `json:"reference,omitempty"`
	UserID           string  `json:"userId"`
	CreatedTimestamp string  `json:"createdTimestamp"`
}

type listTransactionsResponse struct {
	Transactions []transactionResponse `json:"transactions"`
}

func (h *AccountHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/v1/accounts/") && strings.Contains(r.URL.Path, "/transactions") {
		h.handleAccountTransactions(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/v1/accounts/") {
		h.handleAccountByNumber(w, r)
		return
	}
	if r.URL.Path != "/v1/accounts" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPost:
		h.handleCreateAccount(w, r)
	case http.MethodGet:
		h.handleListAccounts(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *AccountHandler) handleAccountTransactions(w http.ResponseWriter, r *http.Request) {
	accountNumber, transactionID, ok := parseAccountTransactionPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodPost:
		if transactionID != "" {
			http.NotFound(w, r)
			return
		}
		h.handleCreateTransaction(w, r, accountNumber)
	case http.MethodGet:
		if transactionID == "" {
			h.handleListTransactions(w, r, accountNumber)
			return
		}
		h.handleFetchTransaction(w, r, accountNumber, transactionID)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *AccountHandler) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}

	var req createBankAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: "invalid JSON"})
		return
	}
	if err := validateCreateBankAccountRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: err.Error()})
		return
	}

	accountNumber, err := generateAccountNumber()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	account, err := h.repo.Create(r.Context(), accounts.CreateParams{
		AccountNumber: accountNumber,
		UserID:        claims.Subject,
		Name:          req.Name,
		AccountType:   req.AccountType,
	})
	if err != nil {
		if errors.Is(err, accounts.ErrAlreadyExists) {
			writeJSON(w, http.StatusConflict, errorResponse{Message: "bank account already exists"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toBankAccountResponse(account))
}

func (h *AccountHandler) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	accountsForUser, err := h.repo.ListByUserID(r.Context(), claims.Subject)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	resp := listBankAccountsResponse{Accounts: make([]bankAccountResponse, 0, len(accountsForUser))}
	for _, account := range accountsForUser {
		resp.Accounts = append(resp.Accounts, toBankAccountResponse(account))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AccountHandler) handleAccountByNumber(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	accountNumber := strings.TrimPrefix(r.URL.Path, "/v1/accounts/")
	if accountNumber == "" || strings.Contains(accountNumber, "/") {
		http.NotFound(w, r)
		return
	}
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}

	account, err := h.repo.GetByAccountNumber(r.Context(), accountNumber)
	if err != nil {
		if errors.Is(err, accounts.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "bank account not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	if account.UserID != claims.Subject {
		writeJSON(w, http.StatusForbidden, errorResponse{Message: "forbidden"})
		return
	}
	writeJSON(w, http.StatusOK, toBankAccountResponse(account))
}

func (h *AccountHandler) handleCreateTransaction(w http.ResponseWriter, r *http.Request, accountNumber string) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}

	account, err := h.repo.GetByAccountNumber(r.Context(), accountNumber)
	if err != nil {
		if errors.Is(err, accounts.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "bank account not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	if account.UserID != claims.Subject {
		writeJSON(w, http.StatusForbidden, errorResponse{Message: "forbidden"})
		return
	}

	var req createTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: "invalid JSON"})
		return
	}
	if err := validateCreateTransactionRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Message: err.Error()})
		return
	}

	transactionID, err := generateTransactionID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	transaction, err := h.repo.CreateTransaction(r.Context(), accounts.CreateTransactionParams{
		ID:            transactionID,
		AccountNumber: accountNumber,
		UserID:        claims.Subject,
		Amount:        poundsToPence(req.Amount),
		Currency:      req.Currency,
		Type:          req.Type,
		Reference:     req.Reference,
	})
	if err != nil {
		if errors.Is(err, accounts.ErrInsufficientFunds) {
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Message: "insufficient funds"})
			return
		}
		if errors.Is(err, accounts.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "bank account not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	writeJSON(w, http.StatusCreated, toTransactionResponse(transaction))
}

func (h *AccountHandler) handleListTransactions(w http.ResponseWriter, r *http.Request, accountNumber string) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}

	account, err := h.repo.GetByAccountNumber(r.Context(), accountNumber)
	if err != nil {
		if errors.Is(err, accounts.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "bank account not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	if account.UserID != claims.Subject {
		writeJSON(w, http.StatusForbidden, errorResponse{Message: "forbidden"})
		return
	}

	transactions, err := h.repo.ListTransactionsByAccountNumber(r.Context(), accountNumber)
	if err != nil {
		if errors.Is(err, accounts.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "bank account not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	resp := listTransactionsResponse{Transactions: make([]transactionResponse, 0, len(transactions))}
	for _, transaction := range transactions {
		resp.Transactions = append(resp.Transactions, toTransactionResponse(transaction))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AccountHandler) handleFetchTransaction(w http.ResponseWriter, r *http.Request, accountNumber string, transactionID string) {
	claims, ok := h.authenticate(w, r)
	if !ok {
		return
	}

	account, err := h.repo.GetByAccountNumber(r.Context(), accountNumber)
	if err != nil {
		if errors.Is(err, accounts.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "bank account not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	if account.UserID != claims.Subject {
		writeJSON(w, http.StatusForbidden, errorResponse{Message: "forbidden"})
		return
	}

	transaction, err := h.repo.GetTransactionByID(r.Context(), transactionID)
	if err != nil {
		if errors.Is(err, accounts.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Message: "transaction not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Message: "internal server error"})
		return
	}
	if transaction.AccountNumber != accountNumber {
		writeJSON(w, http.StatusNotFound, errorResponse{Message: "transaction not found"})
		return
	}
	writeJSON(w, http.StatusOK, toTransactionResponse(transaction))
}

func (h *AccountHandler) authenticate(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	if h.tokens == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "unauthorized"})
		return nil, false
	}
	token, ok := bearerToken(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "unauthorized"})
		return nil, false
	}
	claims, err := h.tokens.Verify(token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Message: "unauthorized"})
		return nil, false
	}
	return claims, true
}

func parseAccountTransactionPath(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/v1/accounts/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 && len(parts) != 3 {
		return "", "", false
	}
	if parts[0] == "" || parts[1] != "transactions" {
		return "", "", false
	}
	if len(parts) == 3 && parts[2] == "" {
		return "", "", false
	}
	if strings.Contains(parts[0], "/") {
		return "", "", false
	}
	if len(parts) == 2 {
		return parts[0], "", true
	}
	return parts[0], parts[2], true
}

func validateCreateBankAccountRequest(req createBankAccountRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(req.AccountType) == "" {
		return errors.New("accountType is required")
	}
	if req.AccountType != "personal" {
		return errors.New("accountType must be personal")
	}
	return nil
}

func validateCreateTransactionRequest(req createTransactionRequest) error {
	if req.Amount <= 0 {
		return errors.New("amount is required")
	}
	if req.Currency != "GBP" {
		return errors.New("currency must be GBP")
	}
	if req.Type != "deposit" && req.Type != "withdrawal" {
		return errors.New("type must be deposit or withdrawal")
	}
	return nil
}

func generateAccountNumber() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("01%06d", n.Int64()), nil
}

func generateTransactionID() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tan-%09d", n.Int64()), nil
}

func toBankAccountResponse(account *accounts.BankAccount) bankAccountResponse {
	return bankAccountResponse{
		AccountNumber:    account.AccountNumber,
		SortCode:         account.SortCode,
		Name:             account.Name,
		AccountType:      account.AccountType,
		Balance:          penceToPounds(account.Balance),
		Currency:         account.Currency,
		CreatedTimestamp: account.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedTimestamp: account.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func toTransactionResponse(transaction *accounts.Transaction) transactionResponse {
	return transactionResponse{
		ID:               transaction.ID,
		Amount:           penceToPounds(transaction.Amount),
		Currency:         transaction.Currency,
		Type:             transaction.Type,
		Reference:        transaction.Reference,
		UserID:           transaction.UserID,
		CreatedTimestamp: transaction.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func penceToPounds(pence int64) float64 {
	return float64(pence) / 100
}

func poundsToPence(pounds float64) int64 {
	return int64(math.Round(pounds * 100))
}
