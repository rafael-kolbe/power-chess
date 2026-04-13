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
	disconnectDeadline map[gameplay.PlayerID]time.Time // when grace timer fires for that seat
	DisconnectGrace    time.Duration
	matchEnded         bool
	winner             gameplay.PlayerID
	endReason          string
	reactionTimeout    time.Duration
	reactionDeadline   time.Time
	turnDeadline       time.Time
	turnDeadlineFor    gameplay.PlayerID
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
		disconnectTimers:   map[gameplay.PlayerID]*time.Timer{},
		disconnectDeadline: map[gameplay.PlayerID]time.Time{},
		DisconnectGrace:    60 * time.Second,
		reactionTimeout:    30 * time.Second,
		rematchVotes:       map[gameplay.PlayerID]bool{},
		lastActivity:       time.Now().UTC(),
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
	return nil
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
	if rw, stackSize, ok := r.Engine.ReactionWindowSnapshot(); ok {
		eligible := make([]string, 0, len(rw.EligibleTypes))
		for _, t := range rw.EligibleTypes {
			eligible = append(eligible, string(t))
		}
		payload.ReactionWindow = ReactionWindowState{
			Open:          rw.Open,
			Trigger:       rw.Trigger,
			Actor:         string(rw.Actor),
			EligibleTypes: eligible,
			StackSize:     stackSize,
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
	rw, _, ok := r.Engine.ReactionWindowSnapshot()
	if !ok || !rw.Open {
		r.reactionDeadline = time.Time{}
		return false, nil
	}
	if r.reactionDeadline.IsZero() {
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
		tm.Stop()
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

// RegisterPlayerConnection marks player as connected and clears pending disconnect timeout.
func (r *RoomSession) RegisterPlayerConnection(pid gameplay.PlayerID) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	r.lastActivity = time.Now().UTC()
	r.connectedByPlayer[pid]++
	if timer, ok := r.disconnectTimers[pid]; ok {
		timer.Stop()
		delete(r.disconnectTimers, pid)
	}
	delete(r.disconnectDeadline, pid)
	if r.connectedByPlayer[gameplay.PlayerA] > 0 && r.connectedByPlayer[gameplay.PlayerB] > 0 {
		r.resetTurnDeadlineUnsafe(time.Now())
	}
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
		r.evaluateMatchOutcomeUnsafe()
		if !r.matchEnded {
			r.cancelMatchNoWinner()
		}
		return
	}
	if (pid == gameplay.PlayerA && bConnected) || (pid == gameplay.PlayerB && aConnected) {
		r.turnDeadline = time.Time{}
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
	if r.connectedByPlayer[pid] > 0 {
		r.connectedByPlayer[pid]--
	}
	if r.connectedByPlayer[pid] == 0 {
		r.SetPlayerDisplayNameUnsafe(pid, "")
	}
	r.turnDeadline = time.Time{}
	if timer, ok := r.disconnectTimers[pid]; ok {
		timer.Stop()
		delete(r.disconnectTimers, pid)
	}
	delete(r.disconnectDeadline, pid)
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
	r.lastActivity = now.UTC()
	return true, nil
}

// ResolveTurnTimeoutIfExpired applies strike+turn-pass when current turn timer expires.
func (r *RoomSession) ResolveTurnTimeoutIfExpired(now time.Time) (bool, error) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
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
}

func toChessColor(pid gameplay.PlayerID) chess.Color {
	if pid == gameplay.PlayerA {
		return chess.White
	}
	return chess.Black
}

func (r *RoomSession) cancelMatchNoWinner() {
	r.endReason = "both_disconnected_cancelled"
	r.matchEnded = true
	r.winner = ""
	r.startPostMatchWindowUnsafe()
	r.lastActivity = time.Now().UTC()
	for _, tm := range r.disconnectTimers {
		tm.Stop()
	}
	r.disconnectTimers = map[gameplay.PlayerID]*time.Timer{}
	r.disconnectDeadline = map[gameplay.PlayerID]time.Time{}
}

func (r *RoomSession) scheduleDisconnectTimeout(pid gameplay.PlayerID) {
	if timer, ok := r.disconnectTimers[pid]; ok {
		timer.Stop()
	}
	grace := r.DisconnectGrace
	deadline := time.Now().Add(grace)
	if r.disconnectDeadline == nil {
		r.disconnectDeadline = make(map[gameplay.PlayerID]time.Time)
	}
	r.disconnectDeadline[pid] = deadline
	r.disconnectTimers[pid] = time.AfterFunc(grace, func() {
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
	timerA := r.disconnectTimers[gameplay.PlayerA]
	timerB := r.disconnectTimers[gameplay.PlayerB]
	r.disconnectTimers[gameplay.PlayerA] = timerB
	r.disconnectTimers[gameplay.PlayerB] = timerA
	ddA := r.disconnectDeadline[gameplay.PlayerA]
	ddB := r.disconnectDeadline[gameplay.PlayerB]
	r.disconnectDeadline[gameplay.PlayerA] = ddB
	r.disconnectDeadline[gameplay.PlayerB] = ddA
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
	r.reactionDeadline = time.Time{}
	r.mulliganDeadline = time.Time{}
	r.reactionModeByPlayer = map[gameplay.PlayerID]string{
		gameplay.PlayerA: ReactionModeOn,
		gameplay.PlayerB: ReactionModeOn,
	}
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
