package gameplay

import "errors"

// ZoneService centralizes card transitions across gameplay zones.
type ZoneService struct{}

// NewZoneService builds a zone service for card lifecycle operations.
func NewZoneService() ZoneService {
	return ZoneService{}
}

// MoveCardBetweenSlices removes one card from source index and appends it to destination.
func (ZoneService) MoveCardBetweenSlices(
	source []CardInstance,
	destination []CardInstance,
	sourceIndex int,
) (CardInstance, []CardInstance, []CardInstance, error) {
	if sourceIndex < 0 || sourceIndex >= len(source) {
		return CardInstance{}, source, destination, errors.New("invalid source index")
	}
	card := source[sourceIndex]
	nextSource := append(source[:sourceIndex], source[sourceIndex+1:]...)
	nextDestination := append(destination, card)
	return card, nextSource, nextDestination, nil
}

// SendCardToCooldown stores a card on cooldown or routes immediate destination when cooldown is zero.
func (z ZoneService) SendCardToCooldown(player *PlayerState, card CardInstance) {
	if card.Cooldown <= 0 {
		z.RouteFinishedCooldown(player, card)
		return
	}
	player.Cooldowns = append(player.Cooldowns, CooldownEntry{
		Card:           card,
		TurnsRemaining: card.Cooldown,
	})
}

// RouteFinishedCooldown moves continuous cards to banished and other cards back to deck.
func (ZoneService) RouteFinishedCooldown(player *PlayerState, card CardInstance) {
	def, ok := CardDefinitionByID(card.CardID)
	if ok && def.Type == CardTypeContinuous {
		player.Banished = append(player.Banished, card)
		return
	}
	player.Deck = append(player.Deck, card)
}
