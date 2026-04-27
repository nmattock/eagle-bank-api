package users

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when no row exists for the given user id.
var ErrNotFound = errors.New("user not found")

// ErrAlreadyExists is returned when creating or updating would violate a unique constraint (e.g. email).
var ErrAlreadyExists = errors.New("user already exists")

// User is the persistence-facing view of a bank customer (maps to users table and UserResponse).
type User struct {
	ID string

	Name string

	AddressLine1 string
	AddressLine2 *string
	AddressLine3 *string
	Town         string
	County       string
	Postcode     string

	PhoneNumber string
	Email       string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateParams is everything required to insert a row except timestamps (DB defaults).
// The service layer is responsible for assigning ID (e.g. usr-...) per OpenAPI.
type CreateParams struct {
	ID string

	Name         string
	AddressLine1 string
	AddressLine2 *string
	AddressLine3 *string
	Town         string
	County       string
	Postcode     string
	PhoneNumber  string
	Email        string
	PasswordHash string
}

type AuthCredentials struct {
	UserID       string
	PasswordHash string
}

// UpdateParams is a partial update (nil means leave unchanged).
// For address, the API sends a full address object on update when address is included;
// the HTTP layer can translate that into setting all address fields together.
type UpdateParams struct {
	Name         *string
	AddressLine1 *string
	AddressLine2 *string
	AddressLine3 *string
	Town         *string
	County       *string
	Postcode     *string
	PhoneNumber  *string
	Email        *string
}

// Repository persists users. Implementations live in infrastructure (e.g. Postgres);
// handlers and application services depend on this interface only.
type Repository interface {
	// Create inserts a new user. Duplicate id or email returns ErrAlreadyExists.
	Create(ctx context.Context, params CreateParams) (*User, error)

	// GetByID returns the user or ErrNotFound.
	GetByID(ctx context.Context, id string) (*User, error)

	// GetAuthCredentialsByEmail returns password credentials needed to authenticate by email.
	GetAuthCredentialsByEmail(ctx context.Context, email string) (*AuthCredentials, error)

	// Update applies a partial update and returns the stored row (including new updated_at).
	// Returns ErrNotFound if id does not exist, or ErrAlreadyExists if email collides.
	Update(ctx context.Context, id string, params UpdateParams) (*User, error)

	// Delete removes the user. Returns ErrNotFound if id does not exist.
	// Callers enforce "cannot delete if bank accounts exist" before invoking Delete.
	Delete(ctx context.Context, id string) error
}
