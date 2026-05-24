package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("StrongPassw0rd!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if !VerifyPassword("StrongPassw0rd!", hash) {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Fatal("expected wrong password to fail")
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("admin", "omo2026a"); err != nil {
		t.Fatalf("expected 8-character password with letters and digits: %v", err)
	}
	if err := ValidatePassword("admin", "12345678"); err == nil {
		t.Fatal("expected digits-only password to fail")
	}
	if err := ValidatePassword("admin", "admin1234"); err == nil {
		t.Fatal("expected password containing username to fail")
	}
	if err := ValidatePassword("admin", "abc1234"); err == nil {
		t.Fatal("expected short password to fail")
	}
}
