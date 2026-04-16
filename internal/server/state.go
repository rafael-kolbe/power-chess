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
	// disconnectFrozenReactionRemaining preserves the reaction response deadline slice while exactly one player is offline.
	disconnectFrozenReactionRemaining time.Duration
	matchEnded                        bool
	winner                            gameplay.PlayerID
	endReason                         string
	reactionTimeout                   time.Duration
	reactionDeadline                  time.Time
	// reactionBudgetA/B: per-player reaction time budget within the current turn.
	// Both reset to reactionTimeout when a new turn starts.
	reactionBudgetA time.Duration
	reactionBudgetB time.Duration
	// reactionDeadlineFor tracks which player the current reactionDeadline belongs to.
	reactionDeadlineFor gameplay.PlayerID
	// reactionBudgetRemaining carries leftover opponent reaction time across counter/ignite chain links
	// (one shared budget per reaction window; see noteReactionChainStartedUnsafe).
	reactionBudgetRemaining time.Duration
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
	// adminDebugMatch mirrors server-level ADMIN_DEBUG_MATCH for snapshot/UI capabilities.
	adminDebugMatch bool
	// clientFxHoldCount is nesting depth of client-reported visual-effect holds (each connected
	// client should pair hold/release around the same animations so timers stay fair).
	clientFxHoldCount int
	// clientFxHoldStarted is when the outermost hold began (zero if clientFxHoldCount==0).
	clientFxHoldStarted time.Time
}

const defaultRoomName = "Let's Play!"

// mulliganPhaseDuration is the window for both players to confirm opening mulligan; unconfirmed seats auto-keep.
const mulliganPhaseDuration = 15 * time.Second

// maxClientFxHoldDepth caps nested client_fx_hold calls to limit abuse.
const maxClientFxHoldDepth = 64

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
		_ = c.send(env)
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
// reaction mode skips the window (off, or auto with no eligible opening response). Caller must hold r.stateM.
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
		if gameplay.EligibleForCaptureReactionAUTO(r.Engine.State, responder) {
			return nil
		}
	default: // off
	}
	if err := r.Engine.ResolveReactionStack(); err != nil {
		return err
	}
	r.reactionDeadline = time.Time{}
	r.reactionBudgetRemaining = 0
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
	r.reactionBudgetRemaining = 0
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
	r.reactionBudgetRemaining = 0
	return nil
}

// maybeAutoFinalizeCounterChainIfStuckUnsafe resolves a non-empty capture_attempt stack when the
// seat that must respond cannot legally extend the chain under ignition/clear-card rules.
// Caller must hold r.stateM.
func (r *RoomSession) maybeAutoFinalizeCounterChainIfStuckUnsafe() error {
	rw, stackSize, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open || rw.Trigger != "capture_attempt" || stackSize == 0 {
		return nil
	}
	top, ok := r.Engine.ReactionStackTopSnapshot()
	if !ok {
		return nil
	}
	next := oppositePlayer(top.Owner)
	mode := r.reactionModeUnsafe(next)
	if mode == ReactionModeOn || mode == ReactionModeAuto {
		if r.Engine.CanPlayerExtendCaptureReactionChain(next) {
			return nil
		}
	}
	if err := r.Engine.ResolveReactionStack(); err != nil {
		return err
	}
	r.reactionDeadline = time.Time{}
	r.reactionBudgetRemaining = 0
	return nil
}

// noteReactionChainStartedUnsafe clears the reaction timeout while a non-empty stack is resolving.
// Caller must hold r.stateM.
func (r *RoomSession) noteReactionChainStartedUnsafe() {
	now := time.Now()
	if !r.reactionDeadline.IsZero() {
		rem := r.reactionDeadline.Sub(now)
		if rem < 0 {
			rem = 0
		}
		if rem > 0 {
			r.reactionBudgetRemaining = rem
		}
	}
	r.reactionDeadline = time.Time{}
}

// beginClientFxHoldUnsafe records nested client-side FX blocking; timers do not expire until
// matching releases drain the depth. Caller must hold r.stateM.
func (r *RoomSession) beginClientFxHoldUnsafe(now time.Time) {
	if r.matchEnded {
		return
	}
	if r.clientFxHoldCount >= maxClientFxHoldDepth {
		return
	}
	if r.clientFxHoldCount == 0 {
		r.clientFxHoldStarted = now
	}
	r.clientFxHoldCount++
}

