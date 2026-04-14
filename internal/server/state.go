package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

// RoomSession stores per-room runtime state used by websocket handlers.
type RoomSession struct {
	RoomID             string
	RoomName           string
	RoomPrivate        bool
	RoomPassword       string
	Engine             *match.Engine
	Players            map[string]gameplay.PlayerID
	clients            map[*Client]struct{}
	clientsM           sync.RWMutex
	stateM             sync.Mutex
	seen               map[string]struct{}
	connectedByPlayer  map[gameplay.PlayerID]int
	disconnectTimers   map[gameplay.PlayerID]*time.Timer
	disconnectDeadline map[gameplay.PlayerID]time.Time // wall-clock instant when disconnect win may be applied for that seat
	// DisconnectGrace is deprecated: use DisconnectBudgetTotal. Kept for tests that set a long "grace" window.
	DisconnectGrace time.Duration
	// DisconnectBudgetTotal is cumulative wall-clock time a player may spend disconnected per match (default 60s).
	DisconnectBudgetTotal time.Duration
	// DisconnectMinWinDelay is the minimum time after disconnect detection before declaring a disconnect win (default 5s).
	DisconnectMinWinDelay time.Duration
	// disconnectBudgetRemaining is unused disconnect budget per seat for this match (drains while offline).
	disconnectBudgetRemaining map[gameplay.PlayerID]time.Duration
	// disconnectSegmentStart records when the current offline segment began (zero if seat is connected or not in segment).
	disconnectSegmentStart map[gameplay.PlayerID]time.Time
	// disconnectFrozen* preserve match clocks while exactly one player is offline (PROJECT.md pause).
	disconnectFrozenReactionRemaining time.Duration
	disconnectFrozenMainRemaining     time.Duration
	disconnectFrozenMainFor           gameplay.PlayerID
	disconnectFrozenCarryPausedTurn   bool
	matchEnded                        bool
	winner                            gameplay.PlayerID
	endReason                         string
	reactionTimeout                   time.Duration
	reactionDeadline                  time.Time
	turnDeadline                      time.Time
	turnDeadlineFor                   gameplay.PlayerID
	// pausedTurnRemaining holds the main turn timer slice while a reaction window waits for the
	// first response (capture_attempt or ignite_reaction); turnDeadline is cleared in that state.
	pausedTurnRemaining time.Duration
	// mulliganDeadline is when the server auto-confirms any player who has not locked in (opening only).
	mulliganDeadline  time.Time
	postMatchDeadline time.Time
	rematchVotes      map[gameplay.PlayerID]bool
	lastActivity      time.Time
	// displayNameByPlayer holds authenticated usernames per seat for the match HUD (cleared when a seat disconnects).
	displayNameByPlayer map[gameplay.PlayerID]string
	// parent is set when the room is registered on a Server (used to resolve saved decks); nil in isolated tests.
	parent *Server
	// authUIDByPlayer maps each seat to the account id (0 = guest / no auth).
	authUIDByPlayer map[gameplay.PlayerID]uint64
	// sleeveByPlayer stores the sleeve color chosen for each seat (empty string = default blue).
	sleeveByPlayer map[gameplay.PlayerID]string
	// deckMatchInitialized is true after the engine was built from saved decks for both connected players, or when loaded from persistence.
	deckMatchInitialized bool
	// reactionModeByPlayer stores each seat's preference: off / on / auto (see NormalizeReactionMode).
	reactionModeByPlayer map[gameplay.PlayerID]string
	// debugPauseActive blocks gameplay actions and timeout countdowns while admin debugging is enabled.
	debugPauseActive bool
	// debugPauseStartedAt is when debug pause was enabled (used to shift deadlines on resume).
	debugPauseStartedAt time.Time
	// adminDebugMatch mirrors server-level ADMIN_DEBUG_MATCH for snapshot/UI capabilities.
	adminDebugMatch bool
}

const defaultRoomName = "Let's Play!"

// mulliganPhaseDuration is the window for both players to confirm opening mulligan; unconfirmed seats auto-keep.
const mulliganPhaseDuration = 15 * time.Second

// NewRoomSession creates a ready-to-use match engine bound to a room.
func NewRoomSession(roomID string) (*RoomSession, error) {
	return NewRoomSessionWithName(roomID, defaultRoomName)
}

// NewRoomSessionWithName creates a ready-to-use match engine with custom room name.
func NewRoomSessionWithName(roomID, roomName string) (*RoomSession, error) {
	state, err := gameplay.NewMatchState(gameplay.StarterDeck(), gameplay.StarterDeck())
	if err != nil {
		return nil, err
	}
	engine := match.NewEngine(state, chess.NewGame())
	return newRoomSessionWithEngine(roomID, roomName, engine), nil
}

func newRoomSessionWithEngine(roomID, roomName string, engine *match.Engine) *RoomSession {
	if strings.TrimSpace(roomName) == "" {
		roomName = defaultRoomName
	}
	defaultBudget := 60 * time.Second
	return &RoomSession{
		RoomID:   roomID,
		RoomName: roomName,
		Engine:   engine,
		Players:  map[string]gameplay.PlayerID{},
		clients:  map[*Client]struct{}{},
		seen:     map[string]struct{}{},
		connectedByPlayer: map[gameplay.PlayerID]int{
			gameplay.PlayerA: 0,
			gameplay.PlayerB: 0,
		},
		disconnectTimers:      map[gameplay.PlayerID]*time.Timer{},
		disconnectDeadline:    map[gameplay.PlayerID]time.Time{},
		DisconnectGrace:       defaultBudget,
		DisconnectBudgetTotal: defaultBudget,
		DisconnectMinWinDelay: 5 * time.Second,
		disconnectBudgetRemaining: map[gameplay.PlayerID]time.Duration{
			gameplay.PlayerA: defaultBudget,
			gameplay.PlayerB: defaultBudget,
		},
		disconnectSegmentStart: nil,
		reactionTimeout:        30 * time.Second,
		rematchVotes:           map[gameplay.PlayerID]bool{},
		lastActivity:           time.Now().UTC(),
		displayNameByPlayer: map[gameplay.PlayerID]string{
			gameplay.PlayerA: "",
			gameplay.PlayerB: "",
		},
		authUIDByPlayer: map[gameplay.PlayerID]uint64{
			gameplay.PlayerA: 0,
			gameplay.PlayerB: 0,
		},
		sleeveByPlayer: map[gameplay.PlayerID]string{
			gameplay.PlayerA: "",
			gameplay.PlayerB: "",
		},
		deckMatchInitialized: false,
		reactionModeByPlayer: map[gameplay.PlayerID]string{
			gameplay.PlayerA: ReactionModeOn,
			gameplay.PlayerB: ReactionModeOn,
		},
	}
}

// BothPlayersConnected reports whether at least one client is connected on each side.
func (r *RoomSession) BothPlayersConnected() bool {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	return r.connectedByPlayer[gameplay.PlayerA] > 0 && r.connectedByPlayer[gameplay.PlayerB] > 0
}

// AddClient registers a client connection in the room.
func (r *RoomSession) AddClient(c *Client) {
	r.clientsM.Lock()
	defer r.clientsM.Unlock()
	r.clients[c] = struct{}{}
}

// RemoveClient unregisters a client connection from the room.
func (r *RoomSession) RemoveClient(c *Client) {
	r.clientsM.Lock()
	defer r.clientsM.Unlock()
	delete(r.clients, c)
}

