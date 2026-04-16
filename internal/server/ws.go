package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

// Server hosts HTTP and websocket endpoints for multiplayer interactions.
type Server struct {
	upgrader   websocket.Upgrader
	rooms      map[string]*RoomSession
	roomsM     sync.RWMutex
	store      RoomStore
	auth       *AuthService
	decks      *DeckService
	telemetry  *Telemetry
	nextRoomID int
	// adminDebugMatch is true when ADMIN_DEBUG_MATCH is set (e.g. "1", "true"); enables debug_match_fixture handling.
	adminDebugMatch bool
	// userRoom maps authenticated user ID -> room ID they are currently joined to (at most one room).
	userRoom   map[uint64]string
	userRoomMu sync.Mutex
}

// Client wraps a websocket connection with room/player metadata.
type Client struct {
	conn     *websocket.Conn
	server   *Server
	room     *RoomSession
	playerID gameplay.PlayerID
	// authUserID is set when the server runs with auth and the connection presented a valid JWT (same token as /api/auth/login).
	authUserID uint64
	// connID is a unique identifier for this connection, used to scope request deduplication.
	connID      string
	writeM      sync.Mutex
	closeReason error
}

type protocolError struct {
	code    ErrorCode
	message string
}

func (e protocolError) Error() string { return e.message }

var errDuplicateRequest = errors.New("duplicate request")
var errClientLeaveMatch = errors.New("client requested leave_match close")

const duplicateRequestMessage = "request already processed"

// adminDebugMatchFromEnv reports whether ADMIN_DEBUG_MATCH is enabled (e.g. "1", "true", "yes", "on").
func adminDebugMatchFromEnv() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_DEBUG_MATCH")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// NewServer creates a websocket-capable HTTP server instance.
// When DATABASE_URL is set, PostgreSQL must connect and JWT_SECRET (min 16 chars) must be set for auth.
func NewServer() *Server {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var store RoomStore
	var auth *AuthService
	if dsn != "" {
		pgStore, err := NewPostgresRoomStoreFromEnv()
		if err != nil {
			log.Fatalf("DATABASE_URL is set but postgres connection failed: %v", err)
		}
		store = pgStore
		secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
		if len(secret) < 16 {
			log.Fatal("JWT_SECRET must be at least 16 characters when DATABASE_URL is set")
		}
		auth = NewAuthService(pgStore.DB(), []byte(secret))
	}
	s := NewServerWithStore(store)
	s.auth = auth
	if s.adminDebugMatch {
		log.Printf("ADMIN_DEBUG_MATCH enabled: WebSocket type %s is accepted", MessageDebugMatchFixture)
	}
	if s.auth != nil && store != nil {
		if pg, ok := store.(*PostgresRoomStore); ok {
			s.decks = NewDeckService(pg.DB(), s.userInActiveRoom)
			if err := s.decks.BackfillDefaultDecksForUsersWithout(context.Background()); err != nil {
				log.Printf("deck backfill: %v", err)
			}
		}
	}
	return s
}

// NewServerWithStore creates a websocket-capable HTTP server with optional persistence.
func NewServerWithStore(store RoomStore) *Server {
	nextID := 1
	if store != nil {
		if seed, err := store.NextRoomID(context.Background()); err == nil && seed > 0 {
			nextID = seed
		}
	}
	s := &Server{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		rooms:           map[string]*RoomSession{},
		store:           store,
		telemetry:       NewTelemetry(),
		nextRoomID:      nextID,
		userRoom:        map[uint64]string{},
		adminDebugMatch: adminDebugMatchFromEnv(),
	}
	go s.runReactionTimeoutLoop()
	go s.runMulliganTimeoutLoop()
	go s.runPostMatchTimeoutLoop()
	go s.runRoomCleanupLoop()
	return s
}

