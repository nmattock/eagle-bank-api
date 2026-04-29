package postgres

import (
	"context"
	"database/sql"
	"errors"

	"eagle-bank-api/internal/accounts"
	"github.com/jackc/pgx/v5/pgconn"
)

type AccountDb struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *AccountDb {
	return &AccountDb{db: db}
}

var _ accounts.Repository = (*AccountDb)(nil)

const insertAccount = `
INSERT INTO bank_accounts (account_number, user_id, name, account_type)
VALUES ($1, $2, $3, $4)
RETURNING account_number, user_id, sort_code, name, account_type, balance, currency, created_at, updated_at
`

func (r *AccountDb) Create(ctx context.Context, params accounts.CreateParams) (*accounts.BankAccount, error) {
	row := r.db.QueryRowContext(ctx, insertAccount, params.AccountNumber, params.UserID, params.Name, params.AccountType)
	account, err := scanAccount(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, accounts.ErrAlreadyExists
		}
		return nil, err
	}
	return account, nil
}

func (r *AccountDb) ListByUserID(ctx context.Context, userID string) ([]*accounts.BankAccount, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT account_number, user_id, sort_code, name, account_type, balance, currency, created_at, updated_at
FROM bank_accounts
WHERE user_id = $1
ORDER BY account_number ASC
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*accounts.BankAccount, 0)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *AccountDb) GetByAccountNumber(ctx context.Context, accountNumber string) (*accounts.BankAccount, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT account_number, user_id, sort_code, name, account_type, balance, currency, created_at, updated_at
FROM bank_accounts
WHERE account_number = $1
`, accountNumber)
	account, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, accounts.ErrNotFound
		}
		return nil, err
	}
	return account, nil
}

func (r *AccountDb) CreateTransaction(ctx context.Context, params accounts.CreateTransactionParams) (*accounts.Transaction, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var currentBalance int64
	err = tx.QueryRowContext(ctx, `
SELECT balance
FROM bank_accounts
WHERE account_number = $1
FOR UPDATE
`, params.AccountNumber).Scan(&currentBalance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, accounts.ErrNotFound
		}
		return nil, err
	}
	if params.Type == accounts.TransactionTypeWithdrawal && currentBalance < params.Amount {
		return nil, accounts.ErrInsufficientFunds
	}

	transaction, err := scanTransaction(tx.QueryRowContext(ctx, `
INSERT INTO transactions (id, account_number, user_id, amount, currency, type, reference)
VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))
RETURNING id, account_number, user_id, amount, currency, type, reference, created_at
`, params.ID, params.AccountNumber, params.UserID, params.Amount, params.Currency, params.Type, params.Reference))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, accounts.ErrAlreadyExists
		}
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
UPDATE bank_accounts
SET balance = CASE
    WHEN $1 = 'deposit' THEN balance + $2
    WHEN $1 = 'withdrawal' THEN balance - $2
    ELSE balance
END,
updated_at = now()
WHERE account_number = $3
`, params.Type, params.Amount, params.AccountNumber)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return transaction, nil
}

func (r *AccountDb) ListTransactionsByAccountNumber(ctx context.Context, accountNumber string) ([]*accounts.Transaction, error) {
	var exists int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM bank_accounts WHERE account_number = $1`, accountNumber).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, accounts.ErrNotFound
		}
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT id, account_number, user_id, amount, currency, type, reference, created_at
FROM transactions
WHERE account_number = $1
ORDER BY created_at ASC, id ASC
`, accountNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*accounts.Transaction, 0)
	for rows.Next() {
		transaction, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *AccountDb) GetTransactionByID(ctx context.Context, id string) (*accounts.Transaction, error) {
	transaction, err := scanTransaction(r.db.QueryRowContext(ctx, `
SELECT id, account_number, user_id, amount, currency, type, reference, created_at
FROM transactions
WHERE id = $1
`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, accounts.ErrNotFound
		}
		return nil, err
	}
	return transaction, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAccount(row rowScanner) (*accounts.BankAccount, error) {
	var account accounts.BankAccount
	err := row.Scan(
		&account.AccountNumber,
		&account.UserID,
		&account.SortCode,
		&account.Name,
		&account.AccountType,
		&account.Balance,
		&account.Currency,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func scanTransaction(row rowScanner) (*accounts.Transaction, error) {
	var transaction accounts.Transaction
	var reference sql.NullString
	err := row.Scan(
		&transaction.ID,
		&transaction.AccountNumber,
		&transaction.UserID,
		&transaction.Amount,
		&transaction.Currency,
		&transaction.Type,
		&reference,
		&transaction.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if reference.Valid {
		transaction.Reference = &reference.String
	}
	return &transaction, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
