package memory

import (
	"context"
	"sync"
	"time"

	"eagle-bank-api/internal/users"
)

// UserStore is an in-memory users.Repository for unit tests (no database).
type UserStore struct {
	mu        sync.Mutex
	byID      map[string]*users.User
	emailToID map[string]string
}

// NewRepository returns an empty in-memory store.
func NewRepository() *UserStore {
	return &UserStore{
		byID:      make(map[string]*users.User),
		emailToID: make(map[string]string),
	}
}

var _ users.Repository = (*UserStore)(nil)

func (r *UserStore) Create(ctx context.Context, params users.CreateParams) (*users.User, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[params.ID]; ok {
		return nil, users.ErrAlreadyExists
	}
	if existingID, ok := r.emailToID[params.Email]; ok && existingID != params.ID {
		return nil, users.ErrAlreadyExists
	}

	now := time.Now().UTC()
	u := &users.User{
		ID:           params.ID,
		Name:         params.Name,
		AddressLine1: params.AddressLine1,
		AddressLine2: copyStrPtr(params.AddressLine2),
		AddressLine3: copyStrPtr(params.AddressLine3),
		Town:         params.Town,
		County:       params.County,
		Postcode:     params.Postcode,
		PhoneNumber:  params.PhoneNumber,
		Email:        params.Email,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	r.byID[u.ID] = u
	r.emailToID[u.Email] = u.ID
	return cloneUser(u), nil
}

func (r *UserStore) GetByID(ctx context.Context, id string) (*users.User, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.byID[id]
	if !ok {
		return nil, users.ErrNotFound
	}
	return cloneUser(u), nil
}

func (r *UserStore) Update(ctx context.Context, id string, params users.UpdateParams) (*users.User, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.byID[id]
	if !ok {
		return nil, users.ErrNotFound
	}

	if params.Email != nil && *params.Email != u.Email {
		if other, taken := r.emailToID[*params.Email]; taken && other != id {
			return nil, users.ErrAlreadyExists
		}
		delete(r.emailToID, u.Email)
		r.emailToID[*params.Email] = id
	}

	if params.Name != nil {
		u.Name = *params.Name
	}
	if params.AddressLine1 != nil {
		u.AddressLine1 = *params.AddressLine1
	}
	if params.AddressLine2 != nil {
		u.AddressLine2 = copyStrPtr(params.AddressLine2)
	}
	if params.AddressLine3 != nil {
		u.AddressLine3 = copyStrPtr(params.AddressLine3)
	}
	if params.Town != nil {
		u.Town = *params.Town
	}
	if params.County != nil {
		u.County = *params.County
	}
	if params.Postcode != nil {
		u.Postcode = *params.Postcode
	}
	if params.PhoneNumber != nil {
		u.PhoneNumber = *params.PhoneNumber
	}
	if params.Email != nil {
		u.Email = *params.Email
	}
	u.UpdatedAt = time.Now().UTC()

	return cloneUser(u), nil
}

func (r *UserStore) Delete(ctx context.Context, id string) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.byID[id]
	if !ok {
		return users.ErrNotFound
	}
	delete(r.emailToID, u.Email)
	delete(r.byID, id)
	return nil
}

func cloneUser(u *users.User) *users.User {
	out := *u
	out.AddressLine2 = copyStrPtr(u.AddressLine2)
	out.AddressLine3 = copyStrPtr(u.AddressLine3)
	return &out
}

func copyStrPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
