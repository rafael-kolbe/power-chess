package server

import (
	"encoding/json"
	"testing"
)

// TestDecodeEnvelopeRequiresType ensures protocol messages always specify a type.
func TestDecodeEnvelopeRequiresType(t *testing.T) {
	_, err := DecodeEnvelope([]byte(`{"payload":{}}`))
	if err == nil {
		t.Fatalf("expected error when type is missing")
	}
}

// TestEncodeDecodeRoundTrip ensures envelope serialization is stable.
func TestEncodeDecodeRoundTrip(t *testing.T) {
	in := Envelope{
		ID:      "1",
		Type:    MessagePing,
		Payload: MustPayload(PingPayload{Timestamp: 42}),
	}
	raw, err := EncodeEnvelope(in)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	out, err := DecodeEnvelope(raw)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if out.Type != in.Type || out.ID != in.ID {
		t.Fatalf("round-trip mismatch")
	}
}

func TestJoinMatchPayloadUnmarshalsNumericRoomID(t *testing.T) {
	var p JoinMatchPayload
	if err := json.Unmarshal([]byte(`{"roomId":42,"pieceType":"black"}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(p.RoomID) != "42" {
		t.Fatalf("room id: want 42 got %q", p.RoomID)
	}
}

func TestIgniteCardPayloadUnmarshalsTargetPieces(t *testing.T) {
	var p IgniteCardPayload
	raw := []byte(`{"handIndex":1,"target_pieces":[{"row":6,"col":4}]}`)
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.HandIndex != 1 {
		t.Fatalf("hand index: want 1 got %d", p.HandIndex)
	}
	if len(p.TargetPieces) != 1 || p.TargetPieces[0].Row != 6 || p.TargetPieces[0].Col != 4 {
		t.Fatalf("target pieces mismatch: %+v", p.TargetPieces)
	}
}

// TestSubmitMovePayloadUnmarshalsPromotion verifies that pawn promotion choice
// can travel through the submit_move payload.
func TestSubmitMovePayloadUnmarshalsPromotion(t *testing.T) {
	var p SubmitMovePayload
	raw := []byte(`{"fromRow":1,"fromCol":0,"toRow":0,"toCol":0,"promotion":"knight"}`)
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Promotion != "knight" {
		t.Fatalf("promotion: want knight got %q", p.Promotion)
	}
}

// TestResolvePendingPayloadAcceptsTargetCardID verifies that the deck-search target card field
// round-trips through JSON correctly.
func TestResolvePendingPayloadAcceptsTargetCardID(t *testing.T) {
	cardID := "knight-touch"
	raw, err := json.Marshal(ResolvePendingPayload{TargetCardID: &cardID})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out ResolvePendingPayload
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.TargetCardID == nil || *out.TargetCardID != cardID {
		t.Fatalf("expected targetCardId %q, got %v", cardID, out.TargetCardID)
	}
}

// TestPendingEffectStateDeckSearchChoicesRoundTrip ensures deck search choices serialize correctly.
func TestPendingEffectStateDeckSearchChoicesRoundTrip(t *testing.T) {
	pes := PendingEffectState{
		Owner:  "A",
		CardID: "archmage-arsenal",
		DeckSearchChoices: []DeckSearchChoice{
			{CardID: "knight-touch"},
			{CardID: "energy-gain"},
		},
	}
	raw, err := json.Marshal(pes)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out PendingEffectState
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.DeckSearchChoices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(out.DeckSearchChoices))
	}
	if out.DeckSearchChoices[0].CardID != "knight-touch" {
		t.Fatalf("expected first choice knight-touch, got %s", out.DeckSearchChoices[0].CardID)
	}
}