// Broadcast sends a pre-encoded envelope to all room clients.
func (r *RoomSession) Broadcast(env Envelope) {
	r.clientsM.RLock()
	defer r.clientsM.RUnlock()
	for c := range r.clients {
		c.send(env)
	}
}

// Execute runs a mutation block under room-state lock.
func (r *RoomSession) Execute(fn func() error) error {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	return fn()
}

// MarkRequestOnce records a request key and returns false when key already exists.
func (r *RoomSession) MarkRequestOnce(requestKey string) bool {
	if _, ok := r.seen[requestKey]; ok {
		return false
	}
	r.seen[requestKey] = struct{}{}
	return true
}

// reactionModeUnsafe returns the canonical reaction mode for pid. Caller must hold r.stateM.
func (r *RoomSession) reactionModeUnsafe(pid gameplay.PlayerID) string {
	if r.reactionModeByPlayer == nil {
		return ReactionModeOn
	}
	m, ok := r.reactionModeByPlayer[pid]
	if !ok || m == "" {
		return ReactionModeOn
	}
	return m
}

// setReactionModeUnsafe stores the player's reaction preference. Caller must hold r.stateM.
func (r *RoomSession) setReactionModeUnsafe(pid gameplay.PlayerID, mode string) {
	if r.reactionModeByPlayer == nil {
		r.reactionModeByPlayer = map[gameplay.PlayerID]string{
			gameplay.PlayerA: ReactionModeOn,
			gameplay.PlayerB: ReactionModeOn,
		}
	}
	r.reactionModeByPlayer[pid] = NormalizeReactionMode(mode)
}

// maybeAutoResolveCaptureReactionUnsafe applies capture immediately when the responder's
// reaction mode skips the window (off, or auto with no eligible Counter). Caller must hold r.stateM.
func (r *RoomSession) maybeAutoResolveCaptureReactionUnsafe() error {
	rw, stackSize, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open || rw.Trigger != "capture_attempt" || stackSize != 0 {
		return nil
	}
	responder := oppositePlayer(rw.Actor)
	switch r.reactionModeUnsafe(responder) {
	case ReactionModeOn:
		return nil
	case ReactionModeAuto:
		if gameplay.EligibleForCaptureCounterReactionAUTO(r.Engine.State, responder) {
			return nil
		}
	default: // off
	}
	if err := r.Engine.ResolveReactionStack(); err != nil {
		return err
	}
	r.reactionDeadline = time.Time{}
	r.resumeMainTurnAfterReactionUnsafe(time.Now())
	return nil
}

// maybeAutoResolveIgniteReactionUnsafe resolves an empty ignite_reaction window when the
// opponent's reaction mode skips it (off, or auto with no eligible opening card).
func (r *RoomSession) maybeAutoResolveIgniteReactionUnsafe() error {
	rw, stackSize, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open || rw.Trigger != "ignite_reaction" || stackSize != 0 {
		return nil
	}
	responder := oppositePlayer(rw.Actor)
	switch r.reactionModeUnsafe(responder) {
	case ReactionModeOn:
		return nil
	case ReactionModeAuto:
		if gameplay.EligibleForIgniteReactionAUTO(r.Engine.State, responder) {
			return nil
		}
	default:
	}
	if err := r.Engine.ResolveReactionStack(); err != nil {
		return err
	}
	r.reactionDeadline = time.Time{}
	r.resumeMainTurnAfterReactionUnsafe(time.Now())
	return nil
}

// maybeAutoFinalizeIgniteChainIfStuckUnsafe resolves a non-empty ignite_reaction stack when the
// seat that must respond cannot legally extend the chain, or has reaction mode off, or (auto)
// has no eligible follow-up. Caller must hold r.stateM.
func (r *RoomSession) maybeAutoFinalizeIgniteChainIfStuckUnsafe() error {
	rw, stackSize, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open || rw.Trigger != "ignite_reaction" || stackSize == 0 {
		return nil
	}
	top, ok := r.Engine.ReactionStackTopSnapshot()
	if !ok {
		return nil
	}
	next := oppositePlayer(top.Owner)
	mode := r.reactionModeUnsafe(next)
	if mode == ReactionModeOn || mode == ReactionModeAuto {
		if r.Engine.CanPlayerExtendIgniteChain(next) {
			return nil
		}
	}
	if err := r.Engine.ResolveReactionStack(); err != nil {
		return err
	}
	r.reactionDeadline = time.Time{}
	r.resumeMainTurnAfterReactionUnsafe(time.Now())
	return nil
}

// pauseMainTurnIfReactionWindowOpenUnsafe freezes the main turn deadline when a reaction window
// is waiting for the first response. Caller must hold r.stateM.
func (r *RoomSession) pauseMainTurnIfReactionWindowOpenUnsafe(now time.Time) {
	rw, stackSize, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open || stackSize != 0 {
		return
	}
	if rw.Trigger != "capture_attempt" && rw.Trigger != "ignite_reaction" {
		return
	}
	if r.pausedTurnRemaining > 0 {
		return
	}
	if r.turnDeadline.IsZero() || r.turnDeadlineFor != rw.Actor {
		return
	}
	if r.reactionDeadline.IsZero() {
		r.reactionDeadline = now.Add(r.reactionTimeout)
	}
	r.pausedTurnRemaining = r.turnDeadline.Sub(now)
	if r.pausedTurnRemaining < 0 {
		r.pausedTurnRemaining = 0
	}
	r.turnDeadline = time.Time{}
}

// syncTurnDeadlineAfterActionUnsafe keeps turn deadline aligned with the current turn right after
// gameplay actions that may advance the turn, without waiting for timeout loop ticks.
// Caller must hold r.stateM.
func (r *RoomSession) syncTurnDeadlineAfterActionUnsafe(now time.Time) {
	if r.debugPauseActive {
		return
	}
	if r.matchEnded || r.Engine.State.MulliganPhaseActive {
		r.turnDeadline = time.Time{}
		return
	}
	if r.connectedByPlayer[gameplay.PlayerA] == 0 || r.connectedByPlayer[gameplay.PlayerB] == 0 {
		r.turnDeadline = time.Time{}
		return
	}
	if r.pausedTurnRemaining > 0 {
		return
	}
	cur := r.Engine.State.CurrentTurn
	if r.turnDeadline.IsZero() || r.turnDeadlineFor != cur {
		r.resetTurnDeadlineUnsafe(now)
	}
}

// resumeMainTurnAfterReactionUnsafe restores the main turn deadline after the reaction window closes.
// Caller must hold r.stateM.
func (r *RoomSession) resumeMainTurnAfterReactionUnsafe(now time.Time) {
	if r.pausedTurnRemaining <= 0 {
		return
	}
	r.turnDeadlineFor = r.Engine.State.CurrentTurn
	r.turnDeadline = now.Add(r.pausedTurnRemaining)
	r.pausedTurnRemaining = 0
}

// noteReactionChainStartedUnsafe clears the reaction timeout while a non-empty stack is resolving.
// Caller must hold r.stateM.
func (r *RoomSession) noteReactionChainStartedUnsafe() {
	r.reactionDeadline = time.Time{}
}

