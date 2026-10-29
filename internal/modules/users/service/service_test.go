package service

import (
	"context"
	"testing"

	"prod-go/internal/modules/users"
)

type userWriter struct {
	created *users.User
}

func (w *userWriter) Create(_ context.Context, user *users.User) (*users.User, error) {
	w.created = user
	return user, nil
}

func (w *userWriter) Update(_ context.Context, user *users.User) (*users.User, error) {
	return user, nil
}

func TestCreateUser(t *testing.T) {
	repository := &userWriter{}
	result, err := New(repository).Create(context.Background(), &users.User{
		Email: " User@Example.com ", Password: "hash",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result.Email != "user@example.com" || repository.created.Email != result.Email {
		t.Fatalf("Create did not normalize email: result=%q stored=%q", result.Email, repository.created.Email)
	}
}
