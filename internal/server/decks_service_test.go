package server

import (
	"testing"

	"power-chess/internal/gameplay"
)

func TestDeckServiceEnsureDefaultAndMaxDecks(t *testing.T) {
	db, auth := openAuthTestDB(t)
	u, err := auth.RegisterUser("duser", "d@example.com", "password1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	ds := NewDeckService(db, func(uint64) bool { return false })
	if err := ds.EnsureDefaultDeckForUser(u.ID); err != nil {
		t.Fatalf("ensure default: %v", err)
	}
	n, err := ds.DeckCount(u.ID)
	if err != nil || n != 1 {
		t.Fatalf("deck count %d err %v", n, err)
	}
	ids := gameplay.DefaultDeckPresetCardIDs()
	for i := 0; i < gameplay.MaxSavedDecksPerUser-1; i++ {
		name := "extra"
		if i > 0 {
			name = "extra2"
		}
		_, err := ds.CreateDeck(u.ID, name, ids, "reinforcements", SleeveBlue)
		if err != nil {
			t.Fatalf("create deck %d: %v", i, err)
		}
	}
	if _, err := ds.CreateDeck(u.ID, "overflow", ids, "reinforcements", SleeveBlue); err != ErrTooManyDecks {
		t.Fatalf("expected ErrTooManyDecks, got %v", err)
	}
}

func TestDefaultSleeveColor(t *testing.T) {
	if got := DefaultSleeveColor(""); got != SleeveBlue {
		t.Fatalf("empty: want %q got %q", SleeveBlue, got)
	}
	if got := DefaultSleeveColor("  green  "); got != SleeveGreen {
		t.Fatalf("green: want %q got %q", SleeveGreen, got)
	}
	if got := DefaultSleeveColor("not-a-sleeve"); got != SleeveBlue {
		t.Fatalf("invalid: want %q got %q", SleeveBlue, got)
	}
}