// setDebugPauseUnsafe toggles room-wide debug pause and shifts active deadlines on resume.
// Caller must hold r.stateM.
func (r *RoomSession) setDebugPauseUnsafe(paused bool, now time.Time) {
	if paused {
		if r.debugPauseActive {
			return
		}
		r.debugPauseActive = true
		r.debugPauseStartedAt = now
		return
	}
	if !r.debugPauseActive {
		return
	}
	elapsed := now.Sub(r.debugPauseStartedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	if !r.turnDeadline.IsZero() {
		r.turnDeadline = r.turnDeadline.Add(elapsed)
	}
	if !r.reactionDeadline.IsZero() {
		r.reactionDeadline = r.reactionDeadline.Add(elapsed)
	}
	if !r.mulliganDeadline.IsZero() {
		r.mulliganDeadline = r.mulliganDeadline.Add(elapsed)
	}
	// Frozen main-turn slice during reactions is wall-time paused too; do not add elapsed here or
	// the UI can show more than TurnSeconds remaining (e.g. 30s + pause duration).
	r.debugPauseActive = false
	r.debugPauseStartedAt = time.Time{}
}

// Snapshot builds a compact state payload for UI synchronization.
// Call SnapshotForPlayer to get a player-specific view with private hand data.
func (r *RoomSession) Snapshot() StateSnapshotPayload {
	return r.SnapshotForPlayer("")
}

// SnapshotForPlayer builds a state payload tailored to the requesting player.
// When viewerPID is non-empty, the player's own hand cards are included in their PlayerHUDState.
func (r *RoomSession) SnapshotForPlayer(viewerPID gameplay.PlayerID) StateSnapshotPayload {
	r.evaluateMatchOutcomeUnsafe()
	s := r.Engine.State
	cA := r.connectedByPlayer[gameplay.PlayerA]
	cB := r.connectedByPlayer[gameplay.PlayerB]
	reconnectGrace := false
	var reconnectFor string
	var reconnectUntil int64
	for _, pid := range []gameplay.PlayerID{gameplay.PlayerA, gameplay.PlayerB} {
		if r.connectedByPlayer[pid] == 0 && r.disconnectTimers[pid] != nil && r.connectedByPlayer[oppositePlayer(pid)] > 0 {
			reconnectGrace = true
			reconnectFor = string(pid)
			if t, ok := r.disconnectDeadline[pid]; ok {
				reconnectUntil = t.UnixMilli()
			}
			break
		}
	}
	board := serializeBoard(r.Engine.Chess.Board)
	if r.matchEnded {
		board = serializeBoard(chess.NewGame().Board)
	}
	var mulliganReturned map[string]int
	if s.MulliganReturnedCount != nil {
		mulliganReturned = map[string]int{
			"A": s.MulliganReturnedCount[gameplay.PlayerA],
			"B": s.MulliganReturnedCount[gameplay.PlayerB],
		}
	}
	payload := StateSnapshotPayload{
		RoomID:              r.RoomID,
		RoomName:            r.RoomName,
		RoomPrivate:         r.RoomPrivate,
		RoomPassword:        r.RoomPassword,
		ConnectedA:          cA,
		ConnectedB:          cB,
		PlayerAName:         r.displayNameByPlayer[gameplay.PlayerA],
		PlayerBName:         r.displayNameByPlayer[gameplay.PlayerB],
		GameStarted:         (cA > 0 && cB > 0) || reconnectGrace,
		MulliganPhaseActive: s.MulliganPhaseActive,
		MulliganReturned:    mulliganReturned,
		AdminDebugMatch:     r.adminDebugMatch,
		DebugPauseActive:    r.debugPauseActive,
		TurnPlayer:          string(s.CurrentTurn),
		TurnSeconds:         s.TurnSeconds,
		TurnNumber:          s.TurnNumber,
		IgnitionOn:          s.IgnitionSlot.Occupied,
		ViewerPlayerID:      string(viewerPID),
		Board:               board,
		Players: []PlayerHUDState{
			playerHUDState(gameplay.PlayerA, s.Players[gameplay.PlayerA], r.sleeveByPlayer[gameplay.PlayerA], viewerPID, r.reactionModeUnsafe(gameplay.PlayerA)),
			playerHUDState(gameplay.PlayerB, s.Players[gameplay.PlayerB], r.sleeveByPlayer[gameplay.PlayerB], viewerPID, r.reactionModeUnsafe(gameplay.PlayerB)),
		},
	}
	if s.MulliganPhaseActive && !r.mulliganDeadline.IsZero() {
		payload.MulliganDeadlineUnixMs = r.mulliganDeadline.UnixMilli()
	}
	if s.IgnitionSlot.Occupied {
		payload.IgnitionCard = string(s.IgnitionSlot.Card.CardID)
		payload.IgnitionOwner = string(s.IgnitionSlot.ActivationOwner)
		payload.IgnitionTurnsRemaining = s.IgnitionSlot.TurnsRemaining
	}
	ep := r.Engine.Chess.EnPassant
	payload.EnPassant = EnPassantStateSnapshot{Valid: ep.Valid}
	if ep.Valid {
		payload.EnPassant.TargetRow = ep.Target.Row
		payload.EnPassant.TargetCol = ep.Target.Col
		payload.EnPassant.PawnRow = ep.PawnPos.Row
		payload.EnPassant.PawnCol = ep.PawnPos.Col
	}
	cr := r.Engine.Chess.CastlingRights
	payload.CastlingRights = CastlingRightsSnapshot{
		WhiteKingSide:  cr.WhiteKingSide,
		WhiteQueenSide: cr.WhiteQueenSide,
		BlackKingSide:  cr.BlackKingSide,
		BlackQueenSide: cr.BlackQueenSide,
	}
	for _, pe := range r.Engine.PendingEffects() {
		payload.PendingEffects = append(payload.PendingEffects, PendingEffectState{
			Owner:  string(pe.Owner),
			CardID: string(pe.CardID),
		})
	}
	payload.ActivationQueueSize = len(s.ResolvedQueue)
	if rw, stackSize, ok := r.Engine.ReactionWindowSnapshot(); ok {
		eligible := make([]string, 0, len(rw.EligibleTypes))
		for _, t := range rw.EligibleTypes {
			eligible = append(eligible, string(t))
		}
		rwPayload := ReactionWindowState{
			Open:          rw.Open,
			Trigger:       rw.Trigger,
			Actor:         string(rw.Actor),
			EligibleTypes: eligible,
			StackSize:     stackSize,
		}
		if top, ok := r.Engine.ReactionStackTopSnapshot(); ok {
			rwPayload.StagedCardID = string(top.Card.CardID)
			rwPayload.StagedOwner = string(top.Owner)
		}
		for _, ent := range r.Engine.ReactionStackEntries() {
			rwPayload.StackCards = append(rwPayload.StackCards, ReactionStackPreviewEntry{
				CardID: string(ent.CardID),
				Owner:  string(ent.Owner),
			})
		}
		payload.ReactionWindow = rwPayload
		if !r.reactionDeadline.IsZero() {
			payload.ReactionDeadlineUnixMs = r.reactionDeadline.UnixMilli()
		}
	}
	if pm, ok := r.Engine.PendingMove(); ok {
		payload.PendingCapture = PendingCaptureState{
			Active:  true,
			FromRow: pm.Move.From.Row,
			FromCol: pm.Move.From.Col,
			ToRow:   pm.Move.To.Row,
			ToCol:   pm.Move.To.Col,
			Actor:   string(pm.PlayerID),
		}
	}
	payload.MatchEnded = r.matchEnded
	if r.winner != "" {
		payload.Winner = string(r.winner)
	}
	if r.endReason != "" {
		payload.EndReason = r.endReason
	}
	payload.RematchA = r.rematchVotes[gameplay.PlayerA]
	payload.RematchB = r.rematchVotes[gameplay.PlayerB]
	payload.ReconnectPendingFor = reconnectFor
	payload.ReconnectDeadlineUnixMs = reconnectUntil
	if !r.matchEnded && !s.MulliganPhaseActive && (cA > 0 && cB > 0 || reconnectGrace) {
		if reconnectGrace && r.disconnectFrozenMainRemaining > 0 && r.disconnectFrozenMainFor != "" {
			if r.disconnectFrozenCarryPausedTurn {
				payload.TurnMainPausedRemainingMs = r.disconnectFrozenMainRemaining.Milliseconds()
			} else {
				payload.TurnMainDeadlineUnixMs = time.Now().Add(r.disconnectFrozenMainRemaining).UnixMilli()
			}
		} else if !r.turnDeadline.IsZero() {
			payload.TurnMainDeadlineUnixMs = r.turnDeadline.UnixMilli()
		} else if r.pausedTurnRemaining > 0 {
			payload.TurnMainPausedRemainingMs = r.pausedTurnRemaining.Milliseconds()
		}
	}
	if r.matchEnded && !r.postMatchDeadline.IsZero() {
		msLeft := time.Until(r.postMatchDeadline).Milliseconds()
		if msLeft < 0 {
			msLeft = 0
		}
		payload.PostMatchMsLeft = msLeft
	}
	return payload
}

// SnapshotSafe returns a room snapshot under room-state lock for consistency.
// Use BroadcastSnapshot to send player-specific views over WebSocket.
func (r *RoomSession) SnapshotSafe() StateSnapshotPayload {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	return r.Snapshot()
}

// SnapshotForPlayerSafe returns a player-specific snapshot under lock.
func (r *RoomSession) SnapshotForPlayerSafe(viewerPID gameplay.PlayerID) StateSnapshotPayload {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	return r.SnapshotForPlayer(viewerPID)
}

// BroadcastSnapshot sends each connected client a snapshot tailored to their player seat.
// Clients with no assigned seat receive a generic (no hand) snapshot.
func (r *RoomSession) BroadcastSnapshot() {
	r.stateM.Lock()
	snapA := r.SnapshotForPlayer(gameplay.PlayerA)
	snapB := r.SnapshotForPlayer(gameplay.PlayerB)
	snapGeneric := r.SnapshotForPlayer("")
	r.stateM.Unlock()

	r.clientsM.RLock()
	defer r.clientsM.RUnlock()
	for c := range r.clients {
		switch c.playerID {
		case gameplay.PlayerA:
			c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapA)})
		case gameplay.PlayerB:
			c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapB)})
		default:
			c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapGeneric)})
		}
	}
}

