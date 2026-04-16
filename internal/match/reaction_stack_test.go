package match

import (
	"testing"

	"power-chess/internal/gameplay"
)

func TestReactionRuntimePushPopLIFO(t *testing.T) {
	rt := NewReactionRuntime()
	first := ReactionAction{Owner: gameplay.PlayerA, Card: gameplay.CardInstance{CardID: "counterattack"}}
	second := ReactionAction{Owner: gameplay.PlayerB, Card: gameplay.CardInstance{CardID: "blockade"}}
	rt.Push(first)
	rt.Push(second)

	gotSecond, ok := rt.Pop()
	if !ok {
		t.Fatalf("expected first pop to succeed")
	}
	if gotSecond.Card.CardID != "blockade" {
		t.Fatalf("expected LIFO top blockade, got %s", gotSecond.Card.CardID)
	}

	gotFirst, ok := rt.Pop()
	if !ok {
		t.Fatalf("expected second pop to succeed")
	}
	if gotFirst.Card.CardID != "counterattack" {
		t.Fatalf("expected second pop counterattack, got %s", gotFirst.Card.CardID)
	}
}

func TestReactionRuntimeEntriesBottomFirst(t *testing.T) {
	rt := NewReactionRuntime()
	rt.Push(ReactionAction{Owner: gameplay.PlayerA, Card: gameplay.CardInstance{CardID: "counterattack"}})
	rt.Push(ReactionAction{Owner: gameplay.PlayerB, Card: gameplay.CardInstance{CardID: "blockade"}})

	entries := rt.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].CardID != "counterattack" || entries[1].CardID != "blockade" {
		t.Fatalf("unexpected bottom-first order: %#v", entries)
	}
}
