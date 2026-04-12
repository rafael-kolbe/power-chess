package server

import (
	"context"
)

// persistRoom saves the room to the configured store. When ADMIN_DEBUG_MATCH is enabled,
// persistence is skipped so test servers never write match state to the database.
func (s *Server) persistRoom(ctx context.Context, room *RoomSession) error {
	if s == nil || room == nil {
		return nil
	}
	if s.adminDebugMatch {
		return nil
	}
	return room.Persist(ctx, s.store)
}
