package security

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	cfg := Argon2Config{Memory: 64 * 1024, Iterations: 3, Parallelism: 2, KeyLength: 32, Pepper: "pepper"}
	hash, err := HashPassword("super-secret-passphrase", cfg)
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	ok, err := VerifyPassword("super-secret-passphrase", hash, cfg)
	if err != nil {
		t.Fatalf("VerifyPassword error: %v", err)
	}
	if !ok {
		t.Fatal("expected password verification to succeed")
	}
}
