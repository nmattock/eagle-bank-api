package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"eagle-bank-api/internal/users"
)

// UserDb implements users.Repository against the users table.
type UserDb struct {
	db *sql.DB
}

// NewRepository returns a Postgres-backed users.Repository.
func NewRepository(db *sql.DB) *UserDb {
	return &UserDb{db: db}
}

var _ users.Repository = (*UserDb)(nil)

const insertUser = `
INSERT INTO users (
	id, name,
	address_line1, address_line2, address_line3,
	town, county, postcode,
	phone_number, email
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING
	id, name,
	address_line1, address_line2, address_line3,
	town, county, postcode,
	phone_number, email,
	created_at, updated_at
`

func (r *UserDb) Create(ctx context.Context, params users.CreateParams) (*users.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, insertUser,
		params.ID,
		params.Name,
		params.AddressLine1,
		nullString(params.AddressLine2),
		nullString(params.AddressLine3),
		params.Town,
		params.County,
		params.Postcode,
		params.PhoneNumber,
		params.Email,
	)
	u, err := scanUser(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, users.ErrAlreadyExists
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_credentials (user_id, password_hash) VALUES ($1, $2)`, params.ID, params.PasswordHash); err != nil {
		if isUniqueViolation(err) {
			return nil, users.ErrAlreadyExists
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return u, nil
}

const selectUser = `
SELECT
	id, name,
	address_line1, address_line2, address_line3,
	town, county, postcode,
	phone_number, email,
	created_at, updated_at
FROM users
WHERE id = $1
`

func (r *UserDb) GetByID(ctx context.Context, id string) (*users.User, error) {
	row := r.db.QueryRowContext(ctx, selectUser, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, users.ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

func (r *UserDb) GetAuthCredentialsByEmail(ctx context.Context, email string) (*users.AuthCredentials, error) {
	var creds users.AuthCredentials
	err := r.db.QueryRowContext(ctx, `
SELECT u.id, uc.password_hash
FROM users u
JOIN user_credentials uc ON uc.user_id = u.id
WHERE u.email = $1
`, email).Scan(&creds.UserID, &creds.PasswordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, users.ErrNotFound
		}
		return nil, err
	}
	return &creds, nil
}

func (r *UserDb) Update(ctx context.Context, id string, params users.UpdateParams) (*users.User, error) {
	sets := make([]string, 0, 10)
	args := make([]any, 0, 11)
	n := 1

	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, n))
		args = append(args, val)
		n++
	}

	if params.Name != nil {
		add("name", *params.Name)
	}
	if params.AddressLine1 != nil {
		add("address_line1", *params.AddressLine1)
	}
	if params.AddressLine2 != nil {
		add("address_line2", nullString(params.AddressLine2))
	}
	if params.AddressLine3 != nil {
		add("address_line3", nullString(params.AddressLine3))
	}
	if params.Town != nil {
		add("town", *params.Town)
	}
	if params.County != nil {
		add("county", *params.County)
	}
	if params.Postcode != nil {
		add("postcode", *params.Postcode)
	}
	if params.PhoneNumber != nil {
		add("phone_number", *params.PhoneNumber)
	}
	if params.Email != nil {
		add("email", *params.Email)
	}

	sets = append(sets, "updated_at = now()")
	args = append(args, id)

	q := fmt.Sprintf(`
UPDATE users
SET %s
WHERE id = $%d
RETURNING
	id, name,
	address_line1, address_line2, address_line3,
	town, county, postcode,
	phone_number, email,
	created_at, updated_at
`, strings.Join(sets, ", "), n)

	row := r.db.QueryRowContext(ctx, q, args...)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, users.ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, users.ErrAlreadyExists
		}
		return nil, err
	}
	return u, nil
}

func (r *UserDb) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return users.ErrNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*users.User, error) {
	var u users.User
	var line2, line3 sql.NullString
	err := row.Scan(
		&u.ID,
		&u.Name,
		&u.AddressLine1,
		&line2,
		&line3,
		&u.Town,
		&u.County,
		&u.Postcode,
		&u.PhoneNumber,
		&u.Email,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.AddressLine2 = ptrFromNull(line2)
	u.AddressLine3 = ptrFromNull(line3)
	return &u, nil
}

func nullString(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

func ptrFromNull(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