// Routes builds the HTTP handler mux for health and websocket endpoints.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/metrics", s.HandleMetrics)
	mux.HandleFunc("/api/rooms", s.handleListRooms)
	mux.HandleFunc("/api/auth/register", s.handleAuthRegister)
	mux.HandleFunc("/api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/auth/me", s.handleAuthMe)
	mux.HandleFunc("/api/me/lobby-deck", s.handleMeLobbyDeck)
	mux.HandleFunc("/api/decks/validate", s.handleDeckValidate)
	mux.HandleFunc("/api/decks/", s.handleDecksPath)
	mux.HandleFunc("/api/decks", s.handleDecksCollection)
	mux.HandleFunc("/ws", s.handleWS)
	mux.Handle("/", http.FileServer(http.Dir("web")))
	return mux
}

// handleHealth responds with simple service health status.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleWS upgrades HTTP connections and starts websocket read loop.
// When the server is configured with auth (DATABASE_URL + JWT_SECRET), clients must pass the same JWT as for REST:
// Authorization: Bearer <token> on the upgrade request, or ?token=<jwt> (browser-friendly).
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	var authUID uint64
	if s.auth != nil {
		raw := authTokenFromHTTP(r)
		if raw == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := s.auth.ParseToken(raw)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if _, err := s.auth.UserByID(claims.UserID); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		authUID = claims.UserID
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}
	c := &Client{
		conn:       conn,
		server:     s,
		authUserID: authUID,
		connID:     fmt.Sprintf("%016x", rand.Uint64()),
	}
	_ = c.send(Envelope{Type: MessageHello})
	c.readLoop()
}

// readLoop reads incoming websocket messages and dispatches protocol handling.
func (c *Client) readLoop() {
	defer func() {
		if c.room != nil {
			c.server.unbindUserFromRoom(c.authUserID, c.room.RoomID)
			c.room.RemoveClient(c)
			if c.playerID != "" {
				if errors.Is(c.closeReason, errClientLeaveMatch) {
					// leave_match already applied room state mutation.
				} else {
					c.room.HandlePlayerDisconnect(c.playerID)
				}
				_ = c.server.persistRoom(context.Background(), c.room)
				c.room.BroadcastSnapshot()
			}
		}
		_ = c.conn.Close()
	}()
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		env, err := DecodeEnvelope(raw)
		if err != nil {
			c.sendError(ErrorBadRequest, err.Error())
			continue
		}
		if err := c.handle(env); err != nil {
			rid := clientDebugLogRoomID(c)
			if errors.Is(err, errClientLeaveMatch) {
				c.server.matchDebugLogLine(rid, fmt.Sprintf("handler_leave type=%s id=%s player=%s", env.Type, env.ID, c.playerID))
				c.closeReason = errClientLeaveMatch
				return
			}
			var pErr protocolError
			if errors.As(err, &pErr) {
				c.server.telemetry.ObserveError(pErr.code)
				c.server.matchDebugLogLine(rid, fmt.Sprintf("protocol_err type=%s id=%s player=%s code=%s msg=%q", env.Type, env.ID, c.playerID, pErr.code, pErr.message))
				c.sendError(pErr.code, pErr.message)
				continue
			}
			c.server.telemetry.ObserveError(ErrorActionFailed)
			c.server.matchDebugLogLine(rid, fmt.Sprintf("handler_err type=%s id=%s player=%s msg=%q", env.Type, env.ID, c.playerID, err.Error()))
			c.sendError(ErrorActionFailed, err.Error())
			continue
		}
		c.server.matchDebugLogLine(clientDebugLogRoomID(c), fmt.Sprintf("ok type=%s id=%s player=%s", env.Type, env.ID, c.playerID))
	}
}

// clientDebugLogRoomID returns the room id for debug file logging, or empty before join.
func clientDebugLogRoomID(c *Client) string {
	if c == nil || c.room == nil {
		return ""
	}
	return c.room.RoomID
}

