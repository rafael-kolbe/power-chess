package server

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MessageType identifies protocol-level websocket event kinds.
type MessageType string

const (
	// Client -> Server
	MessagePing                  MessageType = "ping"
	MessageJoinMatch             MessageType = "join_match"
	MessageLeaveMatch            MessageType = "leave_match"
	MessageSubmitMove            MessageType = "submit_move"
	MessageIgniteCard            MessageType = "ignite_card"
	MessageSubmitIgnitionTargets MessageType = "submit_ignition_targets"
	MessageDrawCard              MessageType = "draw_card"
	MessageResolvePending        MessageType = "resolve_pending_effect"
	MessageQueueReaction         MessageType = "queue_reaction"
	MessageResolveReaction       MessageType = "resolve_reactions"
	MessageStayInRoom            MessageType = "stay_in_room"
	MessageRequestRematch        MessageType = "request_rematch"
	MessageConfirmMulligan       MessageType = "confirm_mulligan"
	MessageSetReactionMode       MessageType = "set_reaction_mode"
	MessageDebugMatchFixture     MessageType = "debug_match_fixture"
	MessageClientTrace           MessageType = "client_trace"
	MessageClientFxHold          MessageType = "client_fx_hold"
	MessageClientFxRelease       MessageType = "client_fx_release"
	// Server -> Client
	MessageHello         MessageType = "hello"
	MessageAck           MessageType = "ack"
	MessageError         MessageType = "error"
	MessageStateSnapshot MessageType = "state_snapshot"
	// MessageActivateCard is server→client only: effect activation after ignition counter reaches 0.
	MessageActivateCard MessageType = "activate_card"
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
	ErrorDebugDisabled      ErrorCode = "debug_disabled"
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

// joinRoomID unmarshals roomId from JSON as either a string or a number (some clients send a bare integer).
type joinRoomID string

func (j *joinRoomID) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*j = joinRoomID(strings.TrimSpace(s))
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	s := strings.TrimSpace(n.String())
	*j = joinRoomID(s)
	return nil
}

// JoinMatchPayload requests a room assignment and player identity.
type JoinMatchPayload struct {
	RoomID    joinRoomID `json:"roomId,omitempty"`
	RoomName  string     `json:"roomName,omitempty"`
	PlayerID  string     `json:"playerId,omitempty"`
	PieceType string     `json:"pieceType,omitempty"`
	IsPrivate bool       `json:"isPrivate,omitempty"`
	Password  string     `json:"password,omitempty"`
}

// SubmitMovePayload sends a chess move from frontend to backend.
type SubmitMovePayload struct {
	FromRow int `json:"fromRow"`
	FromCol int `json:"fromCol"`
	ToRow   int `json:"toRow"`
	ToCol   int `json:"toCol"`
}

// IgniteCardPayload moves a card from hand into this player's ignition zone (may open reaction windows).
type IgniteCardPayload struct {
	HandIndex    int                  `json:"handIndex"`
	TargetPieces []TargetPiecePayload `json:"target_pieces,omitempty"`
}

// SubmitIgnitionTargetsPayload locks piece coordinates for a card already in ignition when the catalog requires Targets > 0.
type SubmitIgnitionTargetsPayload struct {
	TargetPieces []TargetPiecePayload `json:"target_pieces"`
}

// TargetPiecePayload identifies one selected board piece coordinate.
type TargetPiecePayload struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// IgnitionTargetingSnapshot exposes ignite targeting state while a target-requiring card sits in ignition.
type IgnitionTargetingSnapshot struct {
	Owner                string               `json:"owner,omitempty"`
	CardID               string               `json:"cardId,omitempty"`
	AwaitingTargetChoice bool                 `json:"awaitingTargetChoice,omitempty"`
	TargetPieces         []TargetPiecePayload `json:"target_pieces,omitempty"`
}

// ActivePieceEffectSnapshot describes an ongoing on-board buff (e.g. movement grant) for client glow and hints.
type ActivePieceEffectSnapshot struct {
	Owner          string `json:"owner"`
	CardID         string `json:"cardId"`
	Row            int    `json:"row"`
	Col            int    `json:"col"`
	TurnsRemaining int    `json:"turnsRemaining"`
}

// ActivateCardEventPayload is server→client: ignition finished and the effect resolution step ran.
type ActivateCardEventPayload struct {
	PlayerID          string `json:"playerId"`
	CardID            string `json:"cardId"`
	CardType          string `json:"cardType,omitempty"`
	Success           bool   `json:"success"`
	RetainIgnition    bool   `json:"retainIgnition,omitempty"`
	// NegatesActivationOf is the player ID whose card in the ignition zone had its activation
	// negated by this event (EffectNegated transitioned false→true). Non-empty only when this
	// event caused that transition. Client must show the negate overlay immediately after the glow.
	NegatesActivationOf string `json:"negatesActivationOf,omitempty"`
}

