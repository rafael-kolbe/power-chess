package match

import "power-chess/internal/gameplay"

// noopResolver applies no board or resource effects. Card **types** still drive reaction windows
// and queue rules; individual card text is not executed in this build.
type noopResolver struct{}

func (noopResolver) RequiresTarget() bool { return false }

func (noopResolver) Apply(_ *Engine, _ gameplay.PlayerID, _ EffectTarget) error { return nil }

// DefaultResolvers registers a no-op effect for every catalog card so ignition and reaction
// resolution always find a resolver without executing card-specific rules.
func DefaultResolvers() map[gameplay.CardID]EffectResolver {
	m := make(map[gameplay.CardID]EffectResolver)
	for _, def := range gameplay.InitialCardCatalog() {
		m[def.ID] = noopResolver{}
	}
	m[CardKnightTouch] = knightTouchResolver{}
	m[CardBishopTouch] = bishopTouchResolver{}
	m[CardRookTouch] = rookTouchResolver{}
	m[CardEnergyGain] = energyGainResolver{}
	m[CardDoubleTurn] = doubleTurnResolver{}
	return m
}