// handle routes one protocol envelope to its corresponding action.
func (c *Client) handle(env Envelope) error {
	started := time.Now()
	defer func() {
		c.server.telemetry.ObserveRequest(env.Type, time.Since(started))
	}()
	switch env.Type {
	case MessagePing:
		return c.sendAck(env, "ok", "", "")
	case MessageJoinMatch:
		return c.handleJoinMatch(env)
	case MessageLeaveMatch:
		return c.handleLeaveMatch(env)
	case MessageSubmitMove:
		return c.handleSubmitMove(env)
	case MessageIgniteCard:
		return c.handleIgniteCard(env)
	case MessageSubmitIgnitionTargets:
		return c.handleSubmitIgnitionTargets(env)
	case MessageDrawCard:
		return c.handleDrawCard(env)
	case MessageConfirmMulligan:
		return c.handleConfirmMulligan(env)
	case MessageSetReactionMode:
		return c.handleSetReactionMode(env)
	case MessageResolvePending:
		return c.handleResolvePending(env)
	case MessageQueueReaction:
		return c.handleQueueReaction(env)
	case MessageResolveReaction:
		return c.handleResolveReactions(env)
	case MessageStayInRoom:
		return c.handleStayInRoom(env)
	case MessageRequestRematch:
		return c.handleRequestRematch(env)
	case MessageDebugMatchFixture:
		return c.handleDebugMatchFixture(env)
	case MessageClientTrace:
		return c.handleClientTrace(env)
	case MessageClientFxHold:
		return c.handleClientFxHold(env)
	case MessageClientFxRelease:
		return c.handleClientFxRelease(env)
	default:
		return protocolError{code: ErrorUnknownMessageType, message: "unknown message type"}
	}
}

// handleClientTrace appends browser debug text to the server process log when ADMIN_DEBUG_MATCH is enabled.
func (c *Client) handleClientTrace(env Envelope) error {
	if !c.server.adminDebugMatch {
		return protocolError{code: ErrorDebugDisabled, message: "admin_debug_match_disabled"}
	}
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before client_trace"}
	}
	var p ClientTracePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	text := strings.TrimSpace(p.Text)
	if text == "" {
		return c.sendAck(env, "ok", "", "")
	}
	line := compactClientTraceLogLine(text)
	log.Printf("client_trace room=%s player=%s %s", c.room.RoomID, c.playerID, line)
	return c.sendAck(env, "ok", "", "")
}

// handleClientFxHold freezes match timers on the server until paired client_fx_release messages
// drain the nesting depth (one hold per client animation scope is expected).
func (c *Client) handleClientFxHold(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before client_fx_hold"}
	}
	if c.playerID == "" {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before client_fx_hold"}
	}
	now := time.Now().UTC()
	if err := c.room.Execute(func() error {
		c.room.beginClientFxHoldUnsafe(now)
		return nil
	}); err != nil {
		return err
	}
	return c.sendAck(env, "ok", "", "")
}

// handleClientFxRelease pops one level of client FX hold and shifts deadlines when the outermost
// hold ends; broadcasts a fresh state_snapshot so clients see updated timer instants.
func (c *Client) handleClientFxRelease(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before client_fx_release"}
	}
	if c.playerID == "" {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before client_fx_release"}
	}
	now := time.Now().UTC()
	if err := c.room.Execute(func() error {
		c.room.endClientFxHoldUnsafe(now)
		return nil
	}); err != nil {
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

func (c *Client) handleStayInRoom(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before stay_in_room"}
	}
	if err := c.room.StayInRoomAfterMatch(c.playerID); err != nil {
		return protocolError{code: ErrorActionFailed, message: err.Error()}
	}
	_ = c.sendAck(env, "ok", "", "")
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

func (c *Client) handleRequestRematch(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before request_rematch"}
	}
	if _, err := c.room.RequestRematch(c.playerID); err != nil {
		return protocolError{code: ErrorActionFailed, message: err.Error()}
	}
	_ = c.sendAck(env, "ok", "", "")
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

// handleLeaveMatch marks intentional player leave and closes websocket after ack.
func (c *Client) handleLeaveMatch(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before leave_match"}
	}
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		c.room.handlePlayerLeaveUnsafe(c.playerID)
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return errClientLeaveMatch
}

// ensureUserCanJoinRoom rejects join when the account is already active in a different room.
func (s *Server) ensureUserCanJoinRoom(userID uint64, roomID string) error {
	if userID == 0 {
		return nil
	}
	s.userRoomMu.Lock()
	defer s.userRoomMu.Unlock()
	if rid, ok := s.userRoom[userID]; ok && rid != roomID {
		return fmt.Errorf("already active in room %s", rid)
	}
	return nil
}

