package validator

import "testing"

func TestMinLength(t *testing.T) {
	if !MinLength("abcdef", 5) {
		t.Fatal("expected minimum length check to pass")
	}
	if MinLength("abc", 5) {
		t.Fatal("expected minimum length check to fail")
	}
}

func TestIsEmail(t *testing.T) {
	if !IsEmail("name@example.com") {
		t.Fatal("valid email should pass")
	}
	if IsEmail("invalid") {
		t.Fatal("invalid email should fail")
	}
}
