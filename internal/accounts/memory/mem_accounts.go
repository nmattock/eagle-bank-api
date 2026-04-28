package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"eagle-bank-api/internal/accounts"
)

type AccountStore struct {
	mu              sync.Mutex
	byAccountNumber map[string]*accounts.BankAccount
}

func NewRepository() *AccountStore {
	return &AccountStore{byAccountNumber: make(map[string]*accounts.BankAccount)}
}

var _ accounts.Repository = (*AccountStore)(nil)

func (r *AccountStore) Create(ctx context.Context, params accounts.CreateParams) (*accounts.BankAccount, error) {
	_ = ctx
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
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

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
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	account, ok := r.byAccountNumber[accountNumber]
	if !ok {
		return nil, accounts.ErrNotFound
	}
	return cloneAccount(account), nil
}

func cloneAccount(account *accounts.BankAccount) *accounts.BankAccount {
	out := *account
	return &out
}
