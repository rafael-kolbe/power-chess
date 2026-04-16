package server

import (
	"time"

	"power-chess/internal/gameplay"
)

// noteReactionChainStartedUnsafe clears the reaction timeout while a non-empty stack is resolving.
// Caller must hold r.stateM.
func (r *RoomSession) noteReactionChainStartedUnsafe() {
	now := time.Now()
	if !r.reactionDeadline.IsZero() {
		rem := r.reactionDeadline.Sub(now)
		if rem < 0 {
			rem = 0
		}
		if rem > 0 {
			r.reactionBudgetRemaining = rem
		}
	}
	r.reactionDeadline = time.Time{}
}

// ResolveReactionTimeoutIfExpired auto-resolves an open reaction window when timeout elapses.
func (r *RoomSession) ResolveReactionTimeoutIfExpired(now time.Time) (bool, error) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if r.clientFxHoldCount > 0 {
		return false, nil
	}
	if r.connectedByPlayer[gameplay.PlayerA] == 0 || r.connectedByPlayer[gameplay.PlayerB] == 0 {
		r.reactionDeadline = time.Time{}
		r.reactionBudgetRemaining = 0
		return false, nil
	}
	rw, stackSize, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open {
		r.reactionDeadline = time.Time{}
		r.reactionDeadlineFor = ""
		r.reactionBudgetRemaining = 0
		return false, nil
	}
	if r.reactionDeadline.IsZero() {
		// noteReactionChainStartedUnsafe clears the deadline when the first reaction is queued.
		// If we only arm reactionTimeout here, the room waits a full cycle even when the ignite
		// chain cannot be extended and ResolveReactionStack should run immediately (same as
		// maybeAutoFinalizeIgniteChainIfStuckUnsafe after queue_reaction).
		if stackSize > 0 && rw.Trigger == "ignite_reaction" {
			if err := r.maybeAutoFinalizeIgniteChainIfStuckUnsafe(); err != nil {
				return false, err
			}
			rw2, _, ok2 := r.Engine.ReactionWindowSnapshot()
			if !ok2 || !rw2.Open {
				r.reactionDeadline = time.Time{}
				r.reactionDeadlineFor = ""
				r.reactionBudgetRemaining = 0
				r.evaluateMatchOutcomeUnsafe()
				return true, nil
			}
		}
		if stackSize > 0 && rw.Trigger == "capture_attempt" {
			if err := r.maybeAutoFinalizeCounterChainIfStuckUnsafe(); err != nil {
				return false, err
			}
			rw2, _, ok2 := r.Engine.ReactionWindowSnapshot()
			if !ok2 || !rw2.Open {
				r.reactionDeadline = time.Time{}
				r.reactionDeadlineFor = ""
				r.reactionBudgetRemaining = 0
				r.evaluateMatchOutcomeUnsafe()
				return true, nil
			}
		}
		responder := r.currentReactionResponder()
		r.reactionDeadline = now.Add(r.reactionBudgetFor(responder))
		r.reactionDeadlineFor = responder
		return false, nil
	}
	if now.Before(r.reactionDeadline) {
		return false, nil
	}
	// Timeout expired — resolve immediately.
	if err := r.Engine.ResolveReactionStack(); err != nil {
		return false, err
	}
	r.reactionDeadline = time.Time{}
	r.reactionDeadlineFor = ""
	r.reactionBudgetRemaining = 0
	r.evaluateMatchOutcomeUnsafe()
	return true, nil
}

// reactionBudgetFor returns the remaining reaction budget for the given player.
// Falls back to reactionTimeout when the budget is zero.
func (r *RoomSession) reactionBudgetFor(pid gameplay.PlayerID) time.Duration {
	switch pid {
	case gameplay.PlayerA:
		if r.reactionBudgetA > 0 {
			return r.reactionBudgetA
		}
	case gameplay.PlayerB:
		if r.reactionBudgetB > 0 {
			return r.reactionBudgetB
		}
	}
	return r.reactionTimeout
}

// saveBudgetForPlayer saves remaining reaction time to the appropriate per-player budget field.
func (r *RoomSession) saveBudgetForPlayer(pid gameplay.PlayerID, rem time.Duration) {
	switch pid {
	case gameplay.PlayerA:
		r.reactionBudgetA = rem
	case gameplay.PlayerB:
		r.reactionBudgetB = rem
	}
}

// currentReactionResponder returns the player who should respond in the current reaction window.
func (r *RoomSession) currentReactionResponder() gameplay.PlayerID {
	rw, stackSize, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open {
		return ""
	}
	if stackSize == 0 {
		return oppositePlayer(rw.Actor)
	}
	if top, okTop := r.Engine.ReactionStackTopSnapshot(); okTop {
		return oppositePlayer(top.Owner)
	}
	return oppositePlayer(rw.Actor)
}

// NoteReactionChainExtendedUnsafe saves the former responder's remaining budget, clears the
// current deadline, and arms a new deadline for the new responder (after the card was queued).
// Call this under stateM after QueueReactionCard succeeds.
func (r *RoomSession) NoteReactionChainExtendedUnsafe(now time.Time) {
	// Save remaining time for whoever was on the clock.
	if !r.reactionDeadline.IsZero() && r.reactionDeadlineFor != "" {
		rem := r.reactionDeadline.Sub(now)
		if rem < 0 {
			rem = 0
		}
		r.saveBudgetForPlayer(r.reactionDeadlineFor, rem)
	}
	// Clear old deadline.
	r.reactionDeadline = time.Time{}
	r.reactionDeadlineFor = ""
	// Arm a new deadline for the new responder.
	newResponder := r.currentReactionResponder()
	if newResponder != "" {
		r.reactionDeadline = now.Add(r.reactionBudgetFor(newResponder))
		r.reactionDeadlineFor = newResponder
	}
}

// NoteReactionResolvedUnsafe saves the former responder's remaining budget when reactions are
// manually resolved (player clicks "resolve"). Call under stateM before clearing the deadline.
func (r *RoomSession) NoteReactionResolvedUnsafe(now time.Time) {
	if !r.reactionDeadline.IsZero() && r.reactionDeadlineFor != "" {
		rem := r.reactionDeadline.Sub(now)
		if rem < 0 {
			rem = 0
		}
		r.saveBudgetForPlayer(r.reactionDeadlineFor, rem)
	}
	r.reactionDeadline = time.Time{}
	r.reactionDeadlineFor = ""
}
