package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAuthRegisterAndMe(t *testing.T) {
	_, auth := openAuthTestDB(t)
	srv := NewServerWithStore(nil)
	srv.auth = auth

	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	body := map[string]string{
		"username":         "http_user",
		"email":            "http@example.com",
		"password":         "password1",
		"confirm_password": "password1",
	}
	raw, _ := json.Marshal(body)
	res, err := http.Post(ts.URL+"/api/auth/register", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post register: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register status %d", res.StatusCode)
	}
	var reg authTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&reg); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	if reg.Token == "" || reg.User.Username != "http_user" || reg.User.Role != userRoleUser {
		t.Fatalf("unexpected register body: %+v", reg)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+reg.Token)
	rr := httptest.NewRecorder()
	srv.handleAuthMe(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("me status %d body %s", rr.Code, rr.Body.String())
	}
	var me authUserResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.Username != "http_user" || me.Email != "http@example.com" {
		t.Fatalf("me: %+v", me)
	}
}

func TestHandleAuthLoginReturnsToken(t *testing.T) {
	_, auth := openAuthTestDB(t)
	srv := NewServerWithStore(nil)
	srv.auth = auth

	if _, err := auth.RegisterUser("log_http", "lh@example.com", "password1"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	body := map[string]string{"email": "lh@example.com", "password": "password1"}
	raw, _ := json.Marshal(body)
	res, err := http.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status %d", res.StatusCode)
	}
	var out authTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Token == "" || out.User.Username != "log_http" {
		t.Fatalf("unexpected %+v", out)
	}
}

func TestHandleAuthMeRejectsBadToken(t *testing.T) {
	_, auth := openAuthTestDB(t)
	srv := NewServerWithStore(nil)
	srv.auth = auth

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer bad")
	rr := httptest.NewRecorder()
	srv.handleAuthMe(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected401, got %d", rr.Code)
	}
}

func TestHandleAuthRegisterConflictDuplicate(t *testing.T) {
	_, auth := openAuthTestDB(t)
	srv := NewServerWithStore(nil)
	srv.auth = auth

	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	reg := func(email string) int {
		body := map[string]string{
			"username":         "same_name",
			"email":            email,
			"password":         "password1",
			"confirm_password": "password1",
		}
		raw, _ := json.Marshal(body)
		res, err := http.Post(ts.URL+"/api/auth/register", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if c := reg("first@example.com"); c != http.StatusCreated {
		t.Fatalf("first register: %d", c)
	}
	if c := reg("second@example.com"); c != http.StatusConflict {
		t.Fatalf("second register want409, got %d", c)
	}
}
