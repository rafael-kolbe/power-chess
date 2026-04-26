package gameplay

import (
	"fmt"
)

// MaxSavedDecksPerUser is the maximum number of named decks a user may persist server-side.
const MaxSavedDecksPerUser = 10

// DefaultDeckDisplayName is the display name assigned to the auto-seeded deck for new accounts.
const DefaultDeckDisplayName = "Default"

// DefaultDeckPresetCardIDs returns the 20 card IDs for the seeded "Default" deck (order = deck order).
func DefaultDeckPresetCardIDs() []CardID {
	pattern := []struct {
		id     CardID
		copies int
	}{
		{"energy-gain", 3},
		{"knight-touch", 2},
		{"bishop-touch", 2},
		{"rook-touch", 2},
		{"sacrifice-of-the-masses", 1},
		{"backstab", 2},
		{"extinguish", 2},
		{"clairvoyance", 1},
		{"save-it-for-later", 1},
		{"retaliate", 2},
		{"counterattack", 1},
		{"archmage-arsenal", 1},
	}
	out := make([]CardID, 0, DefaultDeckSize)
	for _, e := range pattern {
		for i := 0; i < e.copies; i++ {
			out = append(out, e.id)
		}
	}
	return out
}

// ValidateDeckComposition checks that cardIDs forms a legal 20-card deck for constructed play.
func ValidateDeckComposition(cardIDs []CardID) error {
	if len(cardIDs) != DefaultDeckSize {
		return fmt.Errorf("deck must contain exactly %d cards", DefaultDeckSize)
	}
	counts := map[CardID]int{}
	for _, id := range cardIDs {
		def, ok := CardDefinitionByID(id)
		if !ok {
			return fmt.Errorf("unknown card id %q", id)
		}
		counts[id]++
		if counts[id] > def.Limit {
			return fmt.Errorf("too many copies of %q (max %d)", id, def.Limit)
		}
	}
	return nil
}

// DeckInstancesFromCardIDs builds concrete card instances from an ordered list of card IDs.
func DeckInstancesFromCardIDs(cardIDs []CardID) ([]CardInstance, error) {
	if err := ValidateDeckComposition(cardIDs); err != nil {
		return nil, err
	}
	out := make([]CardInstance, 0, len(cardIDs))
	for i, id := range cardIDs {
		def, ok := CardDefinitionByID(id)
		if !ok {
			return nil, fmt.Errorf("unknown card id %q", id)
		}
		out = append(out, CardInstance{
			InstanceID: deckInstanceID(i + 1),
			CardID:     def.ID,
			ManaCost:   def.Cost,
			Ignition:   def.Ignition,
			Cooldown:   def.Cooldown,
		})
	}
	return out, nil
}

// deckInstanceID returns a stable instance id for deck ordering (c1..c20).
func deckInstanceID(n int) string {
	return "c" + itoa(n)
}