// endClientFxHoldUnsafe pops one FX hold level and shifts active deadlines by wall time elapsed
// since the outermost hold began when the depth reaches zero. Caller must hold r.stateM.
func (r *RoomSession) endClientFxHoldUnsafe(now time.Time) {
	if r.clientFxHoldCount == 0 {
		return
	}
	r.clientFxHoldCount--
	if r.clientFxHoldCount > 0 {
		return
	}
	started := r.clientFxHoldStarted
	r.clientFxHoldStarted = time.Time{}
	elapsed := now.Sub(started)
	if elapsed < 0 {
		elapsed = 0
	}
	r.shiftDeadlinesAfterClientFxHoldUnsafe(elapsed)
}

// shiftDeadlinesAfterClientFxHoldUnsafe moves absolute deadlines forward after wall time
// was frozen (e.g. after client_fx_hold / client_fx_release).
// Caller must hold r.stateM.
func (r *RoomSession) shiftDeadlinesAfterClientFxHoldUnsafe(elapsed time.Duration) {
	if elapsed <= 0 {
		return
	}
	if !r.reactionDeadline.IsZero() {
		r.reactionDeadline = r.reactionDeadline.Add(elapsed)
	}
	if !r.mulliganDeadline.IsZero() {
		r.mulliganDeadline = r.mulliganDeadline.Add(elapsed)
	}
}

// resetClientFxHoldUnsafe clears hold state without shifting deadlines (e.g. both seats dropped).
// Caller must hold r.stateM.
func (r *RoomSession) resetClientFxHoldUnsafe() {
	r.clientFxHoldCount = 0
	r.clientFxHoldStarted = time.Time{}
}

// flushClientFxHoldWallTimeUnsafe applies wall elapsed since the outermost hold began and clears
// hold state. Used when one seat disconnects so frozen timeout evaluation does not strand deadlines.
// Caller must hold r.stateM.
func (r *RoomSession) flushClientFxHoldWallTimeUnsafe(now time.Time) {
	if r.clientFxHoldCount == 0 || r.clientFxHoldStarted.IsZero() {
		r.resetClientFxHoldUnsafe()
		return
	}
	elapsed := now.Sub(r.clientFxHoldStarted)
	if elapsed < 0 {
		elapsed = 0
	}
	r.shiftDeadlinesAfterClientFxHoldUnsafe(elapsed)
	r.resetClientFxHoldUnsafe()
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
		TurnPlayer:          string(s.CurrentTurn),
		TurnNumber:          s.TurnNumber,
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

// broadcastActivateCardEvents sends server→client activate_card frames (effect resolution) to every client.
func (r *RoomSession) broadcastActivateCardEvents(evts []match.ActivationFXEvent) {
	if len(evts) == 0 {
		return
	}
	r.clientsM.RLock()
	defer r.clientsM.RUnlock()
	for _, ev := range evts {
		cardType := ""
		if def, ok := gameplay.CardDefinitionByID(ev.CardID); ok {
			cardType = strings.ToLower(string(def.Type))
		}
		env := Envelope{
			Type: MessageActivateCard,
			Payload: MustPayload(ActivateCardEventPayload{
				PlayerID: string(ev.Owner),
				CardID:   string(ev.CardID),
				CardType: cardType,
				Success:  ev.Success,
			}),
		}
		for c := range r.clients {
			_ = c.send(env)
		}
	}
}

// BroadcastSnapshot sends each connected client a snapshot tailored to their player seat.
// Clients with no assigned seat receive a generic (no hand) snapshot.
func (r *RoomSession) BroadcastSnapshot() {
	r.stateM.Lock()
	evts := r.Engine.PullActivationFXEvents()
	snapA := r.SnapshotForPlayer(gameplay.PlayerA)
	snapB := r.SnapshotForPlayer(gameplay.PlayerB)
	snapGeneric := r.SnapshotForPlayer("")
	r.stateM.Unlock()

	r.broadcastActivateCardEvents(evts)

	r.clientsM.RLock()
	defer r.clientsM.RUnlock()
	for c := range r.clients {
		switch c.playerID {
		case gameplay.PlayerA:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapA)})
		case gameplay.PlayerB:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapB)})
		default:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapGeneric)})
		}
	}
}

