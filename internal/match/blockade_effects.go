package match

import (
	"fmt"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// BlockadeEffect stores a temporary per-piece movement lock created by Blockade.
type BlockadeEffect struct {
	Owner               gameplay.PlayerID
	SourceCardID        gameplay.CardID
	Target              chess.Pos
	RemainingOwnerTurns int
}

// addBlockadeEffect appends or replaces a Blockade lock for the same owner and target.
func (e *Engine) addBlockadeEffect(owner gameplay.PlayerID, target chess.Pos, durationTurns int) {
	next := make([]BlockadeEffect, 0, len(e.blockadeEffects)+1)
	for _, fx := range e.blockadeEffects {
		if fx.Owner == owner && fx.Target == target {
			continue
		}
		next = append(next, fx)
	}
	next = append(next, BlockadeEffect{
		Owner:               owner,
		SourceCardID:        CardBlockade,
		Target:              target,
		RemainingOwnerTurns: durationTurns,
	})
	e.blockadeEffects = next
}

// blockadeMoveError reports whether a piece is locked by Blockade this turn.
func (e *Engine) blockadeMoveError(owner gameplay.PlayerID, from chess.Pos) error {
	e.pruneStaleBlockadeEffects()
	for _, fx := range e.blockadeEffects {
		if fx.Owner == owner && fx.Target == from && fx.RemainingOwnerTurns > 0 {
			return fmt.Errorf("Blockade prevents that piece from moving this turn")
		}
	}
	return nil
}

// expireBlockadeEffectsAfterOwnerTurn decrements durations for Blockade locks owned by pid.
func (e *Engine) expireBlockadeEffectsAfterOwnerTurn(pid gameplay.PlayerID) {
	next := make([]BlockadeEffect, 0, len(e.blockadeEffects))
	for _, fx := range e.blockadeEffects {
		if fx.Owner == pid {
			fx.RemainingOwnerTurns--
		}
		if fx.RemainingOwnerTurns > 0 {
			next = append(next, fx)
		}
	}
	e.blockadeEffects = next
}

// pruneStaleBlockadeEffects removes locks whose target square no longer holds an owner piece.
func (e *Engine) pruneStaleBlockadeEffects() {
	next := make([]BlockadeEffect, 0, len(e.blockadeEffects))
	for _, fx := range e.blockadeEffects {
		p := e.Chess.PieceAt(fx.Target)
		if p.IsEmpty() || p.Color != toColor(fx.Owner) {
			continue
		}
		next = append(next, fx)
	}
	e.blockadeEffects = next
}
