package match

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// PieceControlEffect tracks temporary ownership inversion on one board piece.
type PieceControlEffect struct {
	Owner             gameplay.PlayerID
	SourceCardID      gameplay.CardID
	Target            chess.Pos
	OriginalColor     chess.Color
	RemainingTurnEnds int
}

// addPieceControlEffect applies one control effect and stores it for automatic expiration.
// Callers are responsible for validating piece eligibility before calling this method.
func (e *Engine) addPieceControlEffect(effect PieceControlEffect) error {
	p := e.Chess.PieceAt(effect.Target)
	if p.IsEmpty() {
		return nil
	}
	controllerColor := toColor(effect.Owner)
	// Deduplicate: remove any existing effect for the same owner+card+target.
	next := make([]PieceControlEffect, 0, len(e.pieceControlEffects)+1)
	for _, active := range e.pieceControlEffects {
		if active.Owner == effect.Owner && active.SourceCardID == effect.SourceCardID && active.Target == effect.Target {
			continue
		}
		next = append(next, active)
	}
	p.Color = controllerColor
	e.Chess.SetPiece(effect.Target, p)
	next = append(next, effect)
	e.pieceControlEffects = next
	return nil
}

// advancePieceControlPosition updates tracked coordinates when a controlled piece moves.
func (e *Engine) advancePieceControlPosition(pid gameplay.PlayerID, from, to chess.Pos) {
	for i := range e.pieceControlEffects {
		mc := &e.pieceControlEffects[i]
		if mc.Owner == pid && mc.Target == from {
			mc.Target = to
		}
	}
}

// expirePieceControlEffectsAfterOwnerTurn decrements only control effects owned by pid once at
// the end of pid's turn and restores original color when the duration reaches zero.
func (e *Engine) expirePieceControlEffectsAfterOwnerTurn(pid gameplay.PlayerID) {
	next := make([]PieceControlEffect, 0, len(e.pieceControlEffects))
	for _, mc := range e.pieceControlEffects {
		if mc.Owner == pid {
			mc.RemainingTurnEnds--
			if mc.RemainingTurnEnds <= 0 {
				p := e.Chess.PieceAt(mc.Target)
				if !p.IsEmpty() && p.Color == toColor(mc.Owner) {
					p.Color = mc.OriginalColor
					e.Chess.SetPiece(mc.Target, p)
				}
				continue
			}
		}
		next = append(next, mc)
	}
	e.pieceControlEffects = next
}

// pruneStalePieceControlEffects removes effects whose tracked piece no longer belongs to controller.
func (e *Engine) pruneStalePieceControlEffects() {
	next := make([]PieceControlEffect, 0, len(e.pieceControlEffects))
	for _, mc := range e.pieceControlEffects {
		p := e.Chess.PieceAt(mc.Target)
		if p.IsEmpty() || p.Color != toColor(mc.Owner) {
			continue
		}
		next = append(next, mc)
	}
	e.pieceControlEffects = next
}

// ClonePieceControlEffects returns a shallow copy for snapshot/persistence.
func (e *Engine) ClonePieceControlEffects() []PieceControlEffect {
	return append([]PieceControlEffect(nil), e.pieceControlEffects...)
}
