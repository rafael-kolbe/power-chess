package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"power-chess/internal/gameplay"
)

// TestSnapshotHandIsPrivate verifies that each player's snapshot only exposes their own hand.
func TestSnapshotHandIsPrivate(t *testing.T) {
	room, err := NewRoomSession("99")
	if err != nil {
		t.Fatalf("setup room: %v", err)
	}
	// Both players draw from their starter decks (no mana cost for initial draw in tests).
	s := room.Engine.State
	// Give both players a non-empty hand (already from initial draw in NewMatchState).
	if len(s.Players[gameplay.PlayerA].Hand) == 0 {
		t.Fatal("player A should have initial hand cards")
	}

	snapA := room.SnapshotForPlayer(gameplay.PlayerA)
	snapB := room.SnapshotForPlayer(gameplay.PlayerB)
	snapGeneric := room.SnapshotForPlayer("")

	findPlayer := func(snap StateSnapshotPayload, pid string) *PlayerHUDState {
		for i := range snap.Players {
			if snap.Players[i].PlayerID == pid {
				return &snap.Players[i]
			}
		}
		return nil
	}

	// Player A's snapshot: A sees their hand, B's hand is nil.
	pA := findPlayer(snapA, "A")
	if pA == nil {
		t.Fatal("player A not in snapshot")
	}
	if len(pA.Hand) == 0 {
		t.Error("player A should see their own hand in their snapshot")
	}
	pBinA := findPlayer(snapA, "B")
	if pBinA != nil && len(pBinA.Hand) > 0 {
		t.Error("player A should not see player B's hand")
	}

	// Player B's snapshot: B sees their hand, A's hand is nil.
	pBinB := findPlayer(snapB, "B")
	if pBinB == nil {
		t.Fatal("player B not in snapshot")
	}
	if len(pBinB.Hand) == 0 {
		t.Error("player B should see their own hand in their snapshot")
	}
	pAinB := findPlayer(snapB, "A")
	if pAinB != nil && len(pAinB.Hand) > 0 {
		t.Error("player B should not see player A's hand")
	}

	// Generic snapshot: no hand exposed.
	for _, p := range snapGeneric.Players {
		if len(p.Hand) > 0 {
			t.Errorf("generic snapshot should not expose any hand (got %d cards for %s)", len(p.Hand), p.PlayerID)
		}
	}
}

// TestSnapshotZoneFields verifies the new zone fields are present in the snapshot.
func TestSnapshotZoneFields(t *testing.T) {
	room, err := NewRoomSession("100")
	if err != nil {
		t.Fatalf("setup room: %v", err)
	}
	snap := room.SnapshotForPlayer(gameplay.PlayerA)

	for _, p := range snap.Players {
		if p.DeckCount < 0 {
			t.Errorf("DeckCount should be >= 0 for %s", p.PlayerID)
		}
		if p.BanishedCards == nil {
			t.Errorf("BanishedCards should be non-nil slice for %s", p.PlayerID)
		}
		if p.GraveyardPieces == nil {
			t.Errorf("GraveyardPieces should be non-nil slice for %s", p.PlayerID)
		}
		if p.CooldownPreview == nil {
			t.Errorf("CooldownPreview should be non-nil slice for %s", p.PlayerID)
		}
	}
}

