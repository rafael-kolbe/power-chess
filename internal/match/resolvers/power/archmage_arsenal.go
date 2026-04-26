package power

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

const cardArchmageArsenal gameplay.CardID = "archmage-arsenal"

// ArchmageArsenalResolver searches the owner's deck for an eligible Power card and adds it to hand.
// Eligible targets: type Power, cost <= 3, card ID not "archmage-arsenal".
// When no legal targets exist, the pending effect resolves as a no-op success (TargetCard == nil).
type ArchmageArsenalResolver struct{}

// RequiresTarget reports that this resolver waits for a deck-search selection via resolve_pending_effect.
func (ArchmageArsenalResolver) RequiresTarget() bool { return true }

// Apply moves the chosen card from the owner's deck to hand and shuffles the remaining deck.
// Passing a nil TargetCard is the "no legal targets" confirmation and resolves with no effect.
func (ArchmageArsenalResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, target resolvers.EffectTarget) error {
	if target.TargetCard == nil {
		// Player confirmed an empty search result; no card moves.
		return nil
	}
	cardID := *target.TargetCard
	def, ok := gameplay.CardDefinitionByID(cardID)
	if !ok || def.Type != gameplay.CardTypePower || def.Cost > 3 || cardID == cardArchmageArsenal {
		return resolvers.ErrEffectFailed
	}
	if err := e.SearchDeckToHand(owner, cardID); err != nil {
		return resolvers.ErrEffectFailed
	}
	return nil
}
