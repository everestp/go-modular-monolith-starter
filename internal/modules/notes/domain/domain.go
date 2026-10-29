// Package domain defines the note model.
package domain

// Note is the user-owned note entity.
type Note struct {
	ID      string
	Title   string
	Content string
	OwnerID string
}
