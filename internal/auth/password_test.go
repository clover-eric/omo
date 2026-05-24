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
	if err := ValidatePassword("admin", "StrongPassw0rd!"); err != nil {
		t.Fatalf("expected strong password: %v", err)
	}
	if err := ValidatePassword("admin", "admin123"); err == nil {
		t.Fatal("expected weak password to fail")
	}
}
