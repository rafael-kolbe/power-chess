package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWebSocketJoinAndSnapshotBroadcast validates join flow and snapshot delivery.
func TestWebSocketJoinAndSnapshotBroadcast(t *testing.T) {
	srv := NewServerWithStore(nil)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	c1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial c1 failed: %v", err)
	}
	defer c1.Close()
	c2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial c2 failed: %v", err)
	}
	defer c2.Close()

	_ = c1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, _ = c1.ReadMessage() // hello
	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, _ = c2.ReadMessage() // hello

	joinA := Envelope{ID: "j1", Type: MessageJoinMatch, Payload: MustPayload(JoinMatchPayload{RoomID: "1", PieceType: "white", PlayerID: "A"})}
	joinB := Envelope{ID: "j2", Type: MessageJoinMatch, Payload: MustPayload(JoinMatchPayload{RoomID: "1", PieceType: "black", PlayerID: "B"})}

	rawA, _ := EncodeEnvelope(joinA)
	rawB, _ := EncodeEnvelope(joinB)
	if err := c1.WriteMessage(websocket.TextMessage, rawA); err != nil {
		t.Fatalf("write join A failed: %v", err)
	}
	if err := c2.WriteMessage(websocket.TextMessage, rawB); err != nil {
		t.Fatalf("write join B failed: %v", err)
	}

	assertReadType(t, c1, MessageAck)
	// c1 may receive one or more snapshots, at least one should arrive.
	assertEventuallyReadsType(t, c1, MessageStateSnapshot)
	assertEventuallyReadsType(t, c2, MessageAck)
	assertEventuallyReadsType(t, c2, MessageStateSnapshot)
}

// TestWebSocketRequestIdempotency validates duplicate requestId handling.
func TestWebSocketRequestIdempotency(t *testing.T) {
	srv := NewServerWithStore(nil)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, _ = c.ReadMessage() // hello

	join := Envelope{ID: "same-id", Type: MessageJoinMatch, Payload: MustPayload(JoinMatchPayload{RoomID: "2", PieceType: "white", PlayerID: "A"})}
	raw, _ := EncodeEnvelope(join)
	if err := c.WriteMessage(websocket.TextMessage, raw); err != nil {
		t.Fatalf("write join failed: %v", err)
	}
	assertReadType(t, c, MessageAck)
	assertEventuallyReadsType(t, c, MessageStateSnapshot)

	// Replay same request id and type.
	if err := c.WriteMessage(websocket.TextMessage, raw); err != nil {
		t.Fatalf("write duplicate join failed: %v", err)
	}

	env := assertReadType(t, c, MessageAck)
	var ack AckPayload
	if len(env.Payload) > 0 {
		_ = decodePayload(t, env, &ack)
	}
	if ack.Status != "duplicate" {
		t.Fatalf("expected duplicate ack status, got %q", ack.Status)
	}
}

// assertReadType reads one envelope and checks its type.
func assertReadType(t *testing.T, c *websocket.Conn, typ MessageType) Envelope {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	env, err := DecodeEnvelope(raw)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if env.Type != typ {
		t.Fatalf("expected %s, got %s", typ, env.Type)
	}
	return env
}

// assertEventuallyReadsType reads up to a few messages until target type appears.
func assertEventuallyReadsType(t *testing.T, c *websocket.Conn, typ MessageType) {
	t.Helper()
	for i := 0; i < 4; i++ {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, raw, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read failed while waiting for %s: %v", typ, err)
		}
		env, err := DecodeEnvelope(raw)
		if err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if env.Type == typ {
			return
		}
	}
	t.Fatalf("did not observe message type %s", typ)
}

// decodePayload decodes envelope payload into target type.
func decodePayload(t *testing.T, env Envelope, out any) error {
	t.Helper()
	return jsonUnmarshalForTest(env.Payload, out)
}

func jsonUnmarshalForTest(raw []byte, out any) error {
	return json.Unmarshal(raw, out)
}