// broadcastPrecomputedSnapshots sends pre-built player snapshots to all clients without pulling
// activation FX events. Used to broadcast an intermediate state (staged reaction card) before
// auto-finalization resolves the stack, so clients animate the card in the ignition zone.
func (r *RoomSession) broadcastPrecomputedSnapshots(snapA, snapB, snapGeneric StateSnapshotPayload) {
	r.clientsM.RLock()
	defer r.clientsM.RUnlock()
	for c := range r.clients {
		switch c.playerID {
		case gameplay.PlayerA:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapA)})
		case gameplay.PlayerB:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapB)})
		default:
			_ = c.send(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(snapGeneric)})
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
	if r.clientFxHoldCount > 0 {
		return false, nil
	}
	if r.connectedByPlayer[gameplay.PlayerA] == 0 || r.connectedByPlayer[gameplay.PlayerB] == 0 {
		r.reactionDeadline = time.Time{}
		r.reactionBudgetRemaining = 0
		return false, nil
	}
	rw, stackSize, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open {
		r.reactionDeadline = time.Time{}
		r.reactionDeadlineFor = ""
		r.reactionBudgetRemaining = 0
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
				r.reactionDeadlineFor = ""
				r.reactionBudgetRemaining = 0
				r.evaluateMatchOutcomeUnsafe()
				return true, nil
			}
		}
		if stackSize > 0 && rw.Trigger == "capture_attempt" {
			if err := r.maybeAutoFinalizeCounterChainIfStuckUnsafe(); err != nil {
				return false, err
			}
			rw2, _, ok2 := r.Engine.ReactionWindowSnapshot()
			if !ok2 || !rw2.Open {
				r.reactionDeadline = time.Time{}
				r.reactionDeadlineFor = ""
				r.reactionBudgetRemaining = 0
				r.evaluateMatchOutcomeUnsafe()
				return true, nil
			}
		}
		responder := r.currentReactionResponder()
		r.reactionDeadline = now.Add(r.reactionBudgetFor(responder))
		r.reactionDeadlineFor = responder
		return false, nil
	}
	if now.Before(r.reactionDeadline) {
		return false, nil
	}
	// Timeout expired — resolve immediately.
	if err := r.Engine.ResolveReactionStack(); err != nil {
		return false, err
	}
	r.reactionDeadline = time.Time{}
	r.reactionDeadlineFor = ""
	r.reactionBudgetRemaining = 0
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
		r.resumeReactionDeadlineAfterReconnectUnsafe(now)
	}
}

// freezeReactionDeadlineForDisconnectUnsafe snapshots the reaction response deadline before pausing for a single-side disconnect.
func (r *RoomSession) freezeReactionDeadlineForDisconnectUnsafe(now time.Time) {
	r.disconnectFrozenReactionRemaining = 0
	if !r.reactionDeadline.IsZero() && now.Before(r.reactionDeadline) {
		r.disconnectFrozenReactionRemaining = r.reactionDeadline.Sub(now)
	}
	r.reactionDeadline = time.Time{}
}

// resumeReactionDeadlineAfterReconnectUnsafe restores the reaction deadline frozen during disconnect.
func (r *RoomSession) resumeReactionDeadlineAfterReconnectUnsafe(now time.Time) {
	if r.disconnectFrozenReactionRemaining > 0 {
		r.reactionDeadline = now.Add(r.disconnectFrozenReactionRemaining)
		r.disconnectFrozenReactionRemaining = 0
	}
}

// clearDisconnectFrozenUnsafe drops any in-memory disconnect freeze (match end, leave, or reset).
func (r *RoomSession) clearDisconnectFrozenUnsafe() {
	r.disconnectFrozenReactionRemaining = 0
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
		r.resetClientFxHoldUnsafe()
		r.clearDisconnectFrozenUnsafe()
		r.evaluateMatchOutcomeUnsafe()
		if !r.matchEnded {
			r.cancelMatchNoWinner()
		}
		return
	}
	if (pid == gameplay.PlayerA && bConnected) || (pid == gameplay.PlayerB && aConnected) {
		now := time.Now().UTC()
		r.flushClientFxHoldWallTimeUnsafe(now)
		r.freezeReactionDeadlineForDisconnectUnsafe(now)
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
	r.resetClientFxHoldUnsafe()
	r.clearDisconnectFrozenUnsafe()
	if r.connectedByPlayer[pid] > 0 {
		r.connectedByPlayer[pid]--
	}
	if r.connectedByPlayer[pid] == 0 {
		r.SetPlayerDisplayNameUnsafe(pid, "")
	}
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
	if r.clientFxHoldCount > 0 {
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
		r.resetReactionBudgetsUnsafe()
	}
	r.lastActivity = now.UTC()
	return true, nil
}

