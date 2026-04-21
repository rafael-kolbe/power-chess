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

// TestZipLinePendingSnapshotRoundTrip ensures Zip Line pending metadata survives export/import.
func TestZipLinePendingSnapshotRoundTrip(t *testing.T) {
	state, err := gameplay.NewMatchState(gameplay.StarterDeck(), gameplay.StarterDeck())
	if err != nil {
		t.Fatalf("new match state failed: %v", err)
	}
	e := NewEngine(state, chess.NewGame())
	src := chess.Pos{Row: 6, Col: 1}
	e.pendingEffects[gameplay.PlayerA] = []PendingEffect{{
		Owner:       gameplay.PlayerA,
		CardID:      CardZipLine,
		Resolver:    e.resolvers[CardZipLine],
		ZipLineFrom: &src,
	}}

	snapshot := e.ExportState()
	if len(snapshot.PendingEffects) != 1 || snapshot.PendingEffects[0].SourceRow == nil || snapshot.PendingEffects[0].SourceCol == nil {
		t.Fatalf("expected exported source coordinates, got %+v", snapshot.PendingEffects)
	}
	restored, err := NewEngineFromState(snapshot)
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	q := restored.pendingEffects[gameplay.PlayerA]
	if len(q) != 1 || q[0].ZipLineFrom == nil || *q[0].ZipLineFrom != src {
		t.Fatalf("expected ZipLineFrom restored, got %+v", q)
	}
}
