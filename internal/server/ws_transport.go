package server

import (
	"strings"

	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

// WSTransport builds transport envelopes for websocket broadcast payloads.
type WSTransport struct{}

// NewWSTransport creates an envelope builder for websocket payloads.
func NewWSTransport() WSTransport {
	return WSTransport{}
}

// BuildActivateCardEnvelope builds one activate_card frame from backend-authoritative FX events.
func (WSTransport) BuildActivateCardEnvelope(ev match.ActivationFXEvent) Envelope {
	cardType := ""
	if def, ok := gameplay.CardDefinitionByID(ev.CardID); ok {
		cardType = strings.ToLower(string(def.Type))
	}
	return Envelope{
		Type: MessageActivateCard,
		Payload: MustPayload(ActivateCardEventPayload{
			PlayerID: string(ev.Owner),
			CardID:   string(ev.CardID),
			CardType: cardType,
			Success:  ev.Success,
		}),
	}
}
