package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

type authRegisterRequest struct {
	Username        string `json:"username"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	// Role intentionally omitted — always "user" on the server.
}

type authLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authUserResponse struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

type authTokenResponse struct {
	Token string           `json:"token"`
	User  authUserResponse `json:"user"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAuthError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

// bearerToken returns the JWT from Authorization: Bearer <token>.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(h) >= len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// handleAuthRegister creates a user with role "user" and returns a JWT.
func (s *Server) handleAuthRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if s.auth == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "auth_unavailable")
		return
	}
	var body authRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := ValidateRegistrationInput(body.Username, body.Email, body.Password, body.ConfirmPassword); err != nil {
		writeAuthError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := s.auth.RegisterUser(body.Username, body.Email, body.Password)
	if err != nil {
		if IsDuplicateUserError(err) {
			writeAuthError(w, http.StatusConflict, "username_or_email_taken")
			return
		}
		writeAuthError(w, http.StatusInternalServerError, "registration_failed")
		return
	}
	token, err := s.auth.IssueToken(user)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "token_failed")
		return
	}
	writeJSON(w, http.StatusCreated, authTokenResponse{
		Token: token,
		User: authUserResponse{
			ID: user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	})
}

// handleAuthLogin returns a JWT for valid email/password.
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if s.auth == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "auth_unavailable")
		return
	}
	var body authLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	user, err := s.auth.LoginWithEmail(body.Email, body.Password)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	token, err := s.auth.IssueToken(user)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "token_failed")
		return
	}
	writeJSON(w, http.StatusOK, authTokenResponse{
		Token: token,
		User: authUserResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
	})
}

// handleAuthMe returns the current user from the JWT.
func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAuthError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if s.auth == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "auth_unavailable")
		return
	}
	raw := bearerToken(r)
	if raw == "" {
		raw = r.URL.Query().Get("token")
	}
	claims, err := s.auth.ParseToken(raw)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "invalid_token")
		return
	}
	user, err := s.auth.UserByID(claims.UserID)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "user_not_found")
		return
	}
	writeJSON(w, http.StatusOK, authUserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	})
}