// EvaluateMatchOutcome marks checkmate/stalemate results when the board has reached a terminal state.
func (r *RoomSession) EvaluateMatchOutcome() {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	r.evaluateMatchOutcomeUnsafe()
}

// ResolveReactionTimeoutIfExpired auto-resolves an open reaction window when timeout elapses.
func (r *RoomSession) ResolveReactionTimeoutIfExpired(now time.Time) (bool, error) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if r.debugPauseActive {
		return false, nil
	}
	if r.connectedByPlayer[gameplay.PlayerA] == 0 || r.connectedByPlayer[gameplay.PlayerB] == 0 {
		r.reactionDeadline = time.Time{}
		return false, nil
	}
	rw, stackSize, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open {
		r.reactionDeadline = time.Time{}
		return false, nil
	}
	if r.reactionDeadline.IsZero() {
		// noteReactionChainStartedUnsafe clears the deadline when the first reaction is queued.
		// If we only arm reactionTimeout here, the room waits a full cycle even when the ignite
		// chain cannot be extended and ResolveReactionStack should run immediately (same as
		// maybeAutoFinalizeIgniteChainIfStuckUnsafe after queue_reaction).
		if stackSize > 0 && rw.Trigger == "ignite_reaction" {
			if err := r.maybeAutoFinalizeIgniteChainIfStuckUnsafe(); err != nil {
				return false, err
			}
			rw2, _, ok2 := r.Engine.ReactionWindowSnapshot()
			if !ok2 || !rw2.Open {
				r.reactionDeadline = time.Time{}
				r.evaluateMatchOutcomeUnsafe()
				return true, nil
			}
		}
		r.reactionDeadline = now.Add(r.reactionTimeout)
		return false, nil
	}
	if now.Before(r.reactionDeadline) {
		return false, nil
	}
	if err := r.Engine.ResolveReactionStack(); err != nil {
		return false, err
	}
	r.reactionDeadline = time.Time{}
	r.resumeMainTurnAfterReactionUnsafe(now)
	r.evaluateMatchOutcomeUnsafe()
	return true, nil
}

func (r *RoomSession) evaluateMatchOutcomeUnsafe() {
	// If the match already ended for a definitive reason, keep it.
	if r.matchEnded && r.endReason != "both_disconnected_cancelled" {
		return
	}
	// Abandonment-only endings can be superseded by the real board outcome (e.g. checkmate
	// before both websocket clients dropped without EvaluateMatchOutcome having run).
	if r.Engine.Chess.IsCheckmate(chess.White) {
		r.matchEnded = true
		r.winner = gameplay.PlayerB
		r.endReason = "checkmate"
		r.startPostMatchWindowUnsafe()
		r.lastActivity = time.Now().UTC()
		return
	}
	if r.Engine.Chess.IsCheckmate(chess.Black) {
		r.matchEnded = true
		r.winner = gameplay.PlayerA
		r.endReason = "checkmate"
		r.startPostMatchWindowUnsafe()
		r.lastActivity = time.Now().UTC()
		return
	}
	if r.Engine.Chess.IsStalemate(chess.White) || r.Engine.Chess.IsStalemate(chess.Black) {
		r.matchEnded = true
		r.winner = ""
		r.endReason = "stalemate"
		r.startPostMatchWindowUnsafe()
		r.lastActivity = time.Now().UTC()
		return
	}
	if r.matchEnded {
		return
	}
}

func (r *RoomSession) startPostMatchWindowUnsafe() {
	if !r.postMatchDeadline.IsZero() {
		return
	}
	r.postMatchDeadline = time.Now().Add(30 * time.Second)
	r.rematchVotes = map[gameplay.PlayerID]bool{}
	r.mulliganDeadline = time.Time{}
}

// TouchActivity updates the room idle timestamp after gameplay or protocol actions.
func (r *RoomSession) TouchActivity() {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	r.lastActivity = time.Now().UTC()
}

// shouldEvict reports whether an empty room can be removed after idleTTL or because the match ended.
func (r *RoomSession) shouldEvict(now time.Time, idleTTL time.Duration) bool {
	r.clientsM.RLock()
	n := len(r.clients)
	r.clientsM.RUnlock()
	if n > 0 {
		return false
	}
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if r.matchEnded {
		return true
	}
	return now.Sub(r.lastActivity) >= idleTTL
}

// shutdownTimers stops disconnect grace timers to avoid leaks after room eviction.
func (r *RoomSession) shutdownTimers() {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	for _, tm := range r.disconnectTimers {
		if tm != nil {
			tm.Stop()
		}
	}
	r.disconnectTimers = map[gameplay.PlayerID]*time.Timer{}
	r.disconnectDeadline = map[gameplay.PlayerID]time.Time{}
	r.mulliganDeadline = time.Time{}
}

