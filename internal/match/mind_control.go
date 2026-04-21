package match

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// MindControlEffect tracks temporary ownership inversion on one board piece.
type MindControlEffect struct {
	Owner             gameplay.PlayerID
	SourceCardID      gameplay.CardID
	Target            chess.Pos
	OriginalColor     chess.Color
	RemainingTurnEnds int
}

// addMindControlEffect applies one control effect and stores it for automatic expiration.
func (e *Engine) addMindControlEffect(effect MindControlEffect) error {
	p := e.Chess.PieceAt(effect.Target)
	if p.IsEmpty() {
		return nil
	}
	controllerColor := toColor(effect.Owner)
	if p.Color == controllerColor || p.Type == chess.King || p.Type == chess.Queen {
		return nil
	}
	p.Color = controllerColor
	e.Chess.SetPiece(effect.Target, p)
	next := make([]MindControlEffect, 0, len(e.mindControlEffects)+1)
	for _, active := range e.mindControlEffects {
		if active.Owner == effect.Owner && active.SourceCardID == effect.SourceCardID && active.Target == effect.Target {
			continue
		}
		next = append(next, active)
	}
	next = append(next, effect)
	e.mindControlEffects = next
	return nil
}

// advanceMindControlPosition updates tracked coordinates when a controlled piece moves.
func (e *Engine) advanceMindControlPosition(pid gameplay.PlayerID, from, to chess.Pos) {
	for i := range e.mindControlEffects {
		mc := &e.mindControlEffects[i]
		if mc.Owner == pid && mc.Target == from {
			mc.Target = to
		}
	}
}

// expireMindControlEffectsAfterOwnerTurn decrements only control effects owned by pid once at
// the end of pid's turn and restores original color when the duration reaches zero.
func (e *Engine) expireMindControlEffectsAfterOwnerTurn(pid gameplay.PlayerID) {
	next := make([]MindControlEffect, 0, len(e.mindControlEffects))
	for _, mc := range e.mindControlEffects {
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
	e.mindControlEffects = next
}

// pruneStaleMindControlEffects removes effects whose tracked piece no longer belongs to controller.
func (e *Engine) pruneStaleMindControlEffects() {
	next := make([]MindControlEffect, 0, len(e.mindControlEffects))
	for _, mc := range e.mindControlEffects {
		p := e.Chess.PieceAt(mc.Target)
		if p.IsEmpty() || p.Color != toColor(mc.Owner) {
			continue
		}
		next = append(next, mc)
	}
	e.mindControlEffects = next
}

// CloneMindControlEffects returns a shallow copy for snapshot/persistence.
func (e *Engine) CloneMindControlEffects() []MindControlEffect {
	return append([]MindControlEffect(nil), e.mindControlEffects...)
}
