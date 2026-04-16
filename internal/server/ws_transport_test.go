package server

import (
	"testing"

	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

func TestWSTransportBuildsActivateCardEnvelope(t *testing.T) {
	transport := NewWSTransport()
	env := transport.BuildActivateCardEnvelope(match.ActivationFXEvent{
		Owner:   gameplay.PlayerA,
		CardID:  "counterattack",
		Success: true,
	})

	if env.Type != MessageActivateCard {
		t.Fatalf("expected %s envelope, got %s", MessageActivateCard, env.Type)
	}
	var payload ActivateCardEventPayload
	if err := jsonUnmarshalForTest(env.Payload, &payload); err != nil {
		t.Fatalf("decode payload failed: %v", err)
	}
	if payload.PlayerID != "A" || payload.CardID != "counterattack" || !payload.Success {
		t.Fatalf("unexpected activate payload: %+v", payload)
	}
}