// TestSnapshotIgnitionOwnerAndTurns verifies ignition metadata is populated.
func TestSnapshotIgnitionOwnerAndTurns(t *testing.T) {
	room, err := NewRoomSession("101")
	if err != nil {
		t.Fatalf("setup room: %v", err)
	}
	s := room.Engine.State
	// Activate a card from player A's hand (ignition > 0).
	_ = s.StartTurn(gameplay.PlayerA)
	s.Players[gameplay.PlayerA].Mana = 10
	// Find a card with ignition > 0.
	hand := s.Players[gameplay.PlayerA].Hand
	idx := -1
	for i, c := range hand {
		if c.Ignition > 0 {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Skip("no card with ignition > 0 in starter hand")
	}
	if err := room.Engine.ActivateCard(gameplay.PlayerA, idx); err != nil {
		t.Fatalf("activate card: %v", err)
	}

	snap := room.SnapshotForPlayer(gameplay.PlayerA)
	if !snap.IgnitionOn {
		t.Error("IgnitionOn should be true")
	}
	if snap.IgnitionOwner != "A" {
		t.Errorf("IgnitionOwner: want A, got %q", snap.IgnitionOwner)
	}
	if snap.IgnitionTurnsRemaining <= 0 {
		t.Errorf("IgnitionTurnsRemaining should be > 0, got %d", snap.IgnitionTurnsRemaining)
	}
}

// TestSnapshotGraveyardOrder verifies graveyard pieces are sorted by importance.
func TestSnapshotGraveyardOrder(t *testing.T) {
	room, err := NewRoomSession("102")
	if err != nil {
		t.Fatalf("setup room: %v", err)
	}
	s := room.Engine.State
	// Add pieces out of order to player A's graveyard.
	s.Players[gameplay.PlayerA].Graveyard = []gameplay.PieceRef{
		{Color: "w", Type: "P"},
		{Color: "w", Type: "Q"},
		{Color: "w", Type: "N"},
		{Color: "w", Type: "R"},
	}

	snap := room.SnapshotForPlayer(gameplay.PlayerA)
	var pA *PlayerHUDState
	for i := range snap.Players {
		if snap.Players[i].PlayerID == "A" {
			pA = &snap.Players[i]
		}
	}
	if pA == nil {
		t.Fatal("player A not found")
	}
	want := []string{"wQ", "wR", "wN", "wP"}
	if len(pA.GraveyardPieces) != len(want) {
		t.Fatalf("graveyard length: want %d, got %d", len(want), len(pA.GraveyardPieces))
	}
	for i, w := range want {
		if pA.GraveyardPieces[i] != w {
			t.Errorf("graveyard[%d]: want %q, got %q", i, w, pA.GraveyardPieces[i])
		}
	}
}

// TestDrawCardWebSocket verifies draw_card via WebSocket increases handCount and decrements deckCount.
func TestDrawCardWebSocket(t *testing.T) {
	srv := NewServerWithStore(nil)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	dial := func() *websocket.Conn {
		c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, _, _ = c.ReadMessage() // hello
		return c
	}

	cA := dial()
	defer cA.Close()
	cB := dial()
	defer cB.Close()

	send := func(c *websocket.Conn, env Envelope) {
		raw, _ := EncodeEnvelope(env)
		if err := c.WriteMessage(websocket.TextMessage, raw); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	readSnap := func(c *websocket.Conn) StateSnapshotPayload {
		for i := 0; i < 5; i++ {
			_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, raw, err := c.ReadMessage()
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			var env Envelope
			if err := json.Unmarshal(raw, &env); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if env.Type == MessageStateSnapshot {
				var snap StateSnapshotPayload
				if err := json.Unmarshal(env.Payload, &snap); err != nil {
					t.Fatalf("decode snap: %v", err)
				}
				return snap
			}
		}
		t.Fatal("no snapshot received")
		return StateSnapshotPayload{}
	}
	drainUntilSnap := func(c *websocket.Conn) StateSnapshotPayload {
		return readSnap(c)
	}

	send(cA, Envelope{ID: "j1", Type: MessageJoinMatch, Payload: MustPayload(JoinMatchPayload{RoomID: "50", PieceType: "white"})})
	send(cB, Envelope{ID: "j2", Type: MessageJoinMatch, Payload: MustPayload(JoinMatchPayload{RoomID: "50", PieceType: "black"})})

	// drain ack+snapshot for cA
	_ = drainUntilSnap(cA)
	snapBefore := drainUntilSnap(cA)

	var pABefore PlayerHUDState
	for _, p := range snapBefore.Players {
		if p.PlayerID == "A" {
			pABefore = p
		}
	}
	// Give player A enough mana and start their turn (room starts with 0 mana — add mana via state).
	srv.roomsM.RLock()
	room := srv.rooms["50"]
	srv.roomsM.RUnlock()
	room.stateM.Lock()
	room.Engine.State.Players[gameplay.PlayerA].Mana = 10
	room.stateM.Unlock()

	// Draw a card.
	send(cA, Envelope{ID: "d1", Type: MessageDrawCard})
	snapAfter := drainUntilSnap(cA)

	var pAAfter PlayerHUDState
	for _, p := range snapAfter.Players {
		if p.PlayerID == "A" {
			pAAfter = p
		}
	}

	if pAAfter.HandCount != pABefore.HandCount+1 {
		t.Errorf("handCount: want %d, got %d", pABefore.HandCount+1, pAAfter.HandCount)
	}
	if pAAfter.DeckCount != pABefore.DeckCount-1 {
		t.Errorf("deckCount: want %d, got %d", pABefore.DeckCount-1, pAAfter.DeckCount)
	}
}

// TestDrawCardViewerHandPrivacy verifies player B does not receive player A's hand in their snapshot.
func TestDrawCardViewerHandPrivacy(t *testing.T) {
	room, err := NewRoomSession("103")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	s := room.Engine.State
	s.Players[gameplay.PlayerA].Mana = 10
	_ = s.DrawCard(gameplay.PlayerA)

	snapB := room.SnapshotForPlayer(gameplay.PlayerB)
	for _, p := range snapB.Players {
		if p.PlayerID == "A" && len(p.Hand) > 0 {
			t.Error("player B should not see player A's hand")
		}
	}
}

// TestDrawCardEngineValidation verifies draw_card engine-level error paths.
func TestDrawCardEngineValidation(t *testing.T) {
	room, err := NewRoomSession("104")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	s := room.Engine.State

	// Wrong turn: A tries to draw on B's turn.
	_ = s.EndTurn(gameplay.PlayerA)
	s.Players[gameplay.PlayerA].Mana = 10
	if err := room.Engine.DrawCard(gameplay.PlayerA); err == nil {
		t.Error("expected error drawing on wrong turn")
	}

	// Own turn but not enough mana.
	_ = s.EndTurn(gameplay.PlayerB)
	s.Players[gameplay.PlayerA].Mana = 0
	if err := room.Engine.DrawCard(gameplay.PlayerA); err == nil {
		t.Error("expected error drawing with no mana")
	}

	// Own turn, mana ok, hand full.
	s.Players[gameplay.PlayerA].Mana = 10
	for len(s.Players[gameplay.PlayerA].Hand) < gameplay.DefaultMaxHandSize {
		_ = s.DrawCard(gameplay.PlayerA)
	}
	s.Players[gameplay.PlayerA].Mana = 10
	if err := room.Engine.DrawCard(gameplay.PlayerA); err == nil {
		t.Error("expected error drawing with full hand")
	}
}
