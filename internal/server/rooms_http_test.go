package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"power-chess/internal/gameplay"
)

// TestCleanupStaleRoomsRemovesIdleEmptyRoom evicts rooms with no websocket clients after idle TTL.
func TestCleanupStaleRoomsRemovesIdleEmptyRoom(t *testing.T) {
	s := NewServerWithStore(nil)
	room, err := NewRoomSession("idle-room")
	if err != nil {
		t.Fatalf("new room: %v", err)
	}
	room.stateM.Lock()
	room.lastActivity = time.Now().UTC().Add(-200 * time.Second)
	room.stateM.Unlock()
	s.roomsM.Lock()
	s.rooms["idle-room"] = room
	s.roomsM.Unlock()

	s.cleanupStaleRooms(time.Now())

	s.roomsM.RLock()
	_, ok := s.rooms["idle-room"]
	s.roomsM.RUnlock()
	if ok {
		t.Fatalf("expected idle empty room to be removed")
	}
}

// TestCleanupStaleRoomsRemovesEndedMatchWithNoClients evicts finished rooms once everyone left.
func TestCleanupStaleRoomsRemovesEndedMatchWithNoClients(t *testing.T) {
	s := NewServerWithStore(nil)
	room, err := NewRoomSession("done-room")
	if err != nil {
		t.Fatalf("new room: %v", err)
	}
	room.stateM.Lock()
	room.matchEnded = true
	room.endReason = "stalemate"
	room.stateM.Unlock()
	s.roomsM.Lock()
	s.rooms["done-room"] = room
	s.roomsM.Unlock()

	s.cleanupStaleRooms(time.Now())

	s.roomsM.RLock()
	_, ok := s.rooms["done-room"]
	s.roomsM.RUnlock()
	if ok {
		t.Fatalf("expected ended room with no clients to be removed")
	}
}

// TestHandleListRoomsSkipsEndedMatches ensures ended rooms are omitted from the lobby list.
func TestHandleListRoomsSkipsEndedMatches(t *testing.T) {
	s := NewServerWithStore(nil)

	open, err := NewRoomSession("open-room")
	if err != nil {
		t.Fatalf("new room: %v", err)
	}
	ended, err := NewRoomSession("ended-room")
	if err != nil {
		t.Fatalf("new room: %v", err)
	}
	ended.stateM.Lock()
	ended.matchEnded = true
	ended.stateM.Unlock()

	s.roomsM.Lock()
	s.rooms["open-room"] = open
	s.rooms["ended-room"] = ended
	s.roomsM.Unlock()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	s.handleListRooms(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var body struct {
		Rooms []struct {
			RoomID string `json:"roomId"`
		} `json:"rooms"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(body.Rooms) != 1 || body.Rooms[0].RoomID != "open-room" {
		t.Fatalf("expected only open-room, got %+v", body.Rooms)
	}
}

// TestSnapshotReflectsGameStartedWhenBothSidesConnected validates lobby vs playing transition fields.
func TestSnapshotReflectsGameStartedWhenBothSidesConnected(t *testing.T) {
	room, err := NewRoomSession("snap-room")
	if err != nil {
		t.Fatalf("new room: %v", err)
	}
	room.RegisterPlayerConnection(gameplay.PlayerA)
	s1 := room.SnapshotSafe()
	if s1.GameStarted || s1.ConnectedA != 1 || s1.ConnectedB != 0 {
		t.Fatalf("unexpected snapshot: %+v", s1)
	}
	room.RegisterPlayerConnection(gameplay.PlayerB)
	s2 := room.SnapshotSafe()
	if !s2.GameStarted || s2.ConnectedA != 1 || s2.ConnectedB != 1 {
		t.Fatalf("unexpected snapshot: %+v", s2)
	}
}
