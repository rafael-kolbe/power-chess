package match

import (
	"errors"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	matchresolvers "power-chess/internal/match/resolvers"
)

// PendingCaptureAttempt returns the capture move currently waiting on a Counter window.
func (e *Engine) PendingCaptureAttempt() (matchresolvers.CaptureAttempt, bool) {
	if e.pendingMove == nil {
		return matchresolvers.CaptureAttempt{}, false
	}
	return matchresolvers.CaptureAttempt{
		Actor: e.pendingMove.PlayerID,
		From:  e.pendingMove.Move.From,
		To:    e.pendingMove.Move.To,
	}, true
}

// PendingCaptureAttackerHasActivePowerEffect reports whether the pending attacker has a live Power buff.
func (e *Engine) PendingCaptureAttackerHasActivePowerEffect() bool {
	if e.pendingMove == nil {
		return false
	}
	return e.pieceHasActivePowerEffect(e.pendingMove.PlayerID, e.pendingMove.Move.From)
}

// ResolveCounterattack captures the pending attacking piece instead of applying its capture.
func (e *Engine) ResolveCounterattack(owner gameplay.PlayerID) error {
	if e.consumePendingAttackerRemovalNegation() {
		return matchresolvers.ErrEffectFailed
	}
	if e.pendingMove == nil {
		return matchresolvers.ErrEffectFailed
	}
	if owner == e.pendingMove.PlayerID {
		return matchresolvers.ErrEffectFailed
	}
	if !e.PendingCaptureAttackerHasActivePowerEffect() {
		return matchresolvers.ErrEffectFailed
	}
	from := e.pendingMove.Move.From
	attacker := e.Chess.PieceAt(from)
	if attacker.IsEmpty() || attacker.Color != toColor(e.pendingMove.PlayerID) {
		return matchresolvers.ErrEffectFailed
	}
	ref, ok := pieceRefFromChessPiece(attacker)
	if !ok {
		return matchresolvers.ErrEffectFailed
	}
	e.Chess.SetPiece(from, chess.Piece{})
	e.State.AddToGraveyard(owner, ref)
	e.pendingCaptureOutcome = captureOutcomeCancelEndTurn
	e.pruneStaleMovementGrants()
	e.pruneStaleBlockadeEffects()
	e.pruneStalePieceControlEffects()
	e.clearIgnitionLocksAt(from)
	return nil
}

// ResolveBlockade negates the lower Counter that would remove the attacker and locks that piece.
func (e *Engine) ResolveBlockade(owner gameplay.PlayerID) error {
	if !e.CanBlockadePendingAttackerRemoval(owner) {
		return matchresolvers.ErrEffectFailed
	}
	from := e.pendingMove.Move.From
	e.pendingAttackerRemovalNegations++
	e.pendingCaptureOutcome = captureOutcomeCancelKeepTurn
	e.addBlockadeEffect(owner, from, 1)
	return nil
}

// CanBlockadePendingAttackerRemoval reports whether owner may use Blockade in the current chain.
func (e *Engine) CanBlockadePendingAttackerRemoval(owner gameplay.PlayerID) bool {
	if e.pendingMove == nil || owner != e.pendingMove.PlayerID {
		return false
	}
	top, ok := e.reactions.Top()
	if !ok || top.Owner == owner {
		return false
	}
	return counterRemovesPendingAttacker(top.Card.CardID)
}

// validateCounterPrerequisite checks explicit card text prerequisites before reaction costs are paid.
func (e *Engine) validateCounterPrerequisite(pid gameplay.PlayerID, cardID gameplay.CardID) error {
	if e.ReactionWindow == nil || e.ReactionWindow.Trigger != "capture_attempt" {
		return nil
	}
	switch cardID {
	case CardCounterattack:
		if !e.PendingCaptureAttackerHasActivePowerEffect() {
			return errors.New("Counterattack requires the attacking piece to have an active Power effect")
		}
	case CardBlockade:
		if !e.CanBlockadePendingAttackerRemoval(pid) {
			return errors.New("Blockade requires an opponent Counter that would capture the attacking piece")
		}
	}
	return nil
}

// pieceHasActivePowerEffect reports whether owner has a live Power card effect on target.
func (e *Engine) pieceHasActivePowerEffect(owner gameplay.PlayerID, target chess.Pos) bool {
	e.pruneStaleMovementGrants()
	for _, grant := range e.movementGrants {
		if grant.Owner != owner || grant.Target != target || grant.RemainingOwnerTurns <= 0 {
			continue
		}
		def, ok := gameplay.CardDefinitionByID(grant.SourceCardID)
		if ok && def.Type == gameplay.CardTypePower {
			return true
		}
	}
	return false
}

// consumePendingAttackerRemovalNegation consumes one Blockade negation if present.
func (e *Engine) consumePendingAttackerRemovalNegation() bool {
	if e.pendingAttackerRemovalNegations <= 0 {
		return false
	}
	e.pendingAttackerRemovalNegations--
	return true
}

// counterRemovesPendingAttacker identifies Counters whose successful effect removes the attacker.
func counterRemovesPendingAttacker(cardID gameplay.CardID) bool {
	return cardID == CardCounterattack
}

// clearIgnitionLocksAt removes target locks for pieces removed by a Counter effect.
func (e *Engine) clearIgnitionLocksAt(pos chess.Pos) {
	for owner, targets := range e.ignitionTargets {
		next := targets[:0]
		for _, target := range targets {
			if target != pos {
				next = append(next, target)
			}
		}
		if len(next) == 0 {
			e.ignitionTargets[owner] = nil
			delete(e.ignitionTargetCard, owner)
			continue
		}
		e.ignitionTargets[owner] = append([]chess.Pos(nil), next...)
	}
}
