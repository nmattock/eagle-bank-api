package accounts

import (
	"context"
	"errors"
	"time"
)

var ErrAlreadyExists = errors.New("bank account already exists")
var ErrNotFound = errors.New("bank account not found")
var ErrInsufficientFunds = errors.New("insufficient funds")

type BankAccount struct {
	AccountNumber string
	UserID        string
	SortCode      string
	Name          string
	AccountType   string
	Balance       int64 // minor units (pence)
	Currency      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CreateParams struct {
	AccountNumber string
	UserID        string
	Name          string
	AccountType   string
}

type Transaction struct {
	ID            string
	AccountNumber string
	UserID        string
	Amount        int64 // minor units (pence)
	Currency      string
	Type          string
	Reference     *string
	CreatedAt     time.Time
}

type CreateTransactionParams struct {
	ID            string
	AccountNumber string
	UserID        string
	Amount        int64 // minor units (pence)
	Currency      string
	Type          string
	Reference     string
}

type Repository interface {
	Create(ctx context.Context, params CreateParams) (*BankAccount, error)
	ListByUserID(ctx context.Context, userID string) ([]*BankAccount, error)
	GetByAccountNumber(ctx context.Context, accountNumber string) (*BankAccount, error)
	CreateTransaction(ctx context.Context, params CreateTransactionParams) (*Transaction, error)
	ListTransactionsByAccountNumber(ctx context.Context, accountNumber string) ([]*Transaction, error)
	GetTransactionByID(ctx context.Context, id string) (*Transaction, error)
}
