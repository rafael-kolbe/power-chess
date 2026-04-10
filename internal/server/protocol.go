package server

import (
	"encoding/json"
	"fmt"
)

// MessageType identifies protocol-level websocket event kinds.
type MessageType string

const (
	// Client -> Server
	MessagePing            MessageType = "ping"
	MessageJoinMatch       MessageType = "join_match"
	MessageLeaveMatch      MessageType = "leave_match"
	MessageSubmitMove      MessageType = "submit_move"
	MessageActivateCard    MessageType = "activate_card"
	MessageResolvePending  MessageType = "resolve_pending_effect"
	MessageQueueReaction   MessageType = "queue_reaction"
	MessageResolveReaction MessageType = "resolve_reactions"
	MessageStayInRoom      MessageType = "stay_in_room"
	MessageRequestRematch  MessageType = "request_rematch"
	// Server -> Client
	MessageHello         MessageType = "hello"
	MessageAck           MessageType = "ack"
	MessageError         MessageType = "error"
	MessageStateSnapshot MessageType = "state_snapshot"
)

// ErrorCode standardizes transport-level and gameplay-level error identifiers.
type ErrorCode string

const (
	ErrorBadRequest         ErrorCode = "bad_request"
	ErrorUnknownMessageType ErrorCode = "unknown_message_type"
	ErrorJoinRequired       ErrorCode = "join_required"
	ErrorActionFailed       ErrorCode = "action_failed"
	ErrorInvalidPayload     ErrorCode = "invalid_payload"
	ErrorProtocolViolation  ErrorCode = "protocol_violation"
)

