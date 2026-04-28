package accounts

import (
	"context"
	"errors"
	"time"
)

var ErrAlreadyExists = errors.New("bank account already exists")
var ErrNotFound = errors.New("bank account not found")

type BankAccount struct {
	AccountNumber string
	UserID        string
	SortCode      string
	Name          string
	AccountType   string
	Balance       float64
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

type Repository interface {
	Create(ctx context.Context, params CreateParams) (*BankAccount, error)
	ListByUserID(ctx context.Context, userID string) ([]*BankAccount, error)
	GetByAccountNumber(ctx context.Context, accountNumber string) (*BankAccount, error)
}
