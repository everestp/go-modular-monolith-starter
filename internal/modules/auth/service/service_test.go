package service

import "testing"

func TestRegister(t *testing.T) {
	result, err := New().Register("user@example.com", "password123456")
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if result == "" {
		t.Fatal("Register result should not be empty")
	}
}
