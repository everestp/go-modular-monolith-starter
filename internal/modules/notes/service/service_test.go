package service

import "testing"

func TestCreateNote(t *testing.T) {
	result, err := New().Create("title", "content")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result == "" {
		t.Fatal("Create result should not be empty")
	}
}
