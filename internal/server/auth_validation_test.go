package server

import "testing"

func TestValidateRegistrationInput(t *testing.T) {
	if err := ValidateRegistrationInput("ab", "a@b.co", "password1", "password1"); err == nil {
		t.Fatal("expected error for short username")
	}
	if err := ValidateRegistrationInput("valid_user", "not-an-email", "password1", "password1"); err == nil {
		t.Fatal("expected error for bad email")
	}
	if err := ValidateRegistrationInput("valid_user", "a@b.co", "short", "short"); err == nil {
		t.Fatal("expected error for short password")
	}
	if err := ValidateRegistrationInput("valid_user", "a@b.co", "password1", "password2"); err == nil {
		t.Fatal("expected error for mismatch")
	}
	if err := ValidateRegistrationInput("valid_user", "a@b.co", "password1", "password1"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