// Persist stores room state using provided storage adapter.
func (r *RoomSession) Persist(ctx context.Context, store RoomStore) error {
	if store == nil {
		return nil
	}
	r.stateM.Lock()
	defer r.stateM.Unlock()
	return store.SaveRoom(ctx, r)
}

// SetPlayerDisplayNameUnsafe sets the HUD display name for a seat (call with stateM held or from Execute).
func (r *RoomSession) SetPlayerDisplayNameUnsafe(pid gameplay.PlayerID, name string) {
	if r.displayNameByPlayer == nil {
		r.displayNameByPlayer = map[gameplay.PlayerID]string{
			gameplay.PlayerA: "",
			gameplay.PlayerB: "",
		}
	}
	r.displayNameByPlayer[pid] = strings.TrimSpace(name)
}

// ensureDisconnectBudgetMapsUnsafe initializes per-seat disconnect budget maps when nil.
func (r *RoomSession) ensureDisconnectBudgetMapsUnsafe() {
	total := r.effectiveDisconnectBudgetTotal()
	if r.disconnectBudgetRemaining == nil {
		r.disconnectBudgetRemaining = map[gameplay.PlayerID]time.Duration{
			gameplay.PlayerA: total,
			gameplay.PlayerB: total,
		}
	}
	if r.disconnectSegmentStart == nil {
		r.disconnectSegmentStart = map[gameplay.PlayerID]time.Time{}
	}
}

// effectiveDisconnectBudgetTotal returns the configured match-wide disconnect budget per player.
func (r *RoomSession) effectiveDisconnectBudgetTotal() time.Duration {
	if r.DisconnectBudgetTotal > 0 {
		return r.DisconnectBudgetTotal
	}
	if r.DisconnectGrace > 0 {
		return r.DisconnectGrace
	}
	return 60 * time.Second
}

// effectiveDisconnectMinWinDelay returns the minimum delay after disconnect before a disconnect win.
func (r *RoomSession) effectiveDisconnectMinWinDelay() time.Duration {
	if r.DisconnectMinWinDelay > 0 {
		return r.DisconnectMinWinDelay
	}
	return 5 * time.Second
}

// endDisconnectSegmentUnsafe subtracts wall time since disconnectSegmentStart[pid] from budget and clears the segment.
func (r *RoomSession) endDisconnectSegmentUnsafe(pid gameplay.PlayerID, now time.Time) {
	if r.disconnectSegmentStart == nil {
		return
	}
	t0, ok := r.disconnectSegmentStart[pid]
	if !ok || t0.IsZero() {
		return
	}
	spent := now.Sub(t0)
	r.disconnectBudgetRemaining[pid] -= spent
	if r.disconnectBudgetRemaining[pid] < 0 {
		r.disconnectBudgetRemaining[pid] = 0
	}
	r.disconnectSegmentStart[pid] = time.Time{}
}

// RegisterPlayerConnection marks player as connected and clears pending disconnect timeout.
func (r *RoomSession) RegisterPlayerConnection(pid gameplay.PlayerID) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	now := time.Now().UTC()
	r.lastActivity = now
	r.ensureDisconnectBudgetMapsUnsafe()
	r.endDisconnectSegmentUnsafe(pid, now)
	r.connectedByPlayer[pid]++
	if timer, ok := r.disconnectTimers[pid]; ok && timer != nil {
		timer.Stop()
		delete(r.disconnectTimers, pid)
	}
	delete(r.disconnectDeadline, pid)
	if r.connectedByPlayer[gameplay.PlayerA] > 0 && r.connectedByPlayer[gameplay.PlayerB] > 0 {
		r.resumeClocksAfterDisconnectIfNeededUnsafe(now)
	}
}

// freezeClocksForDisconnectUnsafe snapshots active turn/reaction deadlines before pausing for a single-side disconnect.
func (r *RoomSession) freezeClocksForDisconnectUnsafe(now time.Time) {
	r.disconnectFrozenReactionRemaining = 0
	r.disconnectFrozenMainRemaining = 0
	r.disconnectFrozenMainFor = ""
	r.disconnectFrozenCarryPausedTurn = false
	if !r.reactionDeadline.IsZero() && now.Before(r.reactionDeadline) {
		r.disconnectFrozenReactionRemaining = r.reactionDeadline.Sub(now)
	}
	if !r.turnDeadline.IsZero() && now.Before(r.turnDeadline) {
		r.disconnectFrozenMainRemaining = r.turnDeadline.Sub(now)
		r.disconnectFrozenMainFor = r.turnDeadlineFor
	} else if r.pausedTurnRemaining > 0 {
		r.disconnectFrozenMainRemaining = r.pausedTurnRemaining
		r.disconnectFrozenMainFor = r.turnDeadlineFor
		r.disconnectFrozenCarryPausedTurn = true
		r.pausedTurnRemaining = 0
	}
	r.turnDeadline = time.Time{}
	r.reactionDeadline = time.Time{}
}

// resumeClocksAfterDisconnectIfNeededUnsafe restores deadlines frozen during disconnect, or arms a fresh turn clock.
func (r *RoomSession) resumeClocksAfterDisconnectIfNeededUnsafe(now time.Time) {
	hadReaction := r.disconnectFrozenReactionRemaining > 0
	if hadReaction {
		r.reactionDeadline = now.Add(r.disconnectFrozenReactionRemaining)
		r.disconnectFrozenReactionRemaining = 0
	}
	mRem := r.disconnectFrozenMainRemaining
	mFor := r.disconnectFrozenMainFor
	carry := r.disconnectFrozenCarryPausedTurn
	if mRem > 0 && mFor != "" {
		if carry {
			r.pausedTurnRemaining = mRem
			r.turnDeadlineFor = mFor
			r.turnDeadline = time.Time{}
		} else {
			r.turnDeadline = now.Add(mRem)
			r.turnDeadlineFor = mFor
			r.pausedTurnRemaining = 0
		}
		r.disconnectFrozenMainRemaining = 0
		r.disconnectFrozenCarryPausedTurn = false
		return
	}
	r.disconnectFrozenCarryPausedTurn = false
	r.disconnectFrozenMainRemaining = 0
	if !hadReaction {
		r.resetTurnDeadlineUnsafe(now)
	}
}

// clearDisconnectFrozenUnsafe drops any in-memory disconnect freeze (match end, leave, or reset).
func (r *RoomSession) clearDisconnectFrozenUnsafe() {
	r.disconnectFrozenReactionRemaining = 0
	r.disconnectFrozenMainRemaining = 0
	r.disconnectFrozenMainFor = ""
	r.disconnectFrozenCarryPausedTurn = false
}

// HandlePlayerDisconnect marks player as disconnected and applies timeout-based match ending rules.
func (r *RoomSession) HandlePlayerDisconnect(pid gameplay.PlayerID) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	r.lastActivity = time.Now().UTC()
	if r.connectedByPlayer[pid] > 0 {
		r.connectedByPlayer[pid]--
	}
	if r.connectedByPlayer[pid] == 0 {
		r.SetPlayerDisplayNameUnsafe(pid, "")
	}
	if r.matchEnded {
		return
	}
	aConnected := r.connectedByPlayer[gameplay.PlayerA] > 0
	bConnected := r.connectedByPlayer[gameplay.PlayerB] > 0
	if !aConnected && !bConnected {
		r.turnDeadline = time.Time{}
		r.clearDisconnectFrozenUnsafe()
		r.evaluateMatchOutcomeUnsafe()
		if !r.matchEnded {
			r.cancelMatchNoWinner()
		}
		return
	}
	if (pid == gameplay.PlayerA && bConnected) || (pid == gameplay.PlayerB && aConnected) {
		r.freezeClocksForDisconnectUnsafe(time.Now().UTC())
		r.scheduleDisconnectTimeout(pid)
	}
}

