package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketWithoutAuthStillConnects(t *testing.T) {
	srv := NewServerWithStore(nil)
	if srv.auth != nil {
		t.Fatal("test server should not use env-based auth")
	}
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = c.ReadMessage()
	if err != nil {
		t.Fatalf("read hello: %v", err)
	}
}

func TestWebSocketWithAuthRejectsMissingToken(t *testing.T) {
	_, auth := openAuthTestDB(t)
	srv := NewServerWithStore(nil)
	srv.auth = auth

	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 upgrade response, err=%v resp=%v", err, resp)
	}
}

func TestWebSocketWithAuthAcceptsQueryToken(t *testing.T) {
	_, auth := openAuthTestDB(t)
	u, err := auth.RegisterUser("ws_user", "ws@example.com", "password1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	tok, err := auth.IssueToken(u)
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	srv := NewServerWithStore(nil)
	srv.auth = auth
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	base := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	wsURL := base + "?token=" + url.QueryEscape(tok)

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = c.ReadMessage()
	if err != nil {
		t.Fatalf("read hello: %v", err)
	}
}

func TestWebSocketWithAuthAcceptsBearerHeader(t *testing.T) {
	_, auth := openAuthTestDB(t)
	u, err := auth.RegisterUser("ws_hdr", "wsh@example.com", "password1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	tok, err := auth.IssueToken(u)
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	srv := NewServerWithStore(nil)
	srv.auth = auth
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	h := http.Header{}
	h.Set("Authorization", "Bearer "+tok)
	c, _, err := websocket.DefaultDialer.Dial(wsURL, h)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
}
