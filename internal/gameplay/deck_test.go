package gameplay

import "testing"

func TestDefaultDeckPresetCardIDs(t *testing.T) {
	ids := DefaultDeckPresetCardIDs()
	if len(ids) != DefaultDeckSize {
		t.Fatalf("preset size %d, want %d", len(ids), DefaultDeckSize)
	}
	if err := ValidateDeckComposition(ids); err != nil {
		t.Fatalf("preset invalid: %v", err)
	}
}

func TestValidateDeckComposition(t *testing.T) {
	good := DefaultDeckPresetCardIDs()
	if err := ValidateDeckComposition(good); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
	bad := append([]CardID(nil), good[:len(good)-1]...)
	if err := ValidateDeckComposition(bad); err == nil {
		t.Fatal("expected error for 19 cards")
	}
	tooMany := append([]CardID(nil), good...)
	tooMany = append(tooMany, "energy-gain")
	if err := ValidateDeckComposition(tooMany); err == nil {
		t.Fatal("expected error for 21 cards")
	}
}

func TestDeckInstancesFromCardIDs(t *testing.T) {
	ids := DefaultDeckPresetCardIDs()
	inst, err := DeckInstancesFromCardIDs(ids)
	if err != nil {
		t.Fatalf("instances: %v", err)
	}
	if len(inst) != DefaultDeckSize {
		t.Fatalf("len %d", len(inst))
	}
}
