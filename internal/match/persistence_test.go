package match

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// TestEnginePersistenceRoundTrip validates export/import of runtime engine state.
func TestEnginePersistenceRoundTrip(t *testing.T) {
	state, err := gameplay.NewMatchState(gameplay.StarterDeck(), gameplay.StarterDeck())
	if err != nil {
		t.Fatalf("new match state failed: %v", err)
	}
	e := NewEngine(state, chess.NewGame())
	e.OpenReactionWindow("capture_attempt", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeCounter})
	e.pendingMove = &PendingMoveAction{
		PlayerID: gameplay.PlayerA,
		Move: chess.Move{
			From: chess.Pos{Row: 6, Col: 0},
			To:   chess.Pos{Row: 5, Col: 1},
		},
	}

	snapshot := e.ExportState()
	restored, err := NewEngineFromState(snapshot)
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if restored.ReactionWindow == nil || !restored.ReactionWindow.Open {
		t.Fatalf("expected reaction window to survive")
	}
	pm, ok := restored.PendingMove()
	if !ok || pm.Move.To != (chess.Pos{Row: 5, Col: 1}) {
		t.Fatalf("expected pending move to survive")
	}
}
