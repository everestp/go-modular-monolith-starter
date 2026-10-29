package service

import "testing"

func TestCreateUser(t *testing.T) {
	result, err := New().Create("user@example.com")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result == "" {
		t.Fatal("Create result should not be empty")
	}
}
