package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	telemetry  *Telemetry
	nextRoomID int
}

// Client wraps a websocket connection with room/player metadata.
type Client struct {
	conn        *websocket.Conn
	server      *Server
	room        *RoomSession
	playerID    gameplay.PlayerID
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

// NewServer creates a websocket-capable HTTP server instance.
func NewServer() *Server {
	var store RoomStore
	if pgStore, err := NewPostgresRoomStoreFromEnv(); err == nil {
		store = pgStore
	}
	return NewServerWithStore(store)
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
		rooms:      map[string]*RoomSession{},
		store:      store,
		telemetry:  NewTelemetry(),
		nextRoomID: nextID,
	}
	go s.runReactionTimeoutLoop()
	go s.runTurnTimeoutLoop()
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
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}
	c := &Client{
		conn:   conn,
		server: s,
	}
	c.send(Envelope{Type: MessageHello})
	c.readLoop()
}

// readLoop reads incoming websocket messages and dispatches protocol handling.
func (c *Client) readLoop() {
	defer func() {
		if c.room != nil {
			c.room.RemoveClient(c)
			if c.playerID != "" {
				if errors.Is(c.closeReason, errClientLeaveMatch) {
					// leave_match already applied room state mutation.
				} else {
					c.room.HandlePlayerDisconnect(c.playerID)
				}
				_ = c.room.Persist(context.Background(), c.server.store)
				c.room.Broadcast(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(c.room.SnapshotSafe())})
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
			if errors.Is(err, errClientLeaveMatch) {
				c.closeReason = errClientLeaveMatch
				return
			}
			var pErr protocolError
			if errors.As(err, &pErr) {
				c.server.telemetry.ObserveError(pErr.code)
				c.sendError(pErr.code, pErr.message)
				continue
			}
			c.server.telemetry.ObserveError(ErrorActionFailed)
			c.sendError(ErrorActionFailed, err.Error())
		}
	}
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
	case MessageActivateCard:
		return c.handleActivateCard(env)
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
	default:
		return protocolError{code: ErrorUnknownMessageType, message: "unknown message type"}
	}
}

func (c *Client) handleStayInRoom(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before stay_in_room"}
	}
	if err := c.room.StayInRoomAfterMatch(c.playerID); err != nil {
		return protocolError{code: ErrorActionFailed, message: err.Error()}
	}
	_ = c.sendAck(env, "ok", "", "")
	_ = c.room.Persist(context.Background(), c.server.store)
	c.room.TouchActivity()
	c.room.Broadcast(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(c.room.SnapshotSafe())})
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
	_ = c.room.Persist(context.Background(), c.server.store)
	c.room.TouchActivity()
	c.room.Broadcast(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(c.room.SnapshotSafe())})
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
	_ = c.room.Persist(context.Background(), c.server.store)
	c.room.TouchActivity()
	c.room.Broadcast(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(c.room.SnapshotSafe())})
	return errClientLeaveMatch
}

// handleJoinMatch attaches the client to a room and assigns a gameplay side.
func (c *Client) handleJoinMatch(env Envelope) error {
	var p JoinMatchPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	roomID := strings.TrimSpace(p.RoomID)
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
	c.room = room
	if err := room.Execute(func() error {
		requestKey := fmt.Sprintf("%s|join_match|%s", room.RoomID, env.ID)
		if env.ID != "" && !room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		selected, assignErr := room.assignJoinPlayer(p)
		if assignErr != nil {
			return assignErr
		}
		c.playerID = selected
		room.Players[c.conn.RemoteAddr().String()] = c.playerID
		room.AddClient(c)
		return nil
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return protocolError{code: ErrorActionFailed, message: err.Error()}
	}
	room.RegisterPlayerConnection(c.playerID)
	room.EvaluateMatchOutcome()
	_ = room.Persist(context.Background(), c.server.store)
	_ = c.sendAck(env, "ok", "", "")
	room.Broadcast(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(room.SnapshotSafe())})
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
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		return c.room.Engine.SubmitMove(c.playerID, mv)
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	c.room.EvaluateMatchOutcome()
	_ = c.room.Persist(context.Background(), c.server.store)
	c.room.TouchActivity()
	c.room.Broadcast(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(c.room.SnapshotSafe())})
	return nil
}

// handleActivateCard activates a hand card for ignition.
func (c *Client) handleActivateCard(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before activate_card"}
	}
	var p ActivateCardPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		return c.room.Engine.ActivateCard(c.playerID, p.HandIndex)
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	c.room.EvaluateMatchOutcome()
	_ = c.room.Persist(context.Background(), c.server.store)
	c.room.TouchActivity()
	c.room.Broadcast(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(c.room.SnapshotSafe())})
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
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		return c.room.Engine.ResolvePendingEffect(c.playerID, target)
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	c.room.EvaluateMatchOutcome()
	_ = c.room.Persist(context.Background(), c.server.store)
	c.room.TouchActivity()
	c.room.Broadcast(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(c.room.SnapshotSafe())})
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
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		return c.room.Engine.QueueReactionCard(c.playerID, p.HandIndex, target)
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	c.room.EvaluateMatchOutcome()
	_ = c.room.Persist(context.Background(), c.server.store)
	c.room.TouchActivity()
	return c.sendAck(env, "queued", "", "reaction card queued")
}

// handleResolveReactions resolves queued reactions and broadcasts state updates.
func (c *Client) handleResolveReactions(env Envelope) error {
	if c.room == nil {
		return protocolError{code: ErrorJoinRequired, message: "join_match is required before resolve_reactions"}
	}
	if err := c.room.Execute(func() error {
		requestKey := c.requestKey(env)
		if requestKey != "" && !c.room.MarkRequestOnce(requestKey) {
			return errDuplicateRequest
		}
		return c.room.Engine.ResolveReactionStack()
	}); err != nil {
		if errors.Is(err, errDuplicateRequest) {
			return c.sendAck(env, "duplicate", "duplicate_request", duplicateRequestMessage)
		}
		return err
	}
	_ = c.sendAck(env, "ok", "", "")
	c.room.EvaluateMatchOutcome()
	_ = c.room.Persist(context.Background(), c.server.store)
	c.room.TouchActivity()
	c.room.Broadcast(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(c.room.SnapshotSafe())})
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
		return room, nil
	}
	s.roomsM.Lock()
	defer s.roomsM.Unlock()
	if room, ok = s.rooms[roomID]; ok {
		return room, nil
	}
	if s.store != nil {
		loaded, ok, err := s.store.LoadRoom(context.Background(), roomID)
		if err != nil {
			return nil, err
		}
		if ok {
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
				_ = room.Persist(context.Background(), s.store)
				room.TouchActivity()
				room.Broadcast(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(room.SnapshotSafe())})
			}
		}
	}
}

func (s *Server) runTurnTimeoutLoop() {
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
			resolved, err := room.ResolveTurnTimeoutIfExpired(now)
			if err != nil {
				s.telemetry.ObserveError(ErrorActionFailed)
				continue
			}
			if resolved {
				_ = room.Persist(context.Background(), s.store)
				room.TouchActivity()
				room.Broadcast(Envelope{Type: MessageStateSnapshot, Payload: MustPayload(room.SnapshotSafe())})
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
