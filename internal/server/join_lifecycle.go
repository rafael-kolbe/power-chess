package server

import (
	"fmt"
	"strings"
	"time"

	"power-chess/internal/gameplay"
)

// joinSeat returns pid if the seat is joinable while room mutex is held by Execute.
//
// When a disconnect grace timer is running and the seat has zero live connections, the seat is
// still "held" for the prior occupant: the same authenticated account may rejoin (e.g. F5).
// Guests (auth id 0) may reclaim without account binding. A different authenticated user cannot
// take the seat until the grace window ends.
func (r *RoomSession) joinSeat(pid gameplay.PlayerID, joiningAuthUID uint64) (gameplay.PlayerID, error) {
	if r.connectedByPlayer[pid] == 0 && r.disconnectTimers[pid] != nil {
		var seatAuth uint64
		if r.authUIDByPlayer != nil {
			seatAuth = r.authUIDByPlayer[pid]
		}
		if seatAuth != 0 && joiningAuthUID != seatAuth {
			return "", fmt.Errorf("seat reserved for the disconnected account; sign in as the same user to rejoin")
		}
	}
	return pid, nil
}

// assignJoinPlayer picks white/black/random seat for a join_match request.
func (r *RoomSession) assignJoinPlayer(p JoinMatchPayload, joiningAuthUID uint64) (gameplay.PlayerID, error) {
	if r.RoomPrivate && strings.TrimSpace(p.Password) != r.RoomPassword {
		return "", fmt.Errorf("invalid room password")
	}
	occA := r.connectedByPlayer[gameplay.PlayerA] > 0
	occB := r.connectedByPlayer[gameplay.PlayerB] > 0
	if occA && occB {
		return "", fmt.Errorf("room is full")
	}
	raw := strings.ToLower(strings.TrimSpace(p.PieceType))
	switch raw {
	case "white":
		if occA {
			return "", fmt.Errorf("white side is already occupied")
		}
		return r.joinSeat(gameplay.PlayerA, joiningAuthUID)
	case "black":
		if occB {
			return "", fmt.Errorf("black side is already occupied")
		}
		return r.joinSeat(gameplay.PlayerB, joiningAuthUID)
	case "random":
		if p.PlayerID == string(gameplay.PlayerA) && !occA {
			return r.joinSeat(gameplay.PlayerA, joiningAuthUID)
		}
		if p.PlayerID == string(gameplay.PlayerB) && !occB {
			return r.joinSeat(gameplay.PlayerB, joiningAuthUID)
		}
		if occA {
			return r.joinSeat(gameplay.PlayerB, joiningAuthUID)
		}
		if occB {
			return r.joinSeat(gameplay.PlayerA, joiningAuthUID)
		}
		if time.Now().UnixNano()%2 == 0 {
			return r.joinSeat(gameplay.PlayerA, joiningAuthUID)
		}
		return r.joinSeat(gameplay.PlayerB, joiningAuthUID)
	}
	// Backward-compatible fallback from old playerId payload.
	if p.PlayerID == string(gameplay.PlayerB) {
		if occB {
			return "", fmt.Errorf("black side is already occupied")
		}
		return r.joinSeat(gameplay.PlayerB, joiningAuthUID)
	}
	if occA {
		return "", fmt.Errorf("white side is already occupied")
	}
	return r.joinSeat(gameplay.PlayerA, joiningAuthUID)
}
