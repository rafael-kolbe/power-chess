package server

import (
	"context"
	"testing"
)

type fakeRoomStore struct {
	saveCalls int
	loadRoom  *RoomSession
	loadOK    bool
}

func (f *fakeRoomStore) SaveRoom(_ context.Context, _ *RoomSession) error {
	f.saveCalls++
	return nil
}

func (f *fakeRoomStore) LoadRoom(_ context.Context, _ string) (*RoomSession, bool, error) {
	return f.loadRoom, f.loadOK, nil
}

func (f *fakeRoomStore) DeleteRoom(_ context.Context, _ string) error {
	return nil
}

func (f *fakeRoomStore) NextRoomID(_ context.Context) (int, error) {
	return 1, nil
}

func (f *fakeRoomStore) DeleteAllRooms(_ context.Context) error {
	return nil
}

// TestRoomPersistCallsStore validates the persistence adapter integration point.
func TestRoomPersistCallsStore(t *testing.T) {
	room, err := NewRoomSession("persist-room")
	if err != nil {
		t.Fatalf("new room failed: %v", err)
	}
	store := &fakeRoomStore{}
	if err := room.Persist(context.Background(), store); err != nil {
		t.Fatalf("persist failed: %v", err)
	}
	if store.saveCalls != 1 {
		t.Fatalf("expected save to be called once")
	}
}

// TestGetOrCreateRoomLoadsPersistedRoom validates loading persisted room snapshots.
func TestGetOrCreateRoomLoadsPersistedRoom(t *testing.T) {
	loaded, err := NewRoomSession("loaded-room")
	if err != nil {
		t.Fatalf("new room failed: %v", err)
	}
	store := &fakeRoomStore{loadRoom: loaded, loadOK: true}
	s := NewServerWithStore(store)

	room, err := s.getOrCreateRoom("loaded-room", "", false, "")
	if err != nil {
		t.Fatalf("getOrCreateRoom failed: %v", err)
	}
	if room != loaded {
		t.Fatalf("expected persisted room instance to be returned")
	}
}
