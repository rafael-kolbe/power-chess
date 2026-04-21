package match

import (
	"errors"
	"fmt"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	matchresolvers "power-chess/internal/match/resolvers"
)

// errPlayerHasUnresolvedPendingEffects is returned when a player tries another gameplay action
// while a target-dependent card effect (e.g. Zip Line) is still waiting for input.
var errPlayerHasUnresolvedPendingEffects = errors.New("must resolve pending card effect first")

// errIfPlayerHasUnresolvedPendingEffects blocks actions that would bypass a queued pending effect.
func (e *Engine) errIfPlayerHasUnresolvedPendingEffects(pid gameplay.PlayerID) error {
	if len(e.pendingEffects[pid]) > 0 {
		return errPlayerHasUnresolvedPendingEffects
	}
	return nil
}

// zipLineHasLegalDestination reports whether the piece at from can teleport to some empty square
// on the same rank without leaving the activating player's king in check.
func (e *Engine) zipLineHasLegalDestination(pid gameplay.PlayerID, from chess.Pos) bool {
	own := toColor(pid)
	for col := 0; col < 8; col++ {
		to := chess.Pos{Row: from.Row, Col: col}
		if to == from {
			continue
		}
		if !e.Chess.PieceAt(to).IsEmpty() {
			continue
		}
		cp := e.Chess.Clone()
		if err := cp.TeleportZipLine(from, to); err != nil {
			continue
		}
		if cp.IsCheck(own) {
			continue
		}
		return true
	}
	return false
}

// validateZipLineIgnitionTarget checks the ignition target square for Zip Line.
func (e *Engine) validateZipLineIgnitionTarget(pid gameplay.PlayerID, from chess.Pos) error {
	if !from.InBounds() {
		return errors.New("target piece out of board bounds")
	}
	own := toColor(pid)
	piece := e.Chess.PieceAt(from)
	if piece.IsEmpty() {
		return errors.New("target piece square is empty")
	}
	if piece.Color != own {
		return errors.New("target piece must belong to the activating player")
	}
	if piece.Type == chess.King {
		return errors.New("zip line cannot target the king")
	}
	if !e.zipLineHasLegalDestination(pid, from) {
		return errors.New("no valid zip line destination from that square")
	}
	return nil
}

// hasAnyZipLineTarget returns true if the player controls any non-king piece with at least one
// legal Zip Line destination on its rank.
func (e *Engine) hasAnyZipLineTarget(pid gameplay.PlayerID) bool {
	own := toColor(pid)
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			from := chess.Pos{Row: row, Col: col}
			p := e.Chess.PieceAt(from)
			if p.IsEmpty() || p.Color != own || p.Type == chess.King {
				continue
			}
			if e.zipLineHasLegalDestination(pid, from) {
				return true
			}
		}
	}
	return false
}

// ApplyZipLineTeleport validates a same-rank empty teleport, applies it on the live board, then
// ends the chess turn for the owner (consuming any Double Turn extra move state), matching the
// tail of applyMoveCore after a normal move.
func (e *Engine) ApplyZipLineTeleport(owner gameplay.PlayerID, from, to chess.Pos) error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
	if e.State.CurrentTurn != owner {
		return fmt.Errorf("zip line can only be resolved on your turn")
	}
	if e.Chess.Turn != toColor(owner) {
		return fmt.Errorf("chess turn out of sync for zip line")
	}
	if e.ReactionWindow != nil && e.ReactionWindow.Open {
		return fmt.Errorf("cannot resolve zip line while a reaction window is open")
	}
	own := toColor(owner)
	p := e.Chess.PieceAt(from)
	if p.IsEmpty() || p.Color != own || p.Type == chess.King {
		return matchresolvers.ErrEffectFailed
	}
	if from.Row != to.Row || from == to {
		return matchresolvers.ErrEffectFailed
	}
	if !e.Chess.PieceAt(to).IsEmpty() {
		return matchresolvers.ErrEffectFailed
	}
	cp := e.Chess.Clone()
	if err := cp.TeleportZipLine(from, to); err != nil {
		return matchresolvers.ErrEffectFailed
	}
	if cp.IsCheck(own) {
		return matchresolvers.ErrEffectFailed
	}

	e.pruneStaleMovementGrants()
	e.pruneStaleMindControlEffects()
	if err := e.Chess.TeleportZipLine(from, to); err != nil {
		return err
	}
	m := chess.Move{From: from, To: to}
	e.syncMindControlIgnitionLocksAfterMove(m)
	e.advanceMovementGrantPosition(owner, from, to)
	e.advanceMindControlPosition(owner, from, to)
	e.pruneStaleMovementGrants()
	e.pruneStaleMindControlEffects()

	delete(e.extraMovesRemaining, owner)
	delete(e.doubleTurnEffectTurnsLeft, owner)
	e.expireMovementGrantsAfterOwnerTurn(owner)
	e.expireMindControlEffectsAfterOwnerTurn(owner)
	if err := e.State.EndTurn(owner); err != nil {
		return err
	}
	next := e.State.CurrentTurn
	e.Chess.Turn = toColor(next)
	e.reconcileTurnState()
	if err := e.StartTurn(next); err != nil {
		return err
	}
	return nil
}
