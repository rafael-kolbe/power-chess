package server

import (
	"time"

	"power-chess/internal/gameplay"
)

// startMulliganDeadlineUnsafe sets the wall-clock instant when unconfirmed mulligan seats auto-keep.
// Caller must hold r.stateM.
func (r *RoomSession) startMulliganDeadlineUnsafe(now time.Time) {
	r.mulliganDeadline = now.Add(mulliganPhaseDuration)
}

// ResolveMulliganTimeoutIfExpired auto-confirms mulligan for any seat that has not locked in after the deadline.
func (r *RoomSession) ResolveMulliganTimeoutIfExpired(now time.Time) (bool, error) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if r.clientFxHoldCount > 0 {
		return false, nil
	}
	if r.matchEnded || !r.Engine.State.MulliganPhaseActive {
		r.mulliganDeadline = time.Time{}
		return false, nil
	}
	if r.mulliganDeadline.IsZero() || now.Before(r.mulliganDeadline) {
		return false, nil
	}
	s := r.Engine.State
	for _, pid := range []gameplay.PlayerID{gameplay.PlayerA, gameplay.PlayerB} {
		if s.MulliganConfirmed != nil && s.MulliganConfirmed[pid] {
			continue
		}
		done, err := s.ConfirmMulligan(pid, nil)
		if err != nil {
			return false, err
		}
		if done {
			if err := r.Engine.StartTurn(gameplay.PlayerA); err != nil {
				return false, err
			}
			break
		}
	}
	r.mulliganDeadline = time.Time{}
	if !r.Engine.State.MulliganPhaseActive {
		r.resetReactionBudgetsUnsafe()
	}
	r.lastActivity = now.UTC()
	return true, nil
}

func (r *RoomSession) resetReactionBudgetsUnsafe() {
	// Reset per-player reaction budgets for the new turn.
	r.reactionBudgetA = r.reactionTimeout
	r.reactionBudgetB = r.reactionTimeout
	r.reactionDeadlineFor = ""
	r.reactionBudgetRemaining = 0
}
