package gameplay

import "testing"

func TestInitialCardCatalogUsesDefaultLimit(t *testing.T) {
	catalog := InitialCardCatalog()
	if len(catalog) == 0 {
		t.Fatalf("catalog must not be empty")
	}
	for _, c := range catalog {
		if c.Limit != DefaultCardLimit {
			t.Fatalf("card %s has invalid limit %d", c.ID, c.Limit)
		}
	}
}

func TestStarterDeckHasDefaultDeckSize(t *testing.T) {
	deck := StarterDeck()
	if len(deck) != DefaultDeckSize {
		t.Fatalf("expected deck size %d, got %d", DefaultDeckSize, len(deck))
	}
}