// HandlePlayerLeave marks an intentional room exit and immediately awards win if opponent stays.
func (r *RoomSession) HandlePlayerLeave(pid gameplay.PlayerID) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	r.handlePlayerLeaveUnsafe(pid)
}

func (r *RoomSession) handlePlayerLeaveUnsafe(pid gameplay.PlayerID) {
	r.lastActivity = time.Now().UTC()
	r.clearDisconnectFrozenUnsafe()
	if r.connectedByPlayer[pid] > 0 {
		r.connectedByPlayer[pid]--
	}
	if r.connectedByPlayer[pid] == 0 {
		r.SetPlayerDisplayNameUnsafe(pid, "")
	}
	r.turnDeadline = time.Time{}
	if timer, ok := r.disconnectTimers[pid]; ok && timer != nil {
		timer.Stop()
		delete(r.disconnectTimers, pid)
	}
	delete(r.disconnectDeadline, pid)
	if r.disconnectSegmentStart != nil {
		r.disconnectSegmentStart[pid] = time.Time{}
	}
	if r.matchEnded {
		return
	}
	winner := oppositePlayer(pid)
	if r.connectedByPlayer[winner] > 0 {
		r.matchEnded = true
		r.winner = winner
		r.endReason = "left_room"
		r.startPostMatchWindowUnsafe()
		return
	}
	r.evaluateMatchOutcomeUnsafe()
	if !r.matchEnded {
		r.cancelMatchNoWinner()
	}
}

// startMulliganDeadlineUnsafe sets the wall-clock instant when unconfirmed mulligan seats auto-keep.
// Caller must hold r.stateM.
func (r *RoomSession) startMulliganDeadlineUnsafe(now time.Time) {
	r.mulliganDeadline = now.Add(mulliganPhaseDuration)
}

// ResolveMulliganTimeoutIfExpired auto-confirms mulligan for any seat that has not locked in after the deadline.
func (r *RoomSession) ResolveMulliganTimeoutIfExpired(now time.Time) (bool, error) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if r.debugPauseActive {
		return false, nil
	}
	if r.matchEnded || !r.Engine.State.MulliganPhaseActive {
		r.mulliganDeadline = time.Time{}
		return false, nil
	}
	if r.mulliganDeadline.IsZero() || now.Before(r.mulliganDeadline) {
		return false, nil
	}
	s := r.Engine.State
	for _, pid := range []gameplay.PlayerID{gameplay.PlayerA, gameplay.PlayerB} {
		if s.MulliganConfirmed != nil && s.MulliganConfirmed[pid] {
			continue
		}
		done, err := s.ConfirmMulligan(pid, nil)
		if err != nil {
			return false, err
		}
		if done {
			if err := r.Engine.StartTurn(gameplay.PlayerA); err != nil {
				return false, err
			}
			break
		}
	}
	r.mulliganDeadline = time.Time{}
	if !r.Engine.State.MulliganPhaseActive {
		r.resetTurnDeadlineUnsafe(now)
	}
	r.lastActivity = now.UTC()
	return true, nil
}

// ResolveTurnTimeoutIfExpired applies strike+turn-pass when current turn timer expires.
func (r *RoomSession) ResolveTurnTimeoutIfExpired(now time.Time) (bool, error) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if r.debugPauseActive {
		return false, nil
	}
	if r.matchEnded {
		r.turnDeadline = time.Time{}
		return false, nil
	}
	if r.Engine.State.MulliganPhaseActive {
		r.turnDeadline = time.Time{}
		return false, nil
	}
	if r.connectedByPlayer[gameplay.PlayerA] == 0 || r.connectedByPlayer[gameplay.PlayerB] == 0 {
		r.turnDeadline = time.Time{}
		return false, nil
	}
	cur := r.Engine.State.CurrentTurn
	if r.turnDeadline.IsZero() || r.turnDeadlineFor != cur {
		if r.pausedTurnRemaining > 0 {
			return false, nil
		}
		r.resetTurnDeadlineUnsafe(now)
		return false, nil
	}
	if now.Before(r.turnDeadline) {
		return false, nil
	}
	lost, err := r.Engine.State.HandleTurnTimeout(cur)
	if err != nil {
		return false, err
	}
	if lost {
		r.matchEnded = true
		r.winner = oppositePlayer(cur)
		r.endReason = "strike_limit"
		r.startPostMatchWindowUnsafe()
		r.turnDeadline = time.Time{}
		r.lastActivity = now.UTC()
		return true, nil
	}
	if err := r.Engine.StartTurn(r.Engine.State.CurrentTurn); err != nil {
		return false, err
	}
	r.Engine.Chess.Turn = toChessColor(r.Engine.State.CurrentTurn)
	r.resetTurnDeadlineUnsafe(now)
	r.lastActivity = now.UTC()
	return true, nil
}

func (r *RoomSession) resetTurnDeadlineUnsafe(now time.Time) {
	seconds := r.Engine.State.TurnSeconds
	if seconds <= 0 {
		seconds = gameplay.DefaultTurnSeconds
	}
	r.turnDeadlineFor = r.Engine.State.CurrentTurn
	r.turnDeadline = now.Add(time.Duration(seconds) * time.Second)
	r.pausedTurnRemaining = 0
}

func toChessColor(pid gameplay.PlayerID) chess.Color {
	if pid == gameplay.PlayerA {
		return chess.White
	}
	return chess.Black
}

func (r *RoomSession) cancelMatchNoWinner() {
	r.clearDisconnectFrozenUnsafe()
	r.endReason = "both_disconnected_cancelled"
	r.matchEnded = true
	r.winner = ""
	r.startPostMatchWindowUnsafe()
	r.lastActivity = time.Now().UTC()
	for _, tm := range r.disconnectTimers {
		if tm != nil {
			tm.Stop()
		}
	}
	r.disconnectTimers = map[gameplay.PlayerID]*time.Timer{}
	r.disconnectDeadline = map[gameplay.PlayerID]time.Time{}
	if r.disconnectSegmentStart != nil {
		r.disconnectSegmentStart[gameplay.PlayerA] = time.Time{}
		r.disconnectSegmentStart[gameplay.PlayerB] = time.Time{}
	}
}

