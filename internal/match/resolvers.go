package match

import (
	"power-chess/internal/gameplay"
	matchresolvers "power-chess/internal/match/resolvers"
	"power-chess/internal/match/resolvers/disruption"
	"power-chess/internal/match/resolvers/power"
	"power-chess/internal/match/resolvers/retribution"
)

// noopResolver applies no board or resource effects. Card **types** still drive reaction windows
// and queue rules; individual card text is not executed in this build.
type noopResolver struct{}

// RequiresTarget reports that the noop resolver never waits for target input.
func (noopResolver) RequiresTarget() bool { return false }

// Apply does nothing and always succeeds.
func (noopResolver) Apply(_ matchresolvers.ResolverEngine, _ gameplay.PlayerID, _ matchresolvers.EffectTarget) error {
	return nil
}

// DefaultResolvers registers a no-op effect for every catalog card so ignition and reaction
// resolution always find a resolver without executing card-specific rules.
// Cards with real implementations are registered explicitly below.
func DefaultResolvers() map[gameplay.CardID]EffectResolver {
	m := make(map[gameplay.CardID]EffectResolver)
	for _, def := range gameplay.InitialCardCatalog() {
		m[def.ID] = noopResolver{}
	}
	m[CardKnightTouch] = power.KnightTouchResolver{}
	m[CardBishopTouch] = power.BishopTouchResolver{}
	m[CardRookTouch] = power.RookTouchResolver{}
	m[CardEnergyGain] = power.EnergyGainResolver{}
	m[CardDoubleTurn] = power.DoubleTurnResolver{}
	m[CardExtinguish] = disruption.ExtinguishResolver{}
	m[CardManaBurn] = retribution.ManaBurnResolver{}
	return m
}