// ConfirmMulliganPayload submits which hand cards (by index) are returned to the deck for the mulligan.
type ConfirmMulliganPayload struct {
	HandIndices []int `json:"handIndices"`
}

// SetReactionModePayload updates the player's capture/reaction preference for the match.
// Mode is "off", "on", or "auto" (case-insensitive; unknown values become "on").
type SetReactionModePayload struct {
	Mode string `json:"mode"`
}

// ClientTracePayload carries a browser-side debug line batch for server session logs (ADMIN_DEBUG_MATCH only).
type ClientTracePayload struct {
	Text string `json:"text"`
}

// DebugSideFixture lists full deck order (20 legal constructed cards) and hand card IDs to draw from that deck.
// Keys "white" / "black" map to chess white (player A) and black (player B).
// Optional mana fields override defaults after the preset hands are dealt; omitted fields keep engine defaults.
type DebugSideFixture struct {
	Deck          []string `json:"deck"`
	Hand          []string `json:"hand"`
	Mana          *int     `json:"mana,omitempty"`
	MaxMana       *int     `json:"maxMana,omitempty"`
	EnergizedMana *int     `json:"energizedMana,omitempty"`
	MaxEnergized  *int     `json:"maxEnergized,omitempty"`
}

// DebugMatchFixturePayload replaces opening state when the server runs with ADMIN_DEBUG_MATCH enabled.
// test_environment must be true (handshake); otherwise the request is rejected even when debugging is on.
type DebugMatchFixturePayload struct {
	TestEnvironment bool              `json:"test_environment"`
	White           *DebugSideFixture `json:"white"`
	Black           *DebugSideFixture `json:"black"`
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

// CardSnapshotEntry is a single card's public data exposed in hand, cooldown or banished zones.
type CardSnapshotEntry struct {
	CardID   string `json:"cardId"`
	ManaCost int    `json:"manaCost"`
	Ignition int    `json:"ignition"`
	Cooldown int    `json:"cooldown"`
}

// CooldownPreviewEntry is a card currently in the cooldown pile with its remaining turns.
type CooldownPreviewEntry struct {
	CardID         string `json:"cardId"`
	ManaCost       int    `json:"manaCost"`
	Ignition       int    `json:"ignition"`
	Cooldown       int    `json:"cooldown"`
	TurnsRemaining int    `json:"turnsRemaining"`
}

// PlayerHUDState is the per-player summary used by the frontend HUD.
// The Hand field is only populated when the snapshot is addressed to the owning player.
type PlayerHUDState struct {
	PlayerID       string `json:"playerId"`
	Mana           int    `json:"mana"`
	MaxMana        int    `json:"maxMana"`
	EnergizedMana  int    `json:"energizedMana"`
	MaxEnergized   int    `json:"maxEnergized"`
	HandCount      int    `json:"handCount"`
	CooldownCount  int    `json:"cooldownCount"`
	GraveyardCount int    `json:"graveyardCount"`
	// Zone data — always public unless noted.
	DeckCount       int                    `json:"deckCount"`
	SleeveColor     string                 `json:"sleeveColor"`
	Hand            []CardSnapshotEntry    `json:"hand,omitempty"`
	BanishedCards   []CardSnapshotEntry    `json:"banishedCards"`
	GraveyardPieces []string               `json:"graveyardPieces"`
	CooldownPreview []CooldownPreviewEntry `json:"cooldownPreview"`
	// CooldownHiddenCount is always 0; kept for stable JSON shape (full queue is in CooldownPreview).
	CooldownHiddenCount int `json:"cooldownHiddenCount"`
	// ReactionMode is off / on / auto — server authority for when to open reaction windows.
	ReactionMode string `json:"reactionMode,omitempty"`
	// Per-player ignition zone (public to both players).
	IgnitionOn               bool   `json:"ignitionOn,omitempty"`
	IgnitionCard             string `json:"ignitionCard,omitempty"`
	IgnitionTurnsRemaining   int    `json:"ignitionTurnsRemaining,omitempty"`
	IgnitionEffectNegated    bool   `json:"ignitionEffectNegated,omitempty"`
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
	// StagedCardID is the top of the in-memory reaction stack (last queued card), if any.
	StagedCardID string `json:"stagedCardId,omitempty"`
	// StagedOwner is the seat that played StagedCardID onto the stack.
	StagedOwner string `json:"stagedOwner,omitempty"`
	// StackCards lists queued reaction cards bottom-first (first queued first). Resolution is LIFO
	// (last entry resolves first); clients use this for ordered resolve animations.
	StackCards []ReactionStackPreviewEntry `json:"stackCards,omitempty"`
	// TargetingOwner/TargetingCardID/TargetPieces mirror locked ignite targets while the reaction window is active.
	TargetingOwner  string               `json:"targetingOwner,omitempty"`
	TargetingCardID string               `json:"targetingCardId,omitempty"`
	TargetPieces    []TargetPiecePayload `json:"target_pieces,omitempty"`
}

// ReactionStackPreviewEntry is one card on the reaction stack for client animations.
type ReactionStackPreviewEntry struct {
	CardID string `json:"cardId"`
	Owner  string `json:"owner"`
}

// PendingCaptureState describes a deferred capture move awaiting reaction resolution.
type PendingCaptureState struct {
	Active  bool   `json:"active"`
	FromRow int    `json:"fromRow"`
	FromCol int    `json:"fromCol"`
	ToRow   int    `json:"toRow"`
	ToCol   int    `json:"toCol"`
	Actor   string `json:"actor,omitempty"`
}

// EnPassantStateSnapshot exposes chess en-passant targets for client-side move highlighting.
type EnPassantStateSnapshot struct {
	Valid     bool `json:"valid"`
	TargetRow int  `json:"targetRow"`
	TargetCol int  `json:"targetCol"`
	PawnRow   int  `json:"pawnRow"`
	PawnCol   int  `json:"pawnCol"`
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
	RoomID       string `json:"roomId"`
	RoomName     string `json:"roomName"`
	RoomPrivate  bool   `json:"roomPrivate"`
	RoomPassword string `json:"roomPassword,omitempty"`
	ConnectedA   int    `json:"connectedA"`
	ConnectedB   int    `json:"connectedB"`
	PlayerAName  string `json:"playerAName,omitempty"`
	PlayerBName  string `json:"playerBName,omitempty"`
	GameStarted  bool   `json:"gameStarted"`
	// MulliganPhaseActive is true while players are choosing cards to return (opening).
	MulliganPhaseActive bool `json:"mulliganPhaseActive,omitempty"`
	// MulliganReturned maps seat id ("A"/"B") to how many cards that player returned; -1 until they confirm.
	MulliganReturned map[string]int `json:"mulliganReturned,omitempty"`
	// MulliganDeadlineUnixMs is when unconfirmed seats auto-keep all cards (0 if not in mulligan).
	MulliganDeadlineUnixMs int64 `json:"mulliganDeadlineUnixMs,omitempty"`
	// ReconnectPendingFor is "A" or "B" while that seat's socket is gone but the grace timer has not fired yet.
	ReconnectPendingFor     string                 `json:"reconnectPendingFor,omitempty"`
	ReconnectDeadlineUnixMs int64                  `json:"reconnectDeadlineUnixMs,omitempty"`
	AdminDebugMatch         bool                   `json:"adminDebugMatch,omitempty"`
	TurnPlayer              string                 `json:"turnPlayer"`
	TurnNumber              int                    `json:"turnNumber"`
	Board                   [8][8]string           `json:"board"`
	EnPassant               EnPassantStateSnapshot `json:"enPassant"`
	CastlingRights          CastlingRightsSnapshot `json:"castlingRights"`
	// ViewerPlayerID identifies whose perspective this snapshot is for (drives hand visibility).
	ViewerPlayerID      string               `json:"viewerPlayerId,omitempty"`
	Players             []PlayerHUDState     `json:"players"`
	PendingEffects      []PendingEffectState `json:"pendingEffects"`
	ActivationQueueSize int                  `json:"activationQueueSize"`
	ReactionWindow      ReactionWindowState  `json:"reactionWindow"`
	// IgnitionTargeting is set when a Targets>0 Power/Continuous card is in ignition (awaiting or locked coordinates).
	IgnitionTargeting IgnitionTargetingSnapshot `json:"ignitionTargeting,omitempty"`
	// ActivePieceEffects lists resolved on-board effects still ticking (e.g. knight-pattern grant).
	ActivePieceEffects []ActivePieceEffectSnapshot `json:"activePieceEffects,omitempty"`
	// DoubleTurnActiveFor is the player seat ("A" or "B") for whom the Double Turn visual
	// effect is active this turn (empty when no Double Turn effect is active).
	DoubleTurnActiveFor string `json:"doubleTurnActiveFor,omitempty"`
	// DoubleTurnTurnsRemaining is the number of owner turns the Double Turn effect has left
	// for the player in DoubleTurnActiveFor. Used for the duration badge on highlighted pieces.
	DoubleTurnTurnsRemaining int `json:"doubleTurnTurnsRemaining,omitempty"`
	PendingCapture      PendingCaptureState `json:"pendingCapture"`
	MatchEnded         bool                        `json:"matchEnded"`
	Winner             string                      `json:"winner,omitempty"`
	EndReason          string                      `json:"endReason,omitempty"`
	RematchA           bool                        `json:"rematchA"`
	RematchB           bool                        `json:"rematchB"`
	PostMatchMsLeft    int64                       `json:"postMatchMsLeft,omitempty"`
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
