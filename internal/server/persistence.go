package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

// RoomStore persists and restores room runtime snapshots.
type RoomStore interface {
	SaveRoom(ctx context.Context, room *RoomSession) error
	LoadRoom(ctx context.Context, roomID string) (*RoomSession, bool, error)
	// DeleteRoom removes stored snapshot data when a room is evicted from memory.
	DeleteRoom(ctx context.Context, roomID string) error
	// NextRoomID returns the next numeric room id seed.
	NextRoomID(ctx context.Context) (int, error)
	// DeleteAllRooms removes all persisted rooms.
	DeleteAllRooms(ctx context.Context) error
}

type roomSnapshotModel struct {
	RoomID     string    `gorm:"primaryKey;size:120"`
	EngineJSON []byte    `gorm:"type:bytea;not null"`
	ServerJSON []byte    `gorm:"type:bytea;not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

type roomServerState struct {
	RoomName     string              `json:"roomName"`
	RoomPrivate  bool                `json:"roomPrivate"`
	RoomPassword string              `json:"roomPassword"`
	MatchEnded   bool                `json:"matchEnded"`
	Winner       gameplay.PlayerID   `json:"winner"`
	EndReason    string              `json:"endReason"`
	Seen         map[string]struct{} `json:"seen"`
	AuthUserA    uint64              `json:"authUserA,omitempty"`
	AuthUserB    uint64              `json:"authUserB,omitempty"`
	DeckMatchOK  bool                `json:"deckMatchOk,omitempty"`
}

// PostgresRoomStore stores room snapshots in PostgreSQL.
type PostgresRoomStore struct {
	db *gorm.DB
}

// NewPostgresRoomStoreFromEnv creates a Postgres store using DATABASE_URL and runs migrations.
func NewPostgresRoomStoreFromEnv() (*PostgresRoomStore, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, errors.New("DATABASE_URL is empty")
	}
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormLogger})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&roomSnapshotModel{}, &userModel{}, &userDeckModel{}); err != nil {
		return nil, err
	}
	return &PostgresRoomStore{db: db}, nil
}

// DB returns the underlying GORM handle for other services (e.g. auth).
func (s *PostgresRoomStore) DB() *gorm.DB {
	return s.db
}

// SaveRoom upserts the latest room snapshot.
func (s *PostgresRoomStore) SaveRoom(ctx context.Context, room *RoomSession) error {
	engineState := room.Engine.ExportState()
	engineRaw, err := json.Marshal(engineState)
	if err != nil {
		return err
	}
	var authA, authB uint64
	if room.authUIDByPlayer != nil {
		authA = room.authUIDByPlayer[gameplay.PlayerA]
		authB = room.authUIDByPlayer[gameplay.PlayerB]
	}
	serverRaw, err := json.Marshal(roomServerState{
		RoomName:     room.RoomName,
		RoomPrivate:  room.RoomPrivate,
		RoomPassword: room.RoomPassword,
		MatchEnded:   room.matchEnded,
		Winner:       room.winner,
		EndReason:    room.endReason,
		Seen:         room.seen,
		AuthUserA:    authA,
		AuthUserB:    authB,
		DeckMatchOK:  room.deckMatchInitialized,
	})
	if err != nil {
		return err
	}
	model := roomSnapshotModel{
		RoomID:     room.RoomID,
		EngineJSON: engineRaw,
		ServerJSON: serverRaw,
		UpdatedAt:  time.Now().UTC(),
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "room_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"engine_json", "server_json", "updated_at"}),
		}).
		Create(&model).Error
}

// LoadRoom restores a room session from persisted snapshot.
func (s *PostgresRoomStore) LoadRoom(ctx context.Context, roomID string) (*RoomSession, bool, error) {
	var model roomSnapshotModel
	err := s.db.WithContext(ctx).Where("room_id = ?", roomID).Take(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var engineState match.PersistedEngineState
	if err := json.Unmarshal(model.EngineJSON, &engineState); err != nil {
		return nil, false, err
	}
	engine, err := match.NewEngineFromState(engineState)
	if err != nil {
		return nil, false, err
	}
	var state roomServerState
	if err := json.Unmarshal(model.ServerJSON, &state); err != nil {
		return nil, false, err
	}
	room := newRoomSessionWithEngine(roomID, state.RoomName, engine)
	room.RoomPrivate = state.RoomPrivate
	room.RoomPassword = state.RoomPassword
	room.matchEnded = state.MatchEnded
	room.winner = state.Winner
	room.endReason = state.EndReason
	if state.Seen != nil {
		room.seen = state.Seen
	}
	room.authUIDByPlayer = map[gameplay.PlayerID]uint64{
		gameplay.PlayerA: state.AuthUserA,
		gameplay.PlayerB: state.AuthUserB,
	}
	// Persisted engine is authoritative; do not run MaybeRebuild again.
	room.deckMatchInitialized = true
	return room, true, nil
}

// DeleteRoom deletes the room row for the given id from PostgreSQL.
func (s *PostgresRoomStore) DeleteRoom(ctx context.Context, roomID string) error {
	return s.db.WithContext(ctx).Where("room_id = ?", roomID).Delete(&roomSnapshotModel{}).Error
}

// NextRoomID returns max numeric room_id + 1. Non-numeric legacy ids are ignored.
func (s *PostgresRoomStore) NextRoomID(ctx context.Context) (int, error) {
	type row struct {
		Next int `gorm:"column:next_id"`
	}
	var out row
	err := s.db.WithContext(ctx).Raw(`
		SELECT COALESCE(MAX(CAST(room_id AS INTEGER)), 0) + 1 AS next_id
		FROM room_snapshot_models
		WHERE room_id ~ '^[0-9]+$'
	`).Scan(&out).Error
	if err != nil {
		return 0, err
	}
	if out.Next <= 0 {
		return 1, nil
	}
	return out.Next, nil
}

// DeleteAllRooms truncates persisted room snapshots.
func (s *PostgresRoomStore) DeleteAllRooms(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&roomSnapshotModel{}).Error
}
