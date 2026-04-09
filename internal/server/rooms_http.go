package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const roomIdleEvictionTTL = 120 * time.Second

// handleListRooms responds with active (non-ended) rooms and occupancy for the lobby UI.
func (s *Server) handleListRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type listItem struct {
		RoomID          string `json:"roomId"`
		RoomName        string `json:"roomName"`
		RoomPrivate     bool   `json:"roomPrivate"`
		ConnectedA      int    `json:"connectedA"`
		ConnectedB      int    `json:"connectedB"`
		GameStarted     bool   `json:"gameStarted"`
		OccupiedByColor string `json:"occupiedByColor,omitempty"`
	}
	s.roomsM.RLock()
	out := make([]listItem, 0, len(s.rooms))
	for _, room := range s.rooms {
		snap := room.SnapshotSafe()
		if snap.MatchEnded {
			continue
		}
		item := listItem{
			RoomID:      snap.RoomID,
			RoomName:    snap.RoomName,
			RoomPrivate: snap.RoomPrivate,
			ConnectedA:  snap.ConnectedA,
			ConnectedB:  snap.ConnectedB,
			GameStarted: snap.GameStarted,
		}
		if snap.ConnectedA > 0 && snap.ConnectedB == 0 {
			item.OccupiedByColor = "White"
		}
		if snap.ConnectedB > 0 && snap.ConnectedA == 0 {
			item.OccupiedByColor = "Black"
		}
		out = append(out, item)
	}
	s.roomsM.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(map[string]any{"rooms": out})
}

// runRoomCleanupLoop periodically removes finished or idle empty rooms from memory and persistence.
func (s *Server) runRoomCleanupLoop() {
	t := time.NewTicker(120 * time.Second)
	defer t.Stop()
	for now := range t.C {
		s.cleanupStaleRooms(now)
	}
}

func (s *Server) cleanupStaleRooms(now time.Time) {
	var toRemove []string
	s.roomsM.RLock()
	for id, room := range s.rooms {
		if room.shouldEvict(now, roomIdleEvictionTTL) {
			toRemove = append(toRemove, id)
		}
	}
	s.roomsM.RUnlock()

	for _, id := range toRemove {
		s.roomsM.Lock()
		room, ok := s.rooms[id]
		if !ok || !room.shouldEvict(now, roomIdleEvictionTTL) {
			s.roomsM.Unlock()
			continue
		}
		delete(s.rooms, id)
		s.roomsM.Unlock()
		room.shutdownTimers()
		if s.store != nil {
			_ = s.store.DeleteRoom(context.Background(), id)
		}
	}
}
