package server

import (
	"context"
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
// A confirmation-only window (no eligible types) auto-resolves for all modes except ReactionModeOn,
// which waits for the player to manually confirm.
func (r *RoomSession) maybeAutoResolveIgniteReactionUnsafe() error {
	rw, stackSize, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open || rw.Trigger != "ignite_reaction" || stackSize != 0 {
		return nil
	}
	responder := oppositePlayer(rw.Actor)
	if len(rw.EligibleTypes) == 0 {
		// Confirmation-only window: only ReactionModeOn waits for manual OK.
		if r.reactionModeUnsafe(responder) == ReactionModeOn {
			return nil
		}
	} else {
		switch r.reactionModeUnsafe(responder) {
		case ReactionModeOn:
			return nil
		case ReactionModeAuto:
			if gameplay.EligibleForIgniteReactionAUTO(r.Engine.State, responder) {
				return nil
			}
		default:
		}
	}
	if err := r.Engine.ResolveReactionStack(); err != nil {
		return err
	}
	r.reactionDeadline = time.Time{}
	r.reactionBudgetRemaining = 0
	return nil
}

// maybeAutoFinalizeIgniteChainIfStuckUnsafe resolves a non-empty ignite_reaction stack when the
// seat that must respond has reaction mode off. For on/auto, chain resolution must be confirmed
// explicitly via resolve_reactions to preserve documented chain order semantics.
// Caller must hold r.stateM.
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
	if mode != ReactionModeOff {
		return nil
	}
	if err := r.Engine.ResolveReactionStack(); err != nil {
		return err
	}
	r.reactionDeadline = time.Time{}
	r.reactionBudgetRemaining = 0
	return nil
}

// maybeAutoFinalizeCounterChainIfStuckUnsafe resolves a non-empty capture_attempt stack when the
// seat that must respond has reaction mode off. For on/auto, resolution requires explicit
// resolve_reactions confirmation.
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
	if mode != ReactionModeOff {
		return nil
	}
	if err := r.Engine.ResolveReactionStack(); err != nil {
		return err
	}
	r.reactionDeadline = time.Time{}
	r.reactionBudgetRemaining = 0
	return nil
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
	// Compatibility no-op for legacy queued burn paths.
	r.Engine.FlushPendingManaBurns()
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
		if tOwner, tCardID, tPieces, hasTargets := r.Engine.IgnitionTargetSnapshot(); hasTargets {
			rwPayload.TargetingOwner = string(tOwner)
			rwPayload.TargetingCardID = string(tCardID)
			for _, tp := range tPieces {
				rwPayload.TargetPieces = append(rwPayload.TargetPieces, TargetPiecePayload{
					Row: tp.Row,
					Col: tp.Col,
				})
			}
		}
		payload.ReactionWindow = rwPayload
	}
	for _, pid := range []gameplay.PlayerID{gameplay.PlayerA, gameplay.PlayerB} {
		p := s.Players[pid]
		if !p.Ignition.Occupied {
			continue
		}
		def, ok := gameplay.CardDefinitionByID(p.Ignition.Card.CardID)
		if !ok || def.Targets == 0 {
			continue
		}
		igt := IgnitionTargetingSnapshot{
			Owner:  string(pid),
			CardID: string(p.Ignition.Card.CardID),
		}
		if _, tp, has := r.Engine.IgnitionTargetsForPlayer(pid); has {
			for _, pos := range tp {
				igt.TargetPieces = append(igt.TargetPieces, TargetPiecePayload{
					Row: pos.Row,
					Col: pos.Col,
				})
			}
		} else {
			igt.AwaitingTargetChoice = true
		}
		payload.IgnitionTargeting = igt
		break
	}
	for _, g := range r.Engine.CloneMovementGrants() {
		if g.RemainingOwnerTurns <= 0 {
			continue
		}
		payload.ActivePieceEffects = append(payload.ActivePieceEffects, ActivePieceEffectSnapshot{
			Owner:          string(g.Owner),
			CardID:         string(g.SourceCardID),
			Row:            g.Target.Row,
			Col:            g.Target.Col,
			TurnsRemaining: g.RemainingOwnerTurns,
		})
	}
	for _, mc := range r.Engine.CloneMindControlEffects() {
		if mc.RemainingTurnEnds <= 0 {
			continue
		}
		payload.ActivePieceEffects = append(payload.ActivePieceEffects, ActivePieceEffectSnapshot{
			Owner:          string(mc.Owner),
			CardID:         string(mc.SourceCardID),
			Row:            mc.Target.Row,
			Col:            mc.Target.Col,
			TurnsRemaining: mc.RemainingTurnEnds,
		})
	}
	if dtPID := r.Engine.DoubleTurnActiveFor(); dtPID != "" {
		payload.DoubleTurnActiveFor = string(dtPID)
		payload.DoubleTurnTurnsRemaining = r.Engine.DoubleTurnTurnsRemainingFor(dtPID)
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

// EvaluateMatchOutcome marks checkmate/stalemate results when the board has reached a terminal state.
func (r *RoomSession) EvaluateMatchOutcome() {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	r.evaluateMatchOutcomeUnsafe()
}

func (r *RoomSession) evaluateMatchOutcomeUnsafe() {
	stateSvc := NewGameplayStateService(r)
	// If the match already ended for a definitive reason, keep it.
	if r.matchEnded && r.endReason != "both_disconnected_cancelled" {
		return
	}
	// Abandonment-only endings can be superseded by the real board outcome (e.g. checkmate
	// before both websocket clients dropped without EvaluateMatchOutcome having run).
	if r.Engine.Chess.IsCheckmate(chess.White) {
		stateSvc.Close(gameplay.PlayerB, "checkmate")
		r.lastActivity = time.Now().UTC()
		return
	}
	if r.Engine.Chess.IsCheckmate(chess.Black) {
		stateSvc.Close(gameplay.PlayerA, "checkmate")
		r.lastActivity = time.Now().UTC()
		return
	}
	if r.Engine.Chess.IsStalemate(chess.White) || r.Engine.Chess.IsStalemate(chess.Black) {
		stateSvc.Close("", "stalemate")
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
