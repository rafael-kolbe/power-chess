package server

import (
	"time"

	"power-chess/internal/gameplay"
)

// shouldEvict reports whether an empty room can be removed after idleTTL or because the match ended.
func (r *RoomSession) shouldEvict(now time.Time, idleTTL time.Duration) bool {
	r.clientsM.RLock()
	n := len(r.clients)
	r.clientsM.RUnlock()
	if n > 0 {
		return false
	}
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if r.matchEnded {
		return true
	}
	return now.Sub(r.lastActivity) >= idleTTL
}

// shutdownTimers stops disconnect grace timers to avoid leaks after room eviction.
func (r *RoomSession) shutdownTimers() {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	for _, tm := range r.disconnectTimers {
		if tm != nil {
			tm.Stop()
		}
	}
	r.disconnectTimers = map[gameplay.PlayerID]*time.Timer{}
	r.disconnectDeadline = map[gameplay.PlayerID]time.Time{}
	r.mulliganDeadline = time.Time{}
}

// ShouldForceClosePostMatch reports if post-match idle deadline elapsed.
func (r *RoomSession) ShouldForceClosePostMatch(now time.Time) bool {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if !r.matchEnded || r.postMatchDeadline.IsZero() {
		return false
	}
	return !now.Before(r.postMatchDeadline)
}

// CloseAllClients terminates all room client sockets.
func (r *RoomSession) CloseAllClients() {
	r.clientsM.RLock()
	clients := make([]*Client, 0, len(r.clients))
	for c := range r.clients {
		clients = append(clients, c)
	}
	r.clientsM.RUnlock()
	for _, c := range clients {
		_ = c.conn.Close()
	}
}