func (r *RoomSession) scheduleDisconnectTimeout(pid gameplay.PlayerID) {
	r.ensureDisconnectBudgetMapsUnsafe()
	if timer, ok := r.disconnectTimers[pid]; ok && timer != nil {
		timer.Stop()
	}
	now := time.Now()
	if r.disconnectDeadline == nil {
		r.disconnectDeadline = make(map[gameplay.PlayerID]time.Time)
	}
	budget := r.disconnectBudgetRemaining[pid]
	minD := r.effectiveDisconnectMinWinDelay()
	graceEnd := now.Add(minD)
	budgetEnd := now.Add(budget)
	winAt := graceEnd
	if budgetEnd.After(graceEnd) {
		winAt = budgetEnd
	}
	if budget <= 0 {
		winAt = graceEnd
	}
	r.disconnectSegmentStart[pid] = now
	r.disconnectDeadline[pid] = winAt
	dur := winAt.Sub(now)
	if dur < 0 {
		dur = 0
	}
	r.disconnectTimers[pid] = time.AfterFunc(dur, func() {
		r.stateM.Lock()
		defer r.stateM.Unlock()
		delete(r.disconnectDeadline, pid)
		if r.matchEnded || r.connectedByPlayer[pid] > 0 {
			return
		}
		winner := oppositePlayer(pid)
		if r.connectedByPlayer[winner] == 0 {
			return
		}
		r.endDisconnectSegmentUnsafe(pid, time.Now().UTC())
		r.matchEnded = true
		r.winner = winner
		r.endReason = "disconnect_timeout"
		r.startPostMatchWindowUnsafe()
		r.lastActivity = time.Now().UTC()
	})
}

// StayInRoomAfterMatch keeps room open with single connected player waiting for opponent.
func (r *RoomSession) StayInRoomAfterMatch(pid gameplay.PlayerID) error {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if !r.matchEnded {
		return fmt.Errorf("match is not finished")
	}
	if r.connectedByPlayer[pid] == 0 {
		return fmt.Errorf("player is not connected")
	}
	total := r.connectedByPlayer[gameplay.PlayerA] + r.connectedByPlayer[gameplay.PlayerB]
	if total != 1 {
		return fmt.Errorf("stay action requires exactly one connected player")
	}
	r.resetForNewMatchUnsafe()
	r.connectedByPlayer[pid] = 1
	r.connectedByPlayer[oppositePlayer(pid)] = 0
	r.lastActivity = time.Now().UTC()
	return nil
}

// RequestRematch records rematch vote and resets board when both players accept.
func (r *RoomSession) RequestRematch(pid gameplay.PlayerID) (bool, error) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if !r.matchEnded {
		return false, fmt.Errorf("match is not finished")
	}
	if r.connectedByPlayer[pid] == 0 {
		return false, fmt.Errorf("player is not connected")
	}
	total := r.connectedByPlayer[gameplay.PlayerA] + r.connectedByPlayer[gameplay.PlayerB]
	if total < 2 {
		return false, fmt.Errorf("rematch requires both players connected")
	}
	r.rematchVotes[pid] = true
	r.lastActivity = time.Now().UTC()
	if r.rematchVotes[gameplay.PlayerA] && r.rematchVotes[gameplay.PlayerB] {
		r.swapConnectedPlayerSidesUnsafe()
		r.resetForNewMatchUnsafe()
		return true, nil
	}
	return false, nil
}

// swapConnectedPlayerSidesUnsafe swaps connected players between A/B before rematch reset.
func (r *RoomSession) swapConnectedPlayerSidesUnsafe() {
	for client := range r.clients {
		client.playerID = oppositePlayer(client.playerID)
	}
	for key, pid := range r.Players {
		r.Players[key] = oppositePlayer(pid)
	}
	connectedA := r.connectedByPlayer[gameplay.PlayerA]
	connectedB := r.connectedByPlayer[gameplay.PlayerB]
	r.connectedByPlayer[gameplay.PlayerA] = connectedB
	r.connectedByPlayer[gameplay.PlayerB] = connectedA
	timerA, okTA := r.disconnectTimers[gameplay.PlayerA]
	timerB, okTB := r.disconnectTimers[gameplay.PlayerB]
	delete(r.disconnectTimers, gameplay.PlayerA)
	delete(r.disconnectTimers, gameplay.PlayerB)
	if okTB && timerB != nil {
		r.disconnectTimers[gameplay.PlayerA] = timerB
	}
	if okTA && timerA != nil {
		r.disconnectTimers[gameplay.PlayerB] = timerA
	}
	ddA, okDA := r.disconnectDeadline[gameplay.PlayerA]
	ddB, okDB := r.disconnectDeadline[gameplay.PlayerB]
	delete(r.disconnectDeadline, gameplay.PlayerA)
	delete(r.disconnectDeadline, gameplay.PlayerB)
	if okDB {
		r.disconnectDeadline[gameplay.PlayerA] = ddB
	}
	if okDA {
		r.disconnectDeadline[gameplay.PlayerB] = ddA
	}
	r.ensureDisconnectBudgetMapsUnsafe()
	ba := r.disconnectBudgetRemaining[gameplay.PlayerA]
	bb := r.disconnectBudgetRemaining[gameplay.PlayerB]
	r.disconnectBudgetRemaining[gameplay.PlayerA] = bb
	r.disconnectBudgetRemaining[gameplay.PlayerB] = ba
	if r.disconnectSegmentStart != nil {
		sa := r.disconnectSegmentStart[gameplay.PlayerA]
		sb := r.disconnectSegmentStart[gameplay.PlayerB]
		r.disconnectSegmentStart[gameplay.PlayerA] = sb
		r.disconnectSegmentStart[gameplay.PlayerB] = sa
	}
	nameA := r.displayNameByPlayer[gameplay.PlayerA]
	nameB := r.displayNameByPlayer[gameplay.PlayerB]
	r.displayNameByPlayer[gameplay.PlayerA] = nameB
	r.displayNameByPlayer[gameplay.PlayerB] = nameA
	if r.authUIDByPlayer != nil {
		aUID := r.authUIDByPlayer[gameplay.PlayerA]
		bUID := r.authUIDByPlayer[gameplay.PlayerB]
		r.authUIDByPlayer[gameplay.PlayerA] = bUID
		r.authUIDByPlayer[gameplay.PlayerB] = aUID
	}
}

func (r *RoomSession) resetForNewMatchUnsafe() {
	r.resetMatchEngineFromSavedDecksUnsafe(r.parent)
	r.matchEnded = false
	r.winner = ""
	r.endReason = ""
	r.postMatchDeadline = time.Time{}
	r.rematchVotes = map[gameplay.PlayerID]bool{}
	r.turnDeadline = time.Time{}
	r.turnDeadlineFor = ""
	r.pausedTurnRemaining = 0
	r.reactionDeadline = time.Time{}
	r.mulliganDeadline = time.Time{}
	r.reactionModeByPlayer = map[gameplay.PlayerID]string{
		gameplay.PlayerA: ReactionModeOn,
		gameplay.PlayerB: ReactionModeOn,
	}
	for _, tm := range r.disconnectTimers {
		if tm != nil {
			tm.Stop()
		}
	}
	r.disconnectTimers = map[gameplay.PlayerID]*time.Timer{}
	r.disconnectDeadline = map[gameplay.PlayerID]time.Time{}
	total := r.effectiveDisconnectBudgetTotal()
	r.ensureDisconnectBudgetMapsUnsafe()
	r.disconnectBudgetRemaining[gameplay.PlayerA] = total
	r.disconnectBudgetRemaining[gameplay.PlayerB] = total
	r.disconnectSegmentStart[gameplay.PlayerA] = time.Time{}
	r.disconnectSegmentStart[gameplay.PlayerB] = time.Time{}
	r.clearDisconnectFrozenUnsafe()
}

// ShouldForceClosePostMatch reports if post-match idle deadline elapsed.
func (r *RoomSession) ShouldForceClosePostMatch(now time.Time) bool {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if !r.matchEnded || r.postMatchDeadline.IsZero() {
		return false
	}
	return !now.Before(r.postMatchDeadline)
}

