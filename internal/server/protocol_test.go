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
