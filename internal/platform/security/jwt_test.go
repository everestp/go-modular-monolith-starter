package security

import (
	"testing"
	"time"
)

func TestJWTManagerIssueAndValidate(t *testing.T) {
	t.Parallel()
	manager := NewJWTManager("01234567890123456789012345678901", "issuer", "audience")
	token, err := manager.Issue("user-123", time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	subject, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if subject != "user-123" {
		t.Fatalf("subject = %q, want user-123", subject)
	}
}

func TestJWTManagerRejectsExpiredAndWrongSecret(t *testing.T) {
	t.Parallel()
	manager := NewJWTManager("01234567890123456789012345678901", "issuer", "audience")
	manager.now = func() time.Time { return time.Unix(100, 0).UTC() }
	token, err := manager.Issue("user-123", time.Second)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	manager.now = func() time.Time { return time.Unix(102, 0).UTC() }
	if _, err := manager.Validate(token); err == nil {
		t.Fatal("Validate accepted an expired token")
	}

	other := NewJWTManager("another-secret-01234567890123456789", "issuer", "audience")
	if _, err := other.Validate(token); err == nil {
		t.Fatal("Validate accepted a token signed with another secret")
	}
}

func TestJWTManagerRequiresMatchingIssuerAndAudience(t *testing.T) {
	t.Parallel()
	signer := NewJWTManager("01234567890123456789012345678901", "wrong-issuer", "wrong-audience")
	token, err := signer.Issue("user-123", time.Minute)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	verifier := NewJWTManager("01234567890123456789012345678901", "issuer", "audience")
	if _, err := verifier.Validate(token); err == nil {
		t.Fatal("Validate accepted a token with mismatched issuer/audience")
	}
}
