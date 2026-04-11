package server

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	userRoleUser  = "user"
	userRoleAdmin = "admin"
)

var (
	usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,64}$`)
	emailPattern    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

// ErrEmailAlreadyRegistered is returned by RegisterUser when the normalized email is already in use.
var ErrEmailAlreadyRegistered = errors.New("email already registered")

// userModel is the persisted account row (GORM).
type userModel struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	Username     string `gorm:"size:64;uniqueIndex:idx_users_username;not null"`
	Email        string `gorm:"size:255;uniqueIndex:idx_users_email;not null"`
	PasswordHash string `gorm:"size:255;not null"`
	Role         string `gorm:"size:16;not null;default:user"`
	// LobbyDeckID is the deck selected for the next match (FK to user_decks.id, same user).
	LobbyDeckID *uint64 `gorm:"index:idx_users_lobby_deck"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TableName keeps a stable table name for migrations.
func (userModel) TableName() string { return "users" }

// AuthService registers users, verifies passwords, and issues JWTs.
type AuthService struct {
	db        *gorm.DB
	jwtSecret []byte
}

// NewAuthService returns a service using the given DB and HS256 secret.
func NewAuthService(db *gorm.DB, jwtSecret []byte) *AuthService {
	return &AuthService{db: db, jwtSecret: jwtSecret}
}

// ValidateRegistrationInput checks username, email, and password before persistence.
func ValidateRegistrationInput(username, email, password, confirmPassword string) error {
	u := strings.TrimSpace(username)
	if !usernamePattern.MatchString(u) {
		return errors.New("username must be 3–64 characters: letters, digits, underscore only")
	}
	e := strings.TrimSpace(strings.ToLower(email))
	if !emailPattern.MatchString(e) {
		return errors.New("invalid email address")
	}
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if password != confirmPassword {
		return errors.New("password and confirmation do not match")
	}
	return nil
}

// RegisterUser creates a user with role "user" (never from client input).
func (s *AuthService) RegisterUser(username, email, password string) (*userModel, error) {
	u := strings.TrimSpace(username)
	e := strings.TrimSpace(strings.ToLower(email))
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	var clash userModel
	if err := s.db.Where("email = ?", e).Take(&clash).Error; err == nil {
		return nil, ErrEmailAlreadyRegistered
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	user := userModel{
		Username:     u,
		Email:        e,
		PasswordHash: string(hash),
		Role:         userRoleUser,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// LoginWithEmail returns the user if email and password match.
func (s *AuthService) LoginWithEmail(email, password string) (*userModel, error) {
	e := strings.TrimSpace(strings.ToLower(email))
	var user userModel
	err := s.db.Where("email = ?", e).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("invalid email or password")
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, errors.New("invalid email or password")
	}
	return &user, nil
}

type jwtUserClaims struct {
	UserID   uint64 `json:"uid"`
	Username string `json:"usr"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// IssueToken returns a signed JWT for the given user.
func (s *AuthService) IssueToken(user *userModel) (string, error) {
	if user == nil {
		return "", errors.New("nil user")
	}
	now := time.Now()
	claims := jwtUserClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(s.jwtSecret)
}

// ParseToken validates a bearer token and returns claims.
func (s *AuthService) ParseToken(tokenString string) (*jwtUserClaims, error) {
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, errors.New("missing token")
	}
	var claims jwtUserClaims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return &claims, nil
}

// DeleteUserByID removes a user row (e.g. rollback after failed post-registration steps).
func (s *AuthService) DeleteUserByID(id uint64) error {
	return s.db.Delete(&userModel{}, id).Error
}

// UserByID loads a user by primary key.
func (s *AuthService) UserByID(id uint64) (*userModel, error) {
	var user userModel
	err := s.db.Where("id = ?", id).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// IsDuplicateUserError reports whether err is a unique-constraint violation on users.
func IsDuplicateUserError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique constraint")
}
