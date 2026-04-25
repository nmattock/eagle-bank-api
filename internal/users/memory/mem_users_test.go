package memory

import (
	"context"
	"errors"
	"testing"

	"eagle-bank-api/internal/users"
)

func TestRepository_Create(t *testing.T) {
	ctx := context.Background()
	r := NewRepository()

	line2 := "Suite 1"
	u, err := r.Create(ctx, users.CreateParams{
		ID:             "usr-test1",
		Name:           "Alice",
		AddressLine1:   "1 High St",
		AddressLine2:   &line2,
		Town:           "London",
		County:         "Greater London",
		Postcode:       "SW1A 1AA",
		PhoneNumber:    "+441234567890",
		Email:          "alice@example.com",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID != "usr-test1" || u.Email != "alice@example.com" {
		t.Fatalf("unexpected user: %+v", u)
	}
	if u.AddressLine2 == nil || *u.AddressLine2 != "Suite 1" {
		t.Fatalf("AddressLine2: %+v", u.AddressLine2)
	}

	got, err := r.GetByID(ctx, "usr-test1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Alice" {
		t.Fatalf("GetByID name: %q", got.Name)
	}
}

func TestRepository_Create_duplicateEmail(t *testing.T) {
	ctx := context.Background()
	r := NewRepository()
	params := users.CreateParams{
		ID:           "usr-a",
		Name:         "A",
		AddressLine1: "1 St",
		Town:         "T",
		County:       "C",
		Postcode:     "P1",
		PhoneNumber:  "+441234567890",
		Email:        "dup@example.com",
	}
	if _, err := r.Create(ctx, params); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	params.ID = "usr-b"
	_, err := r.Create(ctx, params)
	if !errors.Is(err, users.ErrAlreadyExists) {
		t.Fatalf("second Create: want ErrAlreadyExists, got %v", err)
	}
}

func TestRepository_Create_duplicateID(t *testing.T) {
	ctx := context.Background()
	r := NewRepository()
	base := users.CreateParams{
		ID:           "usr-same",
		Name:         "A",
		AddressLine1: "1 St",
		Town:         "T",
		County:       "C",
		Postcode:     "P1",
		PhoneNumber:  "+441234567890",
		Email:        "a@example.com",
	}
	if _, err := r.Create(ctx, base); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	base.Email = "b@example.com"
	_, err := r.Create(ctx, base)
	if !errors.Is(err, users.ErrAlreadyExists) {
		t.Fatalf("second Create: want ErrAlreadyExists, got %v", err)
	}
}
