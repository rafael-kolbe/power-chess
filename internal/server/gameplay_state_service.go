package server

import "power-chess/internal/gameplay"

// GameplayStateService centralizes mutable gameplay state transitions for a room session.
type GameplayStateService struct {
	room *RoomSession
}

// NewGameplayStateService creates a room-bound gameplay state service.
func NewGameplayStateService(room *RoomSession) GameplayStateService {
	return GameplayStateService{room: room}
}

// IsOpen reports whether the current match is still open.
func (s GameplayStateService) IsOpen() bool {
	return s.room != nil && !s.room.matchEnded
}

// Close marks the match as ended with winner/reason and starts post-match timing.
func (s GameplayStateService) Close(winner gameplay.PlayerID, reason string) {
	if s.room == nil {
		return
	}
	s.room.matchEnded = true
	s.room.winner = winner
	s.room.endReason = reason
	s.room.startPostMatchWindowUnsafe()
}