// CloseAllClients terminates all room client sockets.
func (r *RoomSession) CloseAllClients() {
	r.clientsM.RLock()
	clients := make([]*Client, 0, len(r.clients))
	for c := range r.clients {
		clients = append(clients, c)
	}
	r.clientsM.RUnlock()
	for _, c := range clients {
		_ = c.conn.Close()
	}
}

func oppositePlayer(pid gameplay.PlayerID) gameplay.PlayerID {
	if pid == gameplay.PlayerA {
		return gameplay.PlayerB
	}
	return gameplay.PlayerA
}

// joinSeat returns pid if that seat is free for a new connection, or an error while a reconnect grace timer is waiting for the same seat.
func (r *RoomSession) joinSeat(pid gameplay.PlayerID) (gameplay.PlayerID, error) {
	if r.connectedByPlayer[pid] == 0 && r.disconnectTimers[pid] != nil {
		return "", fmt.Errorf("waiting for disconnected player to reconnect")
	}
	return pid, nil
}

func (r *RoomSession) assignJoinPlayer(p JoinMatchPayload) (gameplay.PlayerID, error) {
	if r.RoomPrivate && strings.TrimSpace(p.Password) != r.RoomPassword {
		return "", fmt.Errorf("invalid room password")
	}
	occA := r.connectedByPlayer[gameplay.PlayerA] > 0
	occB := r.connectedByPlayer[gameplay.PlayerB] > 0
	if occA && occB {
		return "", fmt.Errorf("room is full")
	}
	raw := strings.ToLower(strings.TrimSpace(p.PieceType))
	switch raw {
	case "white":
		if occA {
			return "", fmt.Errorf("white side is already occupied")
		}
		return r.joinSeat(gameplay.PlayerA)
	case "black":
		if occB {
			return "", fmt.Errorf("black side is already occupied")
		}
		return r.joinSeat(gameplay.PlayerB)
	case "random":
		if p.PlayerID == string(gameplay.PlayerA) && !occA {
			return r.joinSeat(gameplay.PlayerA)
		}
		if p.PlayerID == string(gameplay.PlayerB) && !occB {
			return r.joinSeat(gameplay.PlayerB)
		}
		if occA {
			return r.joinSeat(gameplay.PlayerB)
		}
		if occB {
			return r.joinSeat(gameplay.PlayerA)
		}
		if time.Now().UnixNano()%2 == 0 {
			return r.joinSeat(gameplay.PlayerA)
		}
		return r.joinSeat(gameplay.PlayerB)
	}
	// Backward-compatible fallback from old playerId payload.
	if p.PlayerID == string(gameplay.PlayerB) {
		if occB {
			return "", fmt.Errorf("black side is already occupied")
		}
		return r.joinSeat(gameplay.PlayerB)
	}
	if occA {
		return "", fmt.Errorf("white side is already occupied")
	}
	return r.joinSeat(gameplay.PlayerA)
}

// graveyardPieceImportance returns a sort key for piece codes so the graveyard
// is ordered from most to least important: Q > R > B > N > P (King never captured).
func graveyardPieceImportance(code string) int {
	if len(code) < 2 {
		return 99
	}
	switch code[1] {
	case 'Q':
		return 0
	case 'R':
		return 1
	case 'B':
		return 2
	case 'N':
		return 3
	case 'P':
		return 4
	}
	return 5
}

// playerHUDState converts internal player state to transport-friendly HUD data.
// sleeve is the player's chosen sleeve color; viewerPID restricts which hand is included.
// reactionMode is off / on / auto for that seat.
func playerHUDState(pid gameplay.PlayerID, p *gameplay.PlayerState, sleeve string, viewerPID gameplay.PlayerID, reactionMode string) PlayerHUDState {
	// Build the full cooldown list (all entries sent; frontend picks first 4 for inline display).
	preview := make([]CooldownPreviewEntry, 0, len(p.Cooldowns))
	for _, cd := range p.Cooldowns {
		preview = append(preview, CooldownPreviewEntry{
			CardID:         string(cd.Card.CardID),
			ManaCost:       cd.Card.ManaCost,
			Ignition:       cd.Card.Ignition,
			Cooldown:       cd.Card.Cooldown,
			TurnsRemaining: cd.TurnsRemaining,
		})
	}
	hidden := 0
	if len(preview) > 4 {
		hidden = len(preview) - 4
	}

	// Build banished card list (most recently banished first).
	banished := make([]CardSnapshotEntry, 0, len(p.Banished))
	for i := len(p.Banished) - 1; i >= 0; i-- {
		c := p.Banished[i]
		banished = append(banished, CardSnapshotEntry{
			CardID:   string(c.CardID),
			ManaCost: c.ManaCost,
			Ignition: c.Ignition,
			Cooldown: c.Cooldown,
		})
	}

	// Build graveyard piece list ordered by importance.
	graveyard := make([]string, 0, len(p.Graveyard))
	for _, pr := range p.Graveyard {
		graveyard = append(graveyard, pr.Color+pr.Type)
	}
	// Stable sort by importance order.
	for i := 1; i < len(graveyard); i++ {
		for j := i; j > 0 && graveyardPieceImportance(graveyard[j]) < graveyardPieceImportance(graveyard[j-1]); j-- {
			graveyard[j], graveyard[j-1] = graveyard[j-1], graveyard[j]
		}
	}

	hud := PlayerHUDState{
		PlayerID:            string(pid),
		Mana:                p.Mana,
		MaxMana:             p.MaxMana,
		EnergizedMana:       p.EnergizedMana,
		MaxEnergized:        p.MaxEnergizedMana,
		HandCount:           len(p.Hand),
		CooldownCount:       len(p.Cooldowns),
		GraveyardCount:      len(p.Graveyard),
		Strikes:             p.Strikes,
		DeckCount:           len(p.Deck),
		SleeveColor:         DefaultSleeveColor(sleeve),
		BanishedCards:       banished,
		GraveyardPieces:     graveyard,
		CooldownPreview:     preview,
		CooldownHiddenCount: hidden,
		ReactionMode:        reactionMode,
	}

	// Include hand only for the owning player.
	if viewerPID == pid {
		hand := make([]CardSnapshotEntry, 0, len(p.Hand))
		for _, c := range p.Hand {
			hand = append(hand, CardSnapshotEntry{
				CardID:   string(c.CardID),
				ManaCost: c.ManaCost,
				Ignition: c.Ignition,
				Cooldown: c.Cooldown,
			})
		}
		hud.Hand = hand
	}

	return hud
}

// serializeBoard converts engine board pieces to compact string identifiers.
func serializeBoard(board [8][8]chess.Piece) [8][8]string {
	out := [8][8]string{}
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			out[r][c] = pieceCode(board[r][c])
		}
	}
	return out
}

// pieceCode maps internal piece representation to transport code (e.g. "wK", "bP").
func pieceCode(p chess.Piece) string {
	if p.IsEmpty() {
		return ""
	}
	color := "w"
	if p.Color == chess.Black {
		color = "b"
	}
	pt := ""
	switch p.Type {
	case chess.Pawn:
		pt = "P"
	case chess.Knight:
		pt = "N"
	case chess.Bishop:
		pt = "B"
	case chess.Rook:
		pt = "R"
	case chess.Queen:
		pt = "Q"
	case chess.King:
		pt = "K"
	}
	return color + pt
}