// bindUserToRoom records that this account has successfully joined the room.
func (s *Server) bindUserToRoom(userID uint64, roomID string) {
	if userID == 0 {
		return
	}
	s.userRoomMu.Lock()
	defer s.userRoomMu.Unlock()
	s.userRoom[userID] = roomID
}

// unbindUserFromRoom clears the room binding when the socket leaves (or is closing).
func (s *Server) unbindUserFromRoom(userID uint64, roomID string) {
	if userID == 0 {
		return
	}
	s.userRoomMu.Lock()
	defer s.userRoomMu.Unlock()
	if s.userRoom[userID] == roomID {
		delete(s.userRoom, userID)
	}
}

// resolveClientDisplayName returns the authenticated username for match HUD labels, or empty for guests.
func (s *Server) resolveClientDisplayName(c *Client) string {
	if s.auth == nil || c.authUserID == 0 {
		return ""
	}
	u, err := s.auth.UserByID(c.authUserID)
	if err != nil || u == nil {
		return ""
	}
	return u.Username
}

// handleJoinMatch attaches the client to a room and assigns a gameplay side.
func (c *Client) handleJoinMatch(env Envelope) error {
	var p JoinMatchPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	roomID := strings.TrimSpace(string(p.RoomID))
	if roomID == "" {
		roomID = c.server.allocateRoomID()
	} else {
		id, err := strconv.Atoi(roomID)
		if err != nil || id <= 0 {
			return protocolError{code: ErrorInvalidPayload, message: "roomId must be a positive integer or omitted"}
		}
		roomID = strconv.Itoa(id)
	}
	room, err := c.server.getOrCreateRoom(roomID, p.RoomName, p.IsPrivate, p.Password)
	if err != nil {
		return err
	}
	if c.authUserID != 0 && c.server.decks != nil {
		ok, err := c.server.decks.UserHasAnyDeck(c.authUserID)
		if err != nil {
			return protocolError{code: ErrorActionFailed, message: "deck_lookup_failed"}
		}
		if !ok {
			return protocolError{code: ErrorActionFailed, message: "no_saved_deck"}
		}
	}
	if err := c.server.ensureUserCanJoinRoom(c.authUserID, room.RoomID); err != nil {
		return protocolError{code: ErrorActionFailed, message: err.Error()}
	}
	if err := room.Execute(func() error {
		// Include the connection ID so two different clients with the same
		// envelope ID cannot collide on the room-scoped dedup map.
		requestKey := fmt.Sprintf("%s|join_match|%s|%s", room.RoomID, c.connID, env.ID)
		if env.ID != "" && !room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		selected, assignErr := room.assignJoinPlayer(p)
		if assignErr != nil {
			return assignErr
		}
		c.playerID = selected
		if room.authUIDByPlayer == nil {
			room.authUIDByPlayer = map[gameplay.PlayerID]uint64{
				gameplay.PlayerA: 0,
				gameplay.PlayerB: 0,
			}
		}
		room.authUIDByPlayer[selected] = c.authUserID
		room.Players[c.conn.RemoteAddr().String()] = c.playerID
		if dn := c.server.resolveClientDisplayName(c); dn != "" {
			room.SetPlayerDisplayNameUnsafe(selected, dn)
		}
		room.AddClient(c)
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return protocolError{code: ErrorActionFailed, message: err.Error()}
	}
	c.room = room
	c.server.bindUserToRoom(c.authUserID, room.RoomID)
	room.RegisterPlayerConnection(c.playerID)
	if err := room.MaybeRebuildEngineWithSavedDecks(c.server); err != nil {
		return protocolError{code: ErrorActionFailed, message: err.Error()}
	}
	room.EvaluateMatchOutcome()
	_ = c.server.persistRoom(context.Background(), room)
	_ = c.sendAck(env, "ok", "", "")
	room.BroadcastSnapshot()
	return nil
}

// handleSubmitMove applies a chess move using the match engine.
func (c *Client) handleSubmitMove(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before submit_move"}
	}
	var p SubmitMovePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	mv := chess.Move{
		From: chess.Pos{Row: p.FromRow, Col: p.FromCol},
		To:   chess.Pos{Row: p.ToRow, Col: p.ToCol},
	}
	if !c.room.BothPlayersConnected() {
		return protocolError{code: ErrorActionFailed, message: "waiting_for_opponent"}
	}
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		if err := c.room.Engine.SubmitMove(c.playerID, mv); err != nil {
			return err
		}
		if err := c.room.maybeAutoResolveCaptureReactionUnsafe(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	c.room.EvaluateMatchOutcome()
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

// handleIgniteCard moves a hand card into the player's ignition zone (opens reactions when applicable).
func (c *Client) handleIgniteCard(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before ignite_card"}
	}
	var p IgniteCardPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	targetPieces := make([]chess.Pos, 0, len(p.TargetPieces))
	for _, piece := range p.TargetPieces {
		targetPieces = append(targetPieces, chess.Pos{Row: piece.Row, Col: piece.Col})
	}
	if !c.room.BothPlayersConnected() {
		return protocolError{code: ErrorActionFailed, message: "waiting_for_opponent"}
	}
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		hand := c.room.Engine.State.Players[c.playerID].Hand
		if p.HandIndex < 0 || p.HandIndex >= len(hand) {
			return errors.New("invalid hand index")
		}
		cardID := hand[p.HandIndex].CardID
		if _, ok := gameplay.CardDefinitionByID(cardID); !ok {
			return errors.New("unknown card definition")
		}
		if len(targetPieces) > 0 {
			if owner, _, _, has := c.room.Engine.IgnitionTargetSnapshot(); has && owner != c.playerID {
				return errors.New("target_pieces already locked for another player")
			}
		}
		_, prevStack, hadOpenWindow := c.room.Engine.ReactionWindowSnapshot()
		if err := c.room.Engine.ActivateCardWithTargets(c.playerID, p.HandIndex, targetPieces); err != nil {
			return err
		}
		now := time.Now()
		if hadOpenWindow && prevStack == 0 {
			_, newStack, ok := c.room.Engine.ReactionWindowSnapshot()
			if ok && newStack > 0 {
				c.room.NoteReactionChainExtendedUnsafe(now)
			}
		}
		if err := c.room.maybeAutoResolveIgniteReactionUnsafe(); err != nil {
			return err
		}
		if err := c.room.maybeAutoFinalizeIgniteChainIfStuckUnsafe(); err != nil {
			return err
		}
		if err := c.room.maybeAutoFinalizeCounterChainIfStuckUnsafe(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	c.room.EvaluateMatchOutcome()
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

// handleSubmitIgnitionTargets locks board targets for a target-requiring card already in ignition.
func (c *Client) handleSubmitIgnitionTargets(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before submit_ignition_targets"}
	}
	var p SubmitIgnitionTargetsPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	targetPieces := make([]chess.Pos, 0, len(p.TargetPieces))
	for _, piece := range p.TargetPieces {
		targetPieces = append(targetPieces, chess.Pos{Row: piece.Row, Col: piece.Col})
	}
	if !c.room.BothPlayersConnected() {
		return protocolError{code: ErrorActionFailed, message: "waiting_for_opponent"}
	}
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		if owner, _, _, has := c.room.Engine.IgnitionTargetSnapshot(); has && owner != c.playerID {
			return errors.New("target_pieces already locked for another player")
		}
		if err := c.room.Engine.SubmitIgnitionTargets(c.playerID, targetPieces); err != nil {
			return err
		}
		if err := c.room.maybeAutoResolveIgniteReactionUnsafe(); err != nil {
			return err
		}
		if err := c.room.maybeAutoFinalizeIgniteChainIfStuckUnsafe(); err != nil {
			return err
		}
		if err := c.room.maybeAutoFinalizeCounterChainIfStuckUnsafe(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	c.room.EvaluateMatchOutcome()
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

// handleDrawCard pays the draw-mana cost and moves a card from the player's deck to hand.
func (c *Client) handleDrawCard(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before draw_card"}
	}
	if !c.room.BothPlayersConnected() {
		return protocolError{code: ErrorActionFailed, message: "waiting_for_opponent"}
	}
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		if err := c.room.Engine.DrawCard(c.playerID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return protocolError{code: ErrorActionFailed, message: err.Error()}
	}
	_ = c.sendAck(env, "ok", "", "")
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

// handleResolvePending resolves target-dependent pending effects.
func (c *Client) handleResolvePending(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before resolve_pending_effect"}
	}
	var p ResolvePendingPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	target := match.EffectTarget{}
	if p.PieceRow != nil && p.PieceCol != nil {
		pos := chess.Pos{Row: *p.PieceRow, Col: *p.PieceCol}
		target.PiecePos = &pos
	}
	if !c.room.BothPlayersConnected() {
		return protocolError{code: ErrorActionFailed, message: "waiting_for_opponent"}
	}
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		if err := c.room.Engine.ResolvePendingEffect(c.playerID, target); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	c.room.EvaluateMatchOutcome()
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

// handleQueueReaction queues a reaction card in the open reaction window.
func (c *Client) handleQueueReaction(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before queue_reaction"}
	}
	var p QueueReactionPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	target := match.EffectTarget{}
	if p.PieceRow != nil && p.PieceCol != nil {
		pos := chess.Pos{Row: *p.PieceRow, Col: *p.PieceCol}
		target.PiecePos = &pos
	}
	if !c.room.BothPlayersConnected() {
		return protocolError{code: ErrorActionFailed, message: "waiting_for_opponent"}
	}

	// stagingSnap holds a snapshot taken while the card is still on the reaction stack, before
	// auto-finalization resolves it. If auto-finalization will happen immediately (chain stuck),
	// the frontend would never see the staged state — we broadcast it first so the glow/fly
	// animation has a DOM element to attach to.
	type stagingSnap struct {
		A, B, Generic StateSnapshotPayload
	}
	var staging *stagingSnap

	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		_, prevStack, _ := c.room.Engine.ReactionWindowSnapshot()
		if err := c.room.Engine.QueueReactionCard(c.playerID, p.HandIndex, target); err != nil {
			return err
		}
		now := time.Now()
		// Save former responder's budget and arm new deadline for the new responder.
		// Also handles the first-reaction case (prevStack==0) by arming the initial deadline.
		c.room.NoteReactionChainExtendedUnsafe(now)
		_ = prevStack

		// Detect if auto-finalization will immediately clear the stack. If so, capture a snapshot
		// with the card still staged so the client can show it in the ignition zone during animation.
		// This must be done before maybeAutoFinalize* runs — after that the stack is empty.
		if rw, sz, ok := c.room.Engine.ReactionWindowSnapshot(); ok && rw.Open && sz > 0 {
			if top, topOK := c.room.Engine.ReactionStackTopSnapshot(); topOK {
				next := oppositePlayer(top.Owner)
				mode := c.room.reactionModeUnsafe(next)
				canExtend := (mode == ReactionModeOn || mode == ReactionModeAuto) &&
					((rw.Trigger == "ignite_reaction" && c.room.Engine.CanPlayerExtendIgniteChain(next)) ||
						(rw.Trigger == "capture_attempt" && c.room.Engine.CanPlayerExtendCaptureReactionChain(next)))
				if !canExtend {
					staging = &stagingSnap{
						A:       c.room.SnapshotForPlayer(gameplay.PlayerA),
						B:       c.room.SnapshotForPlayer(gameplay.PlayerB),
						Generic: c.room.SnapshotForPlayer(""),
					}
				}
			}
		}

		if err := c.room.maybeAutoFinalizeIgniteChainIfStuckUnsafe(); err != nil {
			return err
		}
		if err := c.room.maybeAutoFinalizeCounterChainIfStuckUnsafe(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	c.room.EvaluateMatchOutcome()
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	if err := c.sendAck(env, "queued", "", "reaction card queued"); err != nil {
		return err
	}
	// Broadcast the staged state first (card visible in ignition zone), then the resolved state.
	if staging != nil {
		c.room.broadcastPrecomputedSnapshots(staging.A, staging.B, staging.Generic)
	}
	c.room.BroadcastSnapshot()
	return nil
}

// handleResolveReactions resolves queued reactions and broadcasts state updates.
func (c *Client) handleResolveReactions(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before resolve_reactions"}
	}
	if !c.room.BothPlayersConnected() {
		return protocolError{code: ErrorActionFailed, message: "waiting_for_opponent"}
	}
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		now := time.Now()
		// Save the current responder's remaining budget before resolving.
		c.room.NoteReactionResolvedUnsafe(now)
		if err := c.room.Engine.ResolveReactionStack(); err != nil {
			return err
		}
		c.room.reactionDeadline = time.Time{}
		c.room.reactionBudgetRemaining = 0
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	c.room.EvaluateMatchOutcome()
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

// handleDebugMatchFixture applies preset decks and hands when ADMIN_DEBUG_MATCH is enabled on the server.
func (c *Client) handleDebugMatchFixture(env Envelope) error {
	if !c.server.adminDebugMatch {
		return protocolError{code: ErrorDebugDisabled, message: "admin_debug_match_disabled"}
	}
	var p DebugMatchFixturePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	if !p.TestEnvironment {
		return protocolError{code: ErrorInvalidPayload, message: "test_environment must be true"}
	}
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before debug_match_fixture"}
	}
	if p.White == nil || p.Black == nil {
		return protocolError{code: ErrorInvalidPayload, message: "white and black fixtures are required"}
	}
	if !c.room.BothPlayersConnected() {
		return protocolError{code: ErrorActionFailed, message: "waiting_for_opponent"}
	}
	if err := c.room.Execute(func() error {
		requestKey := fmt.Sprintf("%s|debug_match_fixture|%s", c.room.RoomID, env.ID)
		if env.ID != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		return c.room.ApplyDebugMatchFixture(p.White, p.Black, c.server)
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return protocolError{code: ErrorActionFailed, message: err.Error()}
	}
	_ = c.sendAck(env, "ok", "", "")
	c.room.EvaluateMatchOutcome()
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

// handleConfirmMulligan applies the Shadowverse-style mulligan for the requesting player.
func (c *Client) handleConfirmMulligan(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before confirm_mulligan"}
	}
	if !c.room.BothPlayersConnected() {
		return protocolError{code: ErrorActionFailed, message: "waiting_for_opponent"}
	}
	var p ConfirmMulliganPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		done, err := c.room.Engine.State.ConfirmMulligan(c.playerID, p.HandIndices)
		if err != nil {
			return err
		}
		if done {
			c.room.mulliganDeadline = time.Time{}
			if err := c.room.Engine.StartTurn(gameplay.PlayerA); err != nil {
				return err
			}
			c.room.resetReactionBudgetsUnsafe()
			return nil
		}
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return protocolError{code: ErrorActionFailed, message: err.Error()}
	}
	_ = c.sendAck(env, "ok", "", "")
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

// handleSetReactionMode updates the caller's reaction preference (off / on / auto) for the match.
func (c *Client) handleSetReactionMode(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before set_reaction_mode"}
	}
	var p SetReactionModePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		c.room.setReactionModeUnsafe(c.playerID, p.Mode)
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	_ = c.server.persistRoom(context.Background(), c.room)
	c.room.TouchActivity()
	c.room.BroadcastSnapshot()
	return nil
}

// send writes one envelope to the websocket connection.
func (c *Client) send(env Envelope) error {
	raw, err := EncodeEnvelope(env)
	if err != nil {
		return err
	}
	c.writeM.Lock()
	defer c.writeM.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, raw)
}

// sendError sends a protocol-formatted error payload to the websocket client.
func (c *Client) sendError(code ErrorCode, msg string) {
	_ = c.send(Envelope{
		Type:    MessageError,
		Payload: MustPayload(ErrorPayload{Code: code, Message: msg}),
	})
}

// sendAck sends a standardized acknowledgement payload for processed messages.
func (c *Client) sendAck(req Envelope, status, code, message string) error {
	return c.send(Envelope{
		ID:   req.ID,
		Type: MessageAck,
		Payload: MustPayload(AckPayload{
			RequestID:   req.ID,
			RequestType: req.Type,
			Status:      status,
			Code:        code,
			Message:     message,
		}),
	})
}

// requestKey builds a room-scoped idempotency key for request processing.
func (c *Client) requestKey(req Envelope) string {
	if c.room == nil || req.ID == "" {
		return ""
	}
	return fmt.Sprintf("%s|%s|%s|%s", c.room.RoomID, c.playerID, req.Type, req.ID)
}

// getOrCreateRoom returns an existing room or creates a new one on demand.
func (s *Server) getOrCreateRoom(roomID, roomName string, isPrivate bool, password string) (*RoomSession, error) {
	s.roomsM.RLock()
	room, ok := s.rooms[roomID]
	s.roomsM.RUnlock()
	if ok {
		room.adminDebugMatch = s.adminDebugMatch
		return room, nil
	}
	s.roomsM.Lock()
	defer s.roomsM.Unlock()
	if room, ok = s.rooms[roomID]; ok {
		room.adminDebugMatch = s.adminDebugMatch
		return room, nil
	}
	if s.store != nil {
		loaded, ok, err := s.store.LoadRoom(context.Background(), roomID)
		if err != nil {
			return nil, err
		}
		if ok {
			loaded.parent = s
			loaded.adminDebugMatch = s.adminDebugMatch
			s.rooms[roomID] = loaded
			return loaded, nil
		}
	}
	if isPrivate && strings.TrimSpace(password) == "" {
		return nil, fmt.Errorf("private room requires password")
	}
	created, err := NewRoomSessionWithName(roomID, roomName)
	if err != nil {
		return nil, err
	}
	created.parent = s
	created.adminDebugMatch = s.adminDebugMatch
	created.RoomPrivate = isPrivate
	created.RoomPassword = strings.TrimSpace(password)
	s.rooms[roomID] = created
	return created, nil
}

func (s *Server) allocateRoomID() string {
	s.roomsM.Lock()
	defer s.roomsM.Unlock()
	id := s.nextRoomID
	s.nextRoomID++
	return strconv.Itoa(id)
}

func (s *Server) runReactionTimeoutLoop() {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for now := range t.C {
		s.roomsM.RLock()
		rooms := make([]*RoomSession, 0, len(s.rooms))
		for _, room := range s.rooms {
			rooms = append(rooms, room)
		}
		s.roomsM.RUnlock()
		for _, room := range rooms {
			resolved, err := room.ResolveReactionTimeoutIfExpired(now)
			if err != nil {
				s.telemetry.ObserveError(ErrorActionFailed)
				continue
			}
			if resolved {
				_ = s.persistRoom(context.Background(), room)
				room.TouchActivity()
				room.BroadcastSnapshot()
			}
		}
	}
}

func (s *Server) runMulliganTimeoutLoop() {
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	for now := range t.C {
		s.roomsM.RLock()
		rooms := make([]*RoomSession, 0, len(s.rooms))
		for _, room := range s.rooms {
			rooms = append(rooms, room)
		}
		s.roomsM.RUnlock()
		for _, room := range rooms {
			resolvedM, err := room.ResolveMulliganTimeoutIfExpired(now)
			if err != nil {
				s.telemetry.ObserveError(ErrorActionFailed)
				continue
			}
			if resolvedM {
				_ = s.persistRoom(context.Background(), room)
				room.TouchActivity()
				room.BroadcastSnapshot()
				continue
			}
		}
	}
}

func (s *Server) runPostMatchTimeoutLoop() {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for now := range t.C {
		s.roomsM.RLock()
		ids := make([]string, 0, len(s.rooms))
		for id, room := range s.rooms {
			if room.ShouldForceClosePostMatch(now) {
				ids = append(ids, id)
			}
		}
		s.roomsM.RUnlock()
		for _, id := range ids {
			s.roomsM.Lock()
			room, ok := s.rooms[id]
			if !ok || !room.ShouldForceClosePostMatch(now) {
				s.roomsM.Unlock()
				continue
			}
			delete(s.rooms, id)
			s.roomsM.Unlock()
			room.CloseAllClients()
			room.shutdownTimers()
			if s.store != nil {
				_ = s.store.DeleteRoom(context.Background(), id)
			}
		}
	}
}
