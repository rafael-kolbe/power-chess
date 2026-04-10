package server

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openAuthTestDB returns an in-memory SQLite database with the users schema and a matching AuthService.
func openAuthTestDB(t *testing.T) (*gorm.DB, *AuthService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	if err := db.AutoMigrate(&userModel{}); err != nil {
		t.Fatalf("auto migrate users: %v", err)
	}
	auth := NewAuthService(db, []byte("test-jwt-secret-ok"))
	return db, auth
}
