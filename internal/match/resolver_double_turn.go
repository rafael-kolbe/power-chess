package match

import "power-chess/internal/gameplay"

// doubleTurnResolver applies the "double-turn" card: grants the owner one extra move
// on the turn the ignition resolves.
type doubleTurnResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (doubleTurnResolver) RequiresTarget() bool { return false }

// Apply increments the owner's extra-move counter by one, allowing a second move this turn.
func (doubleTurnResolver) Apply(e *Engine, owner gameplay.PlayerID, _ EffectTarget) error {
	e.extraMovesRemaining[owner]++
	return nil
}