// Envelope is the shared JSON container for every websocket message.
type Envelope struct {
	ID      string          `json:"id,omitempty"`
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// PingPayload is used for heartbeat messages.
type PingPayload struct {
	Timestamp int64 `json:"timestamp"`
}

// JoinMatchPayload requests a room assignment and player identity.
type JoinMatchPayload struct {
	RoomID    string `json:"roomId,omitempty"`
	RoomName  string `json:"roomName,omitempty"`
	PlayerID  string `json:"playerId,omitempty"`
	PieceType string `json:"pieceType,omitempty"`
	IsPrivate bool   `json:"isPrivate,omitempty"`
	Password  string `json:"password,omitempty"`
}

// SubmitMovePayload sends a chess move from frontend to backend.
type SubmitMovePayload struct {
	FromRow int `json:"fromRow"`
	FromCol int `json:"fromCol"`
	ToRow   int `json:"toRow"`
	ToCol   int `json:"toCol"`
}

// ActivateCardPayload requests activation of a card from hand index.
type ActivateCardPayload struct {
	HandIndex int `json:"handIndex"`
}

// ResolvePendingPayload provides a generic target for pending effects.
type ResolvePendingPayload struct {
	PieceRow *int `json:"pieceRow,omitempty"`
	PieceCol *int `json:"pieceCol,omitempty"`
}

// QueueReactionPayload queues a reaction card with optional target data.
type QueueReactionPayload struct {
	HandIndex int  `json:"handIndex"`
	PieceRow  *int `json:"pieceRow,omitempty"`
	PieceCol  *int `json:"pieceCol,omitempty"`
}

// AckPayload confirms processing status for a received request.
type AckPayload struct {
	RequestID   string      `json:"requestId,omitempty"`
	RequestType MessageType `json:"requestType"`
	Status      string      `json:"status"`
	Code        string      `json:"code,omitempty"`
	Message     string      `json:"message,omitempty"`
}

// ErrorPayload standardizes error responses for clients.
type ErrorPayload struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// PlayerHUDState is the per-player summary used by the frontend HUD.
type PlayerHUDState struct {
	PlayerID       string `json:"playerId"`
	Mana           int    `json:"mana"`
	MaxMana        int    `json:"maxMana"`
	EnergizedMana  int    `json:"energizedMana"`
	MaxEnergized   int    `json:"maxEnergized"`
	HandCount      int    `json:"handCount"`
	CooldownCount  int    `json:"cooldownCount"`
	GraveyardCount int    `json:"graveyardCount"`
	Strikes        int    `json:"strikes"`
}

// PendingEffectState describes unresolved effects that need player input.
type PendingEffectState struct {
	Owner  string `json:"owner"`
	CardID string `json:"cardId"`
}

// ReactionWindowState describes current reaction context for frontend prompts.
type ReactionWindowState struct {
	Open          bool     `json:"open"`
	Trigger       string   `json:"trigger,omitempty"`
	Actor         string   `json:"actor,omitempty"`
	EligibleTypes []string `json:"eligibleTypes,omitempty"`
	StackSize     int      `json:"stackSize"`
}

// PendingCaptureState describes a deferred capture move awaiting reaction resolution.
type PendingCaptureState struct {
	Active  bool   `json:"active"`
	FromRow int    `json:"fromRow,omitempty"`
	FromCol int    `json:"fromCol,omitempty"`
	ToRow   int    `json:"toRow,omitempty"`
	ToCol   int    `json:"toCol,omitempty"`
	Actor   string `json:"actor,omitempty"`
}

// EnPassantStateSnapshot exposes chess en-passant targets for client-side move highlighting.
type EnPassantStateSnapshot struct {
	Valid     bool `json:"valid"`
	TargetRow int  `json:"targetRow,omitempty"`
	TargetCol int  `json:"targetCol,omitempty"`
	PawnRow   int  `json:"pawnRow,omitempty"`
	PawnCol   int  `json:"pawnCol,omitempty"`
}

// CastlingRightsSnapshot mirrors server-side castling rights for client move hints.
type CastlingRightsSnapshot struct {
	WhiteKingSide  bool `json:"whiteKingSide"`
	WhiteQueenSide bool `json:"whiteQueenSide"`
	BlackKingSide  bool `json:"blackKingSide"`
	BlackQueenSide bool `json:"blackQueenSide"`
}

// StateSnapshotPayload is a transport-friendly match summary for HUD updates.
type StateSnapshotPayload struct {
	RoomID          string                 `json:"roomId"`
	RoomName        string                 `json:"roomName"`
	RoomPrivate     bool                   `json:"roomPrivate"`
	RoomPassword    string                 `json:"roomPassword,omitempty"`
	ConnectedA      int                    `json:"connectedA"`
	ConnectedB      int                    `json:"connectedB"`
	PlayerAName     string                 `json:"playerAName,omitempty"`
	PlayerBName     string                 `json:"playerBName,omitempty"`
	GameStarted     bool                   `json:"gameStarted"`
	TurnPlayer      string                 `json:"turnPlayer"`
	TurnSeconds     int                    `json:"turnSeconds"`
	TurnNumber      int                    `json:"turnNumber"`
	IgnitionOn      bool                   `json:"ignitionOn"`
	IgnitionCard    string                 `json:"ignitionCard,omitempty"`
	Board           [8][8]string           `json:"board"`
	EnPassant       EnPassantStateSnapshot `json:"enPassant"`
	CastlingRights  CastlingRightsSnapshot `json:"castlingRights"`
	Players         []PlayerHUDState       `json:"players"`
	PendingEffects  []PendingEffectState   `json:"pendingEffects"`
	ReactionWindow  ReactionWindowState    `json:"reactionWindow"`
	PendingCapture  PendingCaptureState    `json:"pendingCapture"`
	MatchEnded      bool                   `json:"matchEnded"`
	Winner          string                 `json:"winner,omitempty"`
	EndReason       string                 `json:"endReason,omitempty"`
	RematchA        bool                   `json:"rematchA"`
	RematchB        bool                   `json:"rematchB"`
	PostMatchMsLeft int64                  `json:"postMatchMsLeft,omitempty"`
}

// DecodeEnvelope validates and decodes a raw websocket frame into Envelope.
func DecodeEnvelope(raw []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Envelope{}, fmt.Errorf("invalid envelope json: %w", err)
	}
	if env.Type == "" {
		return Envelope{}, fmt.Errorf("message type is required")
	}
	return env, nil
}

// EncodeEnvelope encodes a protocol envelope for websocket transmission.
func EncodeEnvelope(env Envelope) ([]byte, error) {
	raw, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("encode envelope: %w", err)
	}
	return raw, nil
}

// MustPayload marshals a typed payload into json.RawMessage.
func MustPayload(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}
