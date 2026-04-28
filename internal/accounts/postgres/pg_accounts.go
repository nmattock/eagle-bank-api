package postgres

import (
	"context"
	"database/sql"
	"errors"
	"sort"

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
	sort.Slice(result, func(i, j int) bool {
		return result[i].AccountNumber < result[j].AccountNumber
	})
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
