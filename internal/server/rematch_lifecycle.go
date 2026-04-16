package server

import (
	"fmt"
	"time"

	"power-chess/internal/gameplay"
)

// StayInRoomAfterMatch keeps room open with single connected player waiting for opponent.
func (r *RoomSession) StayInRoomAfterMatch(pid gameplay.PlayerID) error {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if !r.matchEnded {
		return fmt.Errorf("match is not finished")
	}
	if r.connectedByPlayer[pid] == 0 {
		return fmt.Errorf("player is not connected")
	}
	total := r.connectedByPlayer[gameplay.PlayerA] + r.connectedByPlayer[gameplay.PlayerB]
	if total != 1 {
		return fmt.Errorf("stay action requires exactly one connected player")
	}
	if err := r.resetForNewMatchUnsafe(); err != nil {
		return err
	}
	r.connectedByPlayer[pid] = 1
	r.connectedByPlayer[oppositePlayer(pid)] = 0
	r.lastActivity = time.Now().UTC()
	return nil
}

// RequestRematch records rematch vote and resets board when both players accept.
func (r *RoomSession) RequestRematch(pid gameplay.PlayerID) (bool, error) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if !r.matchEnded {
		return false, fmt.Errorf("match is not finished")
	}
	if r.connectedByPlayer[pid] == 0 {
		return false, fmt.Errorf("player is not connected")
	}
	total := r.connectedByPlayer[gameplay.PlayerA] + r.connectedByPlayer[gameplay.PlayerB]
	if total < 2 {
		return false, fmt.Errorf("rematch requires both players connected")
	}
	r.rematchVotes[pid] = true
	r.lastActivity = time.Now().UTC()
	if r.rematchVotes[gameplay.PlayerA] && r.rematchVotes[gameplay.PlayerB] {
		r.swapConnectedPlayerSidesUnsafe()
		if err := r.resetForNewMatchUnsafe(); err != nil {
			r.rematchVotes = map[gameplay.PlayerID]bool{}
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// swapConnectedPlayerSidesUnsafe swaps connected players between A/B before rematch reset.
func (r *RoomSession) swapConnectedPlayerSidesUnsafe() {
	for client := range r.clients {
		client.playerID = oppositePlayer(client.playerID)
	}
	for key, pid := range r.Players {
		r.Players[key] = oppositePlayer(pid)
	}
	connectedA := r.connectedByPlayer[gameplay.PlayerA]
	connectedB := r.connectedByPlayer[gameplay.PlayerB]
	r.connectedByPlayer[gameplay.PlayerA] = connectedB
	r.connectedByPlayer[gameplay.PlayerB] = connectedA
	timerA, okTA := r.disconnectTimers[gameplay.PlayerA]
	timerB, okTB := r.disconnectTimers[gameplay.PlayerB]
	delete(r.disconnectTimers, gameplay.PlayerA)
	delete(r.disconnectTimers, gameplay.PlayerB)
	if okTB && timerB != nil {
		r.disconnectTimers[gameplay.PlayerA] = timerB
	}
	if okTA && timerA != nil {
		r.disconnectTimers[gameplay.PlayerB] = timerA
	}
	ddA, okDA := r.disconnectDeadline[gameplay.PlayerA]
	ddB, okDB := r.disconnectDeadline[gameplay.PlayerB]
	delete(r.disconnectDeadline, gameplay.PlayerA)
	delete(r.disconnectDeadline, gameplay.PlayerB)
	if okDB {
		r.disconnectDeadline[gameplay.PlayerA] = ddB
	}
	if okDA {
		r.disconnectDeadline[gameplay.PlayerB] = ddA
	}
	r.ensureDisconnectBudgetMapsUnsafe()
	ba := r.disconnectBudgetRemaining[gameplay.PlayerA]
	bb := r.disconnectBudgetRemaining[gameplay.PlayerB]
	r.disconnectBudgetRemaining[gameplay.PlayerA] = bb
	r.disconnectBudgetRemaining[gameplay.PlayerB] = ba
	if r.disconnectSegmentStart != nil {
		sa := r.disconnectSegmentStart[gameplay.PlayerA]
		sb := r.disconnectSegmentStart[gameplay.PlayerB]
		r.disconnectSegmentStart[gameplay.PlayerA] = sb
		r.disconnectSegmentStart[gameplay.PlayerB] = sa
	}
	nameA := r.displayNameByPlayer[gameplay.PlayerA]
	nameB := r.displayNameByPlayer[gameplay.PlayerB]
	r.displayNameByPlayer[gameplay.PlayerA] = nameB
	r.displayNameByPlayer[gameplay.PlayerB] = nameA
	if r.authUIDByPlayer != nil {
		aUID := r.authUIDByPlayer[gameplay.PlayerA]
		bUID := r.authUIDByPlayer[gameplay.PlayerB]
		r.authUIDByPlayer[gameplay.PlayerA] = bUID
		r.authUIDByPlayer[gameplay.PlayerB] = aUID
	}
}

func (r *RoomSession) resetForNewMatchUnsafe() error {
	if err := r.resetMatchEngineFromSavedDecksUnsafe(r.parent); err != nil {
		return err
	}
	r.matchEnded = false
	r.winner = ""
	r.endReason = ""
	r.postMatchDeadline = time.Time{}
	r.rematchVotes = map[gameplay.PlayerID]bool{}
	r.reactionDeadline = time.Time{}
	r.reactionDeadlineFor = ""
	r.reactionBudgetA = 0
	r.reactionBudgetB = 0
	r.reactionBudgetRemaining = 0
	r.mulliganDeadline = time.Time{}
	r.reactionModeByPlayer = map[gameplay.PlayerID]string{
		gameplay.PlayerA: ReactionModeOn,
		gameplay.PlayerB: ReactionModeOn,
	}
	for _, tm := range r.disconnectTimers {
		if tm != nil {
			tm.Stop()
		}
	}
	r.disconnectTimers = map[gameplay.PlayerID]*time.Timer{}
	r.disconnectDeadline = map[gameplay.PlayerID]time.Time{}
	total := r.effectiveDisconnectBudgetTotal()
	r.ensureDisconnectBudgetMapsUnsafe()
	r.disconnectBudgetRemaining[gameplay.PlayerA] = total
	r.disconnectBudgetRemaining[gameplay.PlayerB] = total
	r.disconnectSegmentStart[gameplay.PlayerA] = time.Time{}
	r.disconnectSegmentStart[gameplay.PlayerB] = time.Time{}
	r.clearDisconnectFrozenUnsafe()
	return nil
}
