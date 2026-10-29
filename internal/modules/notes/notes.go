package notes

import "time"

// Note is the resource managed by the API.
type Note struct {
	ID        string
	OwnerID   string
	Title     string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Repository describes persistence for notes.
type Repository interface {
	Create(note *Note) (*Note, error)
	GetByID(id, ownerID string) (*Note, error)
	List(ownerID string, limit int, cursor string) ([]Note, error)
	Update(note *Note) (*Note, error)
	Delete(id, ownerID string) error
}
