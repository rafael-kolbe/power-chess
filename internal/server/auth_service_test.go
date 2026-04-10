package server

import (
	"strings"
	"testing"
)

func TestRegisterUserSetsRoleUser(t *testing.T) {
	_, auth := openAuthTestDB(t)
	u, err := auth.RegisterUser("player_one", "p1@example.com", "password1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if u.Role != userRoleUser {
		t.Fatalf("role: got %q want %q", u.Role, userRoleUser)
	}
	if u.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
}

func TestRegisterUserDuplicateUsername(t *testing.T) {
	_, auth := openAuthTestDB(t)
	if _, err := auth.RegisterUser("dup_user", "a@example.com", "password1"); err != nil {
		t.Fatalf("first register: %v", err)
	}
	_, err := auth.RegisterUser("dup_user", "b@example.com", "password2")
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
	if !IsDuplicateUserError(err) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestLoginWithEmailSuccess(t *testing.T) {
	_, auth := openAuthTestDB(t)
	if _, err := auth.RegisterUser("login_ok", "login@example.com", "password1"); err != nil {
		t.Fatalf("register: %v", err)
	}
	u, err := auth.LoginWithEmail("login@example.com", "password1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if u.Username != "login_ok" {
		t.Fatalf("username: %q", u.Username)
	}
}

func TestLoginWithEmailWrongPassword(t *testing.T) {
	_, auth := openAuthTestDB(t)
	if _, err := auth.RegisterUser("u2", "u2@example.com", "password1"); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, err := auth.LoginWithEmail("u2@example.com", "wrongpass")
	if err == nil {
		t.Fatal("expected login error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "invalid") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestIssueTokenAndParseToken(t *testing.T) {
	_, auth := openAuthTestDB(t)
	u, err := auth.RegisterUser("tok_user", "tok@example.com", "password1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	tok, err := auth.IssueToken(u)
	if err != nil || tok == "" {
		t.Fatalf("issue token: %v", err)
	}
	claims, err := auth.ParseToken(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != u.ID || claims.Username != u.Username || claims.Role != u.Role {
		t.Fatalf("claims mismatch: %+v vs user %+v", claims, u)
	}
}

func TestParseTokenRejectsGarbage(t *testing.T) {
	_, auth := openAuthTestDB(t)
	_, err := auth.ParseToken("not-a-jwt")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserByID(t *testing.T) {
	_, auth := openAuthTestDB(t)
	u, err := auth.RegisterUser("byid", "byid@example.com", "password1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	loaded, err := auth.UserByID(u.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if loaded.Email != u.Email {
		t.Fatalf("email mismatch")
	}
	_, err = auth.UserByID(99999)
	if err == nil {
		t.Fatal("expected not found")
	}
}
