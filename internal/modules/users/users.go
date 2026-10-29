package users

import "time"

// User is the domain entity for the application identity.
type User struct {
	ID        string
	Email     string
	Password  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Reader exposes read access to users.
type Reader interface {
	GetByEmail(email string) (*User, error)
	GetByID(id string) (*User, error)
}

// Writer exposes user persistence operations.
type Writer interface {
	Create(user *User) (*User, error)
	Update(user *User) (*User, error)
}
