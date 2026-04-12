package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"power-chess/internal/gameplay"
)

// TestDebugMatchFixtureRejectedWhenEnvDisabled ensures forged test_environment payloads are refused.
func TestDebugMatchFixtureRejectedWhenEnvDisabled(t *testing.T) {
	t.Setenv("ADMIN_DEBUG_MATCH", "0")
	srv := NewServerWithStore(nil)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, _ = c.ReadMessage() // hello

	deck := gameplay.DefaultDeckPresetCardIDs()
	payload := DebugMatchFixturePayload{
		TestEnvironment: true,
		White: &DebugSideFixture{
			Deck: stringifyIDs(deck),
			Hand: []string{"knight-touch", "energy-gain", "bishop-touch"},
		},
		Black: &DebugSideFixture{
			Deck: stringifyIDs(deck),
			Hand: []string{"retaliate", "backstab", "clairvoyance"},
		},
	}
	raw, _ := EncodeEnvelope(Envelope{ID: "dbg1", Type: MessageDebugMatchFixture, Payload: MustPayload(payload)})
	if err := c.WriteMessage(websocket.TextMessage, raw); err != nil {
		t.Fatal(err)
	}
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		env, err := DecodeEnvelope(data)
		if err != nil {
			t.Fatal(err)
		}
		if env.Type == MessageError {
			var ep ErrorPayload
			_ = json.Unmarshal(env.Payload, &ep)
			if ep.Code != ErrorDebugDisabled {
				t.Fatalf("expected debug_disabled, got %q", ep.Code)
			}
			return
		}
	}
}

func stringifyIDs(ids []gameplay.CardID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

// TestDebugMatchFixtureAppliesWhenEnvEnabled runs join + debug fixture with ADMIN_DEBUG_MATCH=1.
func TestDebugMatchFixtureAppliesWhenEnvEnabled(t *testing.T) {
	t.Setenv("ADMIN_DEBUG_MATCH", "1")
	srv := NewServerWithStore(nil)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	c1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Close()
	c2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()

	for _, c := range []*websocket.Conn{c1, c2} {
		_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, _, _ = c.ReadMessage() // hello
	}

	deck := gameplay.DefaultDeckPresetCardIDs()
	joinA := Envelope{ID: "j1", Type: MessageJoinMatch, Payload: MustPayload(JoinMatchPayload{RoomID: joinRoomID("700"), PieceType: "white"})}
	joinB := Envelope{ID: "j2", Type: MessageJoinMatch, Payload: MustPayload(JoinMatchPayload{RoomID: joinRoomID("700"), PieceType: "black"})}
	rawA, _ := EncodeEnvelope(joinA)
	rawB, _ := EncodeEnvelope(joinB)
	if err := c1.WriteMessage(websocket.TextMessage, rawA); err != nil {
		t.Fatal(err)
	}
	if err := c2.WriteMessage(websocket.TextMessage, rawB); err != nil {
		t.Fatal(err)
	}

	drainUntilAck(t, c1)
	drainUntilAck(t, c2)

	fix := DebugMatchFixturePayload{
		TestEnvironment: true,
		White: &DebugSideFixture{
			Deck: stringifyIDs(deck),
			Hand: []string{"knight-touch", "energy-gain", "bishop-touch"},
		},
		Black: &DebugSideFixture{
			Deck: stringifyIDs(deck),
			Hand: []string{"retaliate", "backstab", "clairvoyance"},
		},
	}
	fixRaw, _ := EncodeEnvelope(Envelope{ID: "dbg", Type: MessageDebugMatchFixture, Payload: MustPayload(fix)})
	if err := c1.WriteMessage(websocket.TextMessage, fixRaw); err != nil {
		t.Fatal(err)
	}

	foundAck := false
	for i := 0; i < 20; i++ {
		_, data, err := c1.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		env, err := DecodeEnvelope(data)
		if err != nil {
			t.Fatal(err)
		}
		if env.Type == MessageAck {
			foundAck = true
			break
		}
		if env.Type == MessageError {
			var ep ErrorPayload
			_ = json.Unmarshal(env.Payload, &ep)
			t.Fatalf("unexpected error: %s %s", ep.Code, ep.Message)
		}
	}
	if !foundAck {
		t.Fatal("expected ack for debug_match_fixture")
	}
}

func drainUntilAck(t *testing.T, c *websocket.Conn) {
	t.Helper()
	for i := 0; i < 30; i++ {
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		env, err := DecodeEnvelope(data)
		if err != nil {
			t.Fatal(err)
		}
		if env.Type == MessageAck {
			return
		}
	}
	t.Fatal("ack not found")
}
