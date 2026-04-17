package server

import (
	"time"

	"power-chess/internal/gameplay"
)

// ensureDisconnectBudgetMapsUnsafe initializes per-seat disconnect budget maps when nil.
func (r *RoomSession) ensureDisconnectBudgetMapsUnsafe() {
	total := r.effectiveDisconnectBudgetTotal()
	if r.disconnectBudgetRemaining == nil {
		r.disconnectBudgetRemaining = map[gameplay.PlayerID]time.Duration{
			gameplay.PlayerA: total,
			gameplay.PlayerB: total,
		}
	}
	if r.disconnectSegmentStart == nil {
		r.disconnectSegmentStart = map[gameplay.PlayerID]time.Time{}
	}
}

// effectiveDisconnectBudgetTotal returns the configured match-wide disconnect budget per player.
func (r *RoomSession) effectiveDisconnectBudgetTotal() time.Duration {
	if r.DisconnectBudgetTotal > 0 {
		return r.DisconnectBudgetTotal
	}
	if r.DisconnectGrace > 0 {
		return r.DisconnectGrace
	}
	return 60 * time.Second
}

// effectiveDisconnectMinWinDelay returns the minimum delay after disconnect before a disconnect win.
func (r *RoomSession) effectiveDisconnectMinWinDelay() time.Duration {
	if r.DisconnectMinWinDelay > 0 {
		return r.DisconnectMinWinDelay
	}
	return 5 * time.Second
}

// endDisconnectSegmentUnsafe subtracts wall time since disconnectSegmentStart[pid] from budget and clears the segment.
func (r *RoomSession) endDisconnectSegmentUnsafe(pid gameplay.PlayerID, now time.Time) {
	if r.disconnectSegmentStart == nil {
		return
	}
	t0, ok := r.disconnectSegmentStart[pid]
	if !ok || t0.IsZero() {
		return
	}
	spent := now.Sub(t0)
	r.disconnectBudgetRemaining[pid] -= spent
	if r.disconnectBudgetRemaining[pid] < 0 {
		r.disconnectBudgetRemaining[pid] = 0
	}
	r.disconnectSegmentStart[pid] = time.Time{}
}

// RegisterPlayerConnection marks player as connected and clears pending disconnect timeout.
func (r *RoomSession) RegisterPlayerConnection(pid gameplay.PlayerID) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	now := time.Now().UTC()
	r.lastActivity = now
	r.ensureDisconnectBudgetMapsUnsafe()
	r.endDisconnectSegmentUnsafe(pid, now)
	r.connectedByPlayer[pid]++
	if timer, ok := r.disconnectTimers[pid]; ok && timer != nil {
		timer.Stop()
		delete(r.disconnectTimers, pid)
	}
	delete(r.disconnectDeadline, pid)
	if r.connectedByPlayer[gameplay.PlayerA] > 0 && r.connectedByPlayer[gameplay.PlayerB] > 0 {
		r.resumeReactionDeadlineAfterReconnectUnsafe(now)
	}
}

// freezeReactionDeadlineForDisconnectUnsafe snapshots the reaction response deadline before pausing for a single-side disconnect.
func (r *RoomSession) freezeReactionDeadlineForDisconnectUnsafe(now time.Time) {
	r.disconnectFrozenReactionRemaining = 0
	if !r.reactionDeadline.IsZero() && now.Before(r.reactionDeadline) {
		r.disconnectFrozenReactionRemaining = r.reactionDeadline.Sub(now)
	}
	r.reactionDeadline = time.Time{}
}

// resumeReactionDeadlineAfterReconnectUnsafe restores the reaction deadline frozen during disconnect.
func (r *RoomSession) resumeReactionDeadlineAfterReconnectUnsafe(now time.Time) {
	if r.disconnectFrozenReactionRemaining > 0 {
		r.reactionDeadline = now.Add(r.disconnectFrozenReactionRemaining)
		r.disconnectFrozenReactionRemaining = 0
	}
}

// clearDisconnectFrozenUnsafe drops any in-memory disconnect freeze (match end, leave, or reset).
func (r *RoomSession) clearDisconnectFrozenUnsafe() {
	r.disconnectFrozenReactionRemaining = 0
}