func (r *RoomSession) resetReactionBudgetsUnsafe() {
	// Reset per-player reaction budgets for the new turn.
	r.reactionBudgetA = r.reactionTimeout
	r.reactionBudgetB = r.reactionTimeout
	r.reactionDeadlineFor = ""
	r.reactionBudgetRemaining = 0
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
	if err := r.resetForNewMatchUnsafe(); err != nil {
		return err
	}
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
		if err := r.resetForNewMatchUnsafe(); err != nil {
			r.rematchVotes = map[gameplay.PlayerID]bool{}
			return false, err
		}
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

func (r *RoomSession) resetForNewMatchUnsafe() error {
	if err := r.resetMatchEngineFromSavedDecksUnsafe(r.parent); err != nil {
		return err
	}
	r.matchEnded = false
	r.winner = ""
	r.endReason = ""
	r.postMatchDeadline = time.Time{}
	r.rematchVotes = map[gameplay.PlayerID]bool{}
	r.reactionDeadline = time.Time{}
	r.reactionDeadlineFor = ""
	r.reactionBudgetA = 0
	r.reactionBudgetB = 0
	r.reactionBudgetRemaining = 0
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
	return nil
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

// reactionBudgetFor returns the remaining reaction budget for the given player.
// Falls back to reactionTimeout when the budget is zero.
func (r *RoomSession) reactionBudgetFor(pid gameplay.PlayerID) time.Duration {
	switch pid {
	case gameplay.PlayerA:
		if r.reactionBudgetA > 0 {
			return r.reactionBudgetA
		}
	case gameplay.PlayerB:
		if r.reactionBudgetB > 0 {
			return r.reactionBudgetB
		}
	}
	return r.reactionTimeout
}

// saveBudgetForPlayer saves remaining reaction time to the appropriate per-player budget field.
func (r *RoomSession) saveBudgetForPlayer(pid gameplay.PlayerID, rem time.Duration) {
	switch pid {
	case gameplay.PlayerA:
		r.reactionBudgetA = rem
	case gameplay.PlayerB:
		r.reactionBudgetB = rem
	}
}

// currentReactionResponder returns the player who should respond in the current reaction window.
func (r *RoomSession) currentReactionResponder() gameplay.PlayerID {
	rw, stackSize, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open {
		return ""
	}
	if stackSize == 0 {
		return oppositePlayer(rw.Actor)
	}
	if top, okTop := r.Engine.ReactionStackTopSnapshot(); okTop {
		return oppositePlayer(top.Owner)
	}
	return oppositePlayer(rw.Actor)
}

// NoteReactionChainExtendedUnsafe saves the former responder's remaining budget, clears the
// current deadline, and arms a new deadline for the new responder (after the card was queued).
// Call this under stateM after QueueReactionCard succeeds.
func (r *RoomSession) NoteReactionChainExtendedUnsafe(now time.Time) {
	// Save remaining time for whoever was on the clock.
	if !r.reactionDeadline.IsZero() && r.reactionDeadlineFor != "" {
		rem := r.reactionDeadline.Sub(now)
		if rem < 0 {
			rem = 0
		}
		r.saveBudgetForPlayer(r.reactionDeadlineFor, rem)
	}
	// Clear old deadline.
	r.reactionDeadline = time.Time{}
	r.reactionDeadlineFor = ""
	// Arm a new deadline for the new responder.
	newResponder := r.currentReactionResponder()
	if newResponder != "" {
		r.reactionDeadline = now.Add(r.reactionBudgetFor(newResponder))
		r.reactionDeadlineFor = newResponder
	}
}

// NoteReactionResolvedUnsafe saves the former responder's remaining budget when reactions are
// manually resolved (player clicks "resolve"). Call under stateM before clearing the deadline.
func (r *RoomSession) NoteReactionResolvedUnsafe(now time.Time) {
	if !r.reactionDeadline.IsZero() && r.reactionDeadlineFor != "" {
		rem := r.reactionDeadline.Sub(now)
		if rem < 0 {
			rem = 0
		}
		r.saveBudgetForPlayer(r.reactionDeadlineFor, rem)
	}
	r.reactionDeadline = time.Time{}
	r.reactionDeadlineFor = ""
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
	// Full cooldown queue in preview (UI overlaps cards like the hand; no separate +N overflow tile).
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
		DeckCount:           len(p.Deck),
		SleeveColor:         DefaultSleeveColor(sleeve),
		BanishedCards:       banished,
		GraveyardPieces:     graveyard,
		CooldownPreview:     preview,
		CooldownHiddenCount: 0,
		ReactionMode:        reactionMode,
	}
	if p.Ignition.Occupied {
		hud.IgnitionOn = true
		hud.IgnitionCard = string(p.Ignition.Card.CardID)
		hud.IgnitionTurnsRemaining = p.Ignition.TurnsRemaining
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
