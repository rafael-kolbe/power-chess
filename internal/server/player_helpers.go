package server

import "power-chess/internal/gameplay"

func oppositePlayer(pid gameplay.PlayerID) gameplay.PlayerID {
	if pid == gameplay.PlayerA {
		return gameplay.PlayerB
	}
	return gameplay.PlayerA
}
