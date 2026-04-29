package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"eagle-bank-api/internal/accounts"
)

type AccountStore struct {
	mu              sync.RWMutex
	byAccountNumber map[string]*accounts.BankAccount
	transactions    map[string]*accounts.Transaction
}

func NewRepository() *AccountStore {
	return &AccountStore{
		byAccountNumber: make(map[string]*accounts.BankAccount),
		transactions:    make(map[string]*accounts.Transaction),
	}
}

var _ accounts.Repository = (*AccountStore)(nil)

func (r *AccountStore) Create(ctx context.Context, params accounts.CreateParams) (*accounts.BankAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byAccountNumber[params.AccountNumber]; ok {
		return nil, accounts.ErrAlreadyExists
	}
	now := time.Now().UTC()
	account := &accounts.BankAccount{
		AccountNumber: params.AccountNumber,
		UserID:        params.UserID,
		SortCode:      "10-10-10",
		Name:          params.Name,
		AccountType:   params.AccountType,
		Balance:       0,
		Currency:      "GBP",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	r.byAccountNumber[account.AccountNumber] = account
	return cloneAccount(account), nil
}

func (r *AccountStore) ListByUserID(ctx context.Context, userID string) ([]*accounts.BankAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*accounts.BankAccount, 0)
	for _, account := range r.byAccountNumber {
		if account.UserID == userID {
			result = append(result, cloneAccount(account))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].AccountNumber < result[j].AccountNumber
	})
	return result, nil
}

func (r *AccountStore) GetByAccountNumber(ctx context.Context, accountNumber string) (*accounts.BankAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	account, ok := r.byAccountNumber[accountNumber]
	if !ok {
		return nil, accounts.ErrNotFound
	}
	return cloneAccount(account), nil
}

func (r *AccountStore) CreateTransaction(ctx context.Context, params accounts.CreateTransactionParams) (*accounts.Transaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.transactions[params.ID]; ok {
		return nil, accounts.ErrAlreadyExists
	}
	account, ok := r.byAccountNumber[params.AccountNumber]
	if !ok {
		return nil, accounts.ErrNotFound
	}

	switch params.Type {
	case accounts.TransactionTypeDeposit:
		account.Balance += params.Amount
	case accounts.TransactionTypeWithdrawal:
		if account.Balance < params.Amount {
			return nil, accounts.ErrInsufficientFunds
		}
		account.Balance -= params.Amount
	default:
		return nil, accounts.ErrInvalidTransactionType
	}
	account.UpdatedAt = time.Now().UTC()

	var reference *string
	if params.Reference != "" {
		v := params.Reference
		reference = &v
	}
	transaction := &accounts.Transaction{
		ID:            params.ID,
		AccountNumber: params.AccountNumber,
		UserID:        params.UserID,
		Amount:        params.Amount,
		Currency:      params.Currency,
		Type:          params.Type,
		Reference:     reference,
		CreatedAt:     time.Now().UTC(),
	}
	r.transactions[transaction.ID] = transaction
	return cloneTransaction(transaction), nil
}

func (r *AccountStore) ListTransactionsByAccountNumber(ctx context.Context, accountNumber string) ([]*accounts.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.byAccountNumber[accountNumber]; !ok {
		return nil, accounts.ErrNotFound
	}
	result := make([]*accounts.Transaction, 0)
	for _, transaction := range r.transactions {
		if transaction.AccountNumber == accountNumber {
			result = append(result, cloneTransaction(transaction))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result, nil
}

func (r *AccountStore) GetTransactionByID(ctx context.Context, id string) (*accounts.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	transaction, ok := r.transactions[id]
	if !ok {
		return nil, accounts.ErrNotFound
	}
	return cloneTransaction(transaction), nil
}

func cloneAccount(account *accounts.BankAccount) *accounts.BankAccount {
	out := *account
	return &out
}

func cloneTransaction(transaction *accounts.Transaction) *accounts.Transaction {
	out := *transaction
	if transaction.Reference != nil {
		v := *transaction.Reference
		out.Reference = &v
	}
	return &out
}
