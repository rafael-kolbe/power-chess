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

// TestMaybeCaptureAttemptOnIgnitionCatalogFalseUntilEffects asserts the catalog keeps
// MaybeCaptureAttemptOnIgnition false until card resolvers apply ignition-driven captures.
func TestMaybeCaptureAttemptOnIgnitionCatalogFalseUntilEffects(t *testing.T) {
	for _, c := range InitialCardCatalog() {
		if c.MaybeCaptureAttemptOnIgnition {
			t.Fatalf("card %q: MaybeCaptureAttemptOnIgnition must remain false until ignition capture effects ship", c.ID)
		}
		if MaybeCaptureAttemptOnIgnition(c.ID) {
			t.Fatalf("MaybeCaptureAttemptOnIgnition(%q) must be false for now", c.ID)
		}
	}
	if MaybeCaptureAttemptOnIgnition("") {
		t.Fatal("empty id must be false")
	}
	if MaybeCaptureAttemptOnIgnition("no-such-card") {
		t.Fatal("unknown id must be false")
	}
}
