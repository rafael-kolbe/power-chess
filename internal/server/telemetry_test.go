package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestTelemetryObserveAndSnapshot validates request/error/latency counters.
func TestTelemetryObserveAndSnapshot(t *testing.T) {
	tm := NewTelemetry()
	tm.ObserveRequest(MessageJoinMatch, 10*time.Millisecond)
	tm.ObserveRequest(MessageJoinMatch, 20*time.Millisecond)
	tm.ObserveError(ErrorBadRequest)

	s := tm.Snapshot()
	if s.TotalRequests != 2 {
		t.Fatalf("expected total requests 2, got %d", s.TotalRequests)
	}
	if s.RequestsByType[string(MessageJoinMatch)] != 2 {
		t.Fatalf("expected join count 2")
	}
	if s.ErrorsByCode[string(ErrorBadRequest)] != 1 {
		t.Fatalf("expected bad_request count 1")
	}
	if s.HandlerAvgLatencyMs[string(MessageJoinMatch)] < 10 {
		t.Fatalf("expected avg latency >= 10ms")
	}
}

// TestMetricsEndpointReturnsJSON validates HTTP exposure for metrics.
func TestMetricsEndpointReturnsJSON(t *testing.T) {
	srv := NewServerWithStore(nil)
	srv.telemetry.ObserveRequest(MessagePing, 3*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	srv.HandleMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload TelemetrySnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode metrics failed: %v", err)
	}
	if payload.TotalRequests != 1 {
		t.Fatalf("expected total requests 1")
	}
}
