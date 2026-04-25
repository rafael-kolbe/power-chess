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

// hasSameRankTeleportDestination reports whether the piece at from can teleport to some empty
// square on the same rank without leaving the activating player's king in check.
func (e *Engine) hasSameRankTeleportDestination(pid gameplay.PlayerID, from chess.Pos) bool {
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
		if err := applyNonKingEmptyTeleport(cp, from, to); err != nil {
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
		return errors.New(errTargetPieceOutOfBounds)
	}
	own := toColor(pid)
	piece := e.Chess.PieceAt(from)
	if piece.IsEmpty() {
		return errors.New(errTargetPieceSquareEmpty)
	}
	if piece.Color != own {
		return errors.New("target piece must belong to the activating player")
	}
	if piece.Type == chess.King {
		return errors.New("zip line cannot target the king")
	}
	if !e.hasSameRankTeleportDestination(pid, from) {
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
			if e.hasSameRankTeleportDestination(pid, from) {
				return true
			}
		}
	}
	return false
}

// ApplyPieceTeleport moves the piece at from to to subject to opts constraints, then optionally
// ends the owner's chess turn when opts.ConsumeTurn is true (consuming any Double Turn extra
// move state), matching the tail of applyMoveCore after a normal move.
func (e *Engine) ApplyPieceTeleport(owner gameplay.PlayerID, from, to chess.Pos, opts matchresolvers.TeleportOptions) error {
	if err := e.errIfOpeningBlocksGameplay(); err != nil {
		return err
	}
	if e.State.CurrentTurn != owner {
		return fmt.Errorf("teleport can only be resolved on your turn")
	}
	if e.Chess.Turn != toColor(owner) {
		return fmt.Errorf("chess turn out of sync for teleport")
	}
	if e.ReactionWindow != nil && e.ReactionWindow.Open {
		return fmt.Errorf("cannot resolve teleport while a reaction window is open")
	}
	if err := e.validatePieceTeleport(owner, from, to, opts); err != nil {
		return err
	}

	e.pruneStaleMovementGrants()
	e.pruneStalePieceControlEffects()
	if err := applyNonKingEmptyTeleport(e.Chess, from, to); err != nil {
		return err
	}
	m := chess.Move{From: from, To: to}
	e.syncPieceControlIgnitionLocksAfterMove(m)
	e.advanceMovementGrantPosition(owner, from, to)
	e.advancePieceControlPosition(owner, from, to)
	e.pruneStaleMovementGrants()
	e.pruneStalePieceControlEffects()

	if opts.ConsumeTurn {
		if err := e.consumeTeleportTurn(owner); err != nil {
			return err
		}
	}
	return nil
}

// validatePieceTeleport checks card teleport constraints and rejects teleports
// that would leave the owner's king in check.
func (e *Engine) validatePieceTeleport(owner gameplay.PlayerID, from, to chess.Pos, opts matchresolvers.TeleportOptions) error {
	if !from.InBounds() || !to.InBounds() {
		return matchresolvers.ErrEffectFailed
	}
	own := toColor(owner)
	piece := e.Chess.PieceAt(from)
	if piece.IsEmpty() || piece.Color != own {
		return matchresolvers.ErrEffectFailed
	}
	if opts.ForbidKing && piece.Type == chess.King {
		return matchresolvers.ErrEffectFailed
	}
	if from == to {
		return matchresolvers.ErrEffectFailed
	}
	if opts.RequireSameRow && from.Row != to.Row {
		return matchresolvers.ErrEffectFailed
	}
	if opts.RequireEmptyDestination && !e.Chess.PieceAt(to).IsEmpty() {
		return matchresolvers.ErrEffectFailed
	}

	cp := e.Chess.Clone()
	if err := applyNonKingEmptyTeleport(cp, from, to); err != nil {
		return matchresolvers.ErrEffectFailed
	}
	if cp.IsCheck(own) {
		return matchresolvers.ErrEffectFailed
	}
	return nil
}

// consumeTeleportTurn ends the owner's turn after a turn-consuming teleport and
// synchronizes chess and match turn state for the next player.
func (e *Engine) consumeTeleportTurn(owner gameplay.PlayerID) error {
	delete(e.extraMovesRemaining, owner)
	delete(e.doubleTurnEffectTurnsLeft, owner)
	e.expireMovementGrantsAfterOwnerTurn(owner)
	e.expirePieceControlEffectsAfterOwnerTurn(owner)
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

// applyNonKingEmptyTeleport moves a non-king piece to an empty square without
// advancing chess turn state. It is used by card effects whose destination
// pattern is validated by the match engine instead of core chess movement.
func applyNonKingEmptyTeleport(g *chess.Game, from, to chess.Pos) error {
	if !from.InBounds() || !to.InBounds() {
		return fmt.Errorf("teleport out of bounds")
	}
	if from == to {
		return fmt.Errorf("teleport requires different squares")
	}
	piece := g.PieceAt(from)
	if piece.IsEmpty() {
		return fmt.Errorf("no piece on teleport source")
	}
	if piece.Type == chess.King {
		return fmt.Errorf("teleport cannot move the king")
	}
	if !g.PieceAt(to).IsEmpty() {
		return fmt.Errorf("teleport destination must be empty")
	}

	g.EnPassant = chess.EnPassantState{}
	g.SetPiece(from, chess.Piece{})
	if piece.Type == chess.Pawn && (to.Row == 0 || to.Row == 7) {
		piece.Type = chess.Queen
	}
	g.SetPiece(to, piece)
	clearCastlingRightsAfterTeleport(g, from, piece)
	return nil
}

// clearCastlingRightsAfterTeleport mirrors the castling-right side effect of a
// rook leaving its initial square during a card-driven teleport.
func clearCastlingRightsAfterTeleport(g *chess.Game, from chess.Pos, moved chess.Piece) {
	if moved.Type != chess.Rook {
		return
	}
	switch from {
	case (chess.Pos{Row: 7, Col: 0}):
		g.CastlingRights.WhiteQueenSide = false
	case (chess.Pos{Row: 7, Col: 7}):
		g.CastlingRights.WhiteKingSide = false
	case (chess.Pos{Row: 0, Col: 0}):
		g.CastlingRights.BlackQueenSide = false
	case (chess.Pos{Row: 0, Col: 7}):
		g.CastlingRights.BlackKingSide = false
	}
}
