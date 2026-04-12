package server

import (
	"context"
	"testing"
)

type countingStore struct {
	saves int
}

func (c *countingStore) NextRoomID(context.Context) (int, error) { return 1, nil }
func (c *countingStore) LoadRoom(context.Context, string) (*RoomSession, bool, error) {
	return nil, false, nil
}
func (c *countingStore) SaveRoom(context.Context, *RoomSession) error {
	c.saves++
	return nil
}
func (c *countingStore) DeleteRoom(context.Context, string) error { return nil }
func (c *countingStore) DeleteAllRooms(context.Context) error      { return nil }

// TestPersistRoomSkipsWhenAdminDebugMatch ensures ADMIN_DEBUG_MATCH disables DB writes.
func TestPersistRoomSkipsWhenAdminDebugMatch(t *testing.T) {
	t.Setenv("ADMIN_DEBUG_MATCH", "1")
	st := &countingStore{}
	s := NewServerWithStore(st)
	room, err := NewRoomSession("99")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.persistRoom(context.Background(), room); err != nil {
		t.Fatal(err)
	}
	if st.saves != 0 {
		t.Fatalf("expected no SaveRoom when admin debug, got %d", st.saves)
	}
}

// TestPersistRoomCallsStoreWhenAdminDebugOff ensures normal persistence when debug is off.
func TestPersistRoomCallsStoreWhenAdminDebugOff(t *testing.T) {
	t.Setenv("ADMIN_DEBUG_MATCH", "0")
	st := &countingStore{}
	s := NewServerWithStore(st)
	room, err := NewRoomSession("98")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.persistRoom(context.Background(), room); err != nil {
		t.Fatal(err)
	}
	if st.saves != 1 {
		t.Fatalf("expected one SaveRoom, got %d", st.saves)
	}
}
