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
	e.extraMoveLeft[gameplay.PlayerA] = 1
	e.movesThisTurn[gameplay.PlayerA] = 1
	pos := chess.Pos{Row: 6, Col: 0}
	e.moveBuffTarget[gameplay.PlayerA] = &pos
	e.moveBuffKind[gameplay.PlayerA] = MoveBuffKnight
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
	if restored.extraMoveLeft[gameplay.PlayerA] != 1 {
		t.Fatalf("expected extra move left to survive")
	}
	if restored.moveBuffKind[gameplay.PlayerA] != MoveBuffKnight {
		t.Fatalf("expected move buff kind to survive")
	}
	if restored.ReactionWindow == nil || !restored.ReactionWindow.Open {
		t.Fatalf("expected reaction window to survive")
	}
	pm, ok := restored.PendingMove()
	if !ok || pm.Move.To != (chess.Pos{Row: 5, Col: 1}) {
		t.Fatalf("expected pending move to survive")
	}
}