// HandlePlayerDisconnect marks player as disconnected and applies timeout-based match ending rules.
func (r *RoomSession) HandlePlayerDisconnect(pid gameplay.PlayerID) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	r.lastActivity = time.Now().UTC()
	if r.connectedByPlayer[pid] > 0 {
		r.connectedByPlayer[pid]--
	}
	if r.connectedByPlayer[pid] == 0 {
		r.SetPlayerDisplayNameUnsafe(pid, "")
	}
	if r.matchEnded {
		return
	}
	aConnected := r.connectedByPlayer[gameplay.PlayerA] > 0
	bConnected := r.connectedByPlayer[gameplay.PlayerB] > 0
	if !aConnected && !bConnected {
		r.resetClientFxHoldUnsafe()
		r.clearDisconnectFrozenUnsafe()
		r.evaluateMatchOutcomeUnsafe()
		if !r.matchEnded {
			r.cancelMatchNoWinner()
		}
		return
	}
	if (pid == gameplay.PlayerA && bConnected) || (pid == gameplay.PlayerB && aConnected) {
		now := time.Now().UTC()
		r.flushClientFxHoldWallTimeUnsafe(now)
		r.freezeReactionDeadlineForDisconnectUnsafe(now)
		r.scheduleDisconnectTimeout(pid)
	}
}

// HandlePlayerLeave marks an intentional room exit and immediately awards win if opponent stays.
func (r *RoomSession) HandlePlayerLeave(pid gameplay.PlayerID) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	r.handlePlayerLeaveUnsafe(pid)
}

func (r *RoomSession) handlePlayerLeaveUnsafe(pid gameplay.PlayerID) {
	r.lastActivity = time.Now().UTC()
	r.resetClientFxHoldUnsafe()
	r.clearDisconnectFrozenUnsafe()
	if r.connectedByPlayer[pid] > 0 {
		r.connectedByPlayer[pid]--
	}
	if r.connectedByPlayer[pid] == 0 {
		r.SetPlayerDisplayNameUnsafe(pid, "")
	}
	if timer, ok := r.disconnectTimers[pid]; ok && timer != nil {
		timer.Stop()
		delete(r.disconnectTimers, pid)
	}
	delete(r.disconnectDeadline, pid)
	if r.disconnectSegmentStart != nil {
		r.disconnectSegmentStart[pid] = time.Time{}
	}
	if r.matchEnded {
		return
	}
	winner := oppositePlayer(pid)
	if r.connectedByPlayer[winner] > 0 {
		r.matchEnded = true
		r.winner = winner
		r.endReason = "left_room"
		r.startPostMatchWindowUnsafe()
		return
	}
	r.evaluateMatchOutcomeUnsafe()
	if !r.matchEnded {
		r.cancelMatchNoWinner()
	}
}

func (r *RoomSession) cancelMatchNoWinner() {
	r.clearDisconnectFrozenUnsafe()
	r.endReason = "both_disconnected_cancelled"
	r.matchEnded = true
	r.winner = ""
	r.startPostMatchWindowUnsafe()
	r.lastActivity = time.Now().UTC()
	for _, tm := range r.disconnectTimers {
		if tm != nil {
			tm.Stop()
		}
	}
	r.disconnectTimers = map[gameplay.PlayerID]*time.Timer{}
	r.disconnectDeadline = map[gameplay.PlayerID]time.Time{}
	if r.disconnectSegmentStart != nil {
		r.disconnectSegmentStart[gameplay.PlayerA] = time.Time{}
		r.disconnectSegmentStart[gameplay.PlayerB] = time.Time{}
	}
}

func (r *RoomSession) scheduleDisconnectTimeout(pid gameplay.PlayerID) {
	r.ensureDisconnectBudgetMapsUnsafe()
	if timer, ok := r.disconnectTimers[pid]; ok && timer != nil {
		timer.Stop()
	}
	now := time.Now()
	if r.disconnectDeadline == nil {
		r.disconnectDeadline = make(map[gameplay.PlayerID]time.Time)
	}
	budget := r.disconnectBudgetRemaining[pid]
	minD := r.effectiveDisconnectMinWinDelay()
	graceEnd := now.Add(minD)
	budgetEnd := now.Add(budget)
	winAt := graceEnd
	if budgetEnd.After(graceEnd) {
		winAt = budgetEnd
	}
	if budget <= 0 {
		winAt = graceEnd
	}
	r.disconnectSegmentStart[pid] = now
	r.disconnectDeadline[pid] = winAt
	dur := winAt.Sub(now)
	if dur < 0 {
		dur = 0
	}
	r.disconnectTimers[pid] = time.AfterFunc(dur, func() {
		r.stateM.Lock()
		defer r.stateM.Unlock()
		delete(r.disconnectDeadline, pid)
		if r.matchEnded || r.connectedByPlayer[pid] > 0 {
			return
		}
		winner := oppositePlayer(pid)
		if r.connectedByPlayer[winner] == 0 {
			return
		}
		r.endDisconnectSegmentUnsafe(pid, time.Now().UTC())
		r.matchEnded = true
		r.winner = winner
		r.endReason = "disconnect_timeout"
		r.startPostMatchWindowUnsafe()
		r.lastActivity = time.Now().UTC()
	})
}
