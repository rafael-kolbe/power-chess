package server

import (
	"fmt"
	"strings"
	"time"

	"power-chess/internal/gameplay"
)

// joinSeat returns pid if that seat is free for a new connection, or an error while a reconnect grace timer is waiting for the same seat.
func (r *RoomSession) joinSeat(pid gameplay.PlayerID) (gameplay.PlayerID, error) {
	if r.connectedByPlayer[pid] == 0 && r.disconnectTimers[pid] != nil {
		return "", fmt.Errorf("waiting for disconnected player to reconnect")
	}
	return pid, nil
}

func (r *RoomSession) assignJoinPlayer(p JoinMatchPayload) (gameplay.PlayerID, error) {
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
		return r.joinSeat(gameplay.PlayerA)
	case "black":
		if occB {
			return "", fmt.Errorf("black side is already occupied")
		}
		return r.joinSeat(gameplay.PlayerB)
	case "random":
		if p.PlayerID == string(gameplay.PlayerA) && !occA {
			return r.joinSeat(gameplay.PlayerA)
		}
		if p.PlayerID == string(gameplay.PlayerB) && !occB {
			return r.joinSeat(gameplay.PlayerB)
		}
		if occA {
			return r.joinSeat(gameplay.PlayerB)
		}
		if occB {
			return r.joinSeat(gameplay.PlayerA)
		}
		if time.Now().UnixNano()%2 == 0 {
			return r.joinSeat(gameplay.PlayerA)
		}
		return r.joinSeat(gameplay.PlayerB)
	}
	// Backward-compatible fallback from old playerId payload.
	if p.PlayerID == string(gameplay.PlayerB) {
		if occB {
			return "", fmt.Errorf("black side is already occupied")
		}
		return r.joinSeat(gameplay.PlayerB)
	}
	if occA {
		return "", fmt.Errorf("white side is already occupied")
	}
	return r.joinSeat(gameplay.PlayerA)
}
