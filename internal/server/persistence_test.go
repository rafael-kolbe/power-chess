package server

import (
	"context"
	"encoding/json"
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
// TestRoomServerStateDisconnectFrozenJSON ensures disconnect clock freeze fields round-trip in server JSON.
func TestRoomServerStateDisconnectFrozenJSON(t *testing.T) {
	in := roomServerState{
		DisconnectFrozenMainMs:      42000,
		DisconnectFrozenMainFor:     "A",
		DisconnectFrozenCarryPaused: true,
		DisconnectFrozenReactionMs:  8000,
	}
	raw, err := json.Marshal(&in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out roomServerState
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.DisconnectFrozenMainMs != in.DisconnectFrozenMainMs ||
		out.DisconnectFrozenMainFor != in.DisconnectFrozenMainFor ||
		out.DisconnectFrozenCarryPaused != in.DisconnectFrozenCarryPaused ||
		out.DisconnectFrozenReactionMs != in.DisconnectFrozenReactionMs {
		t.Fatalf("round-trip mismatch: %+v vs %+v", out, in)
	}
}

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
