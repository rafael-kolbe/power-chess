package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

// TelemetrySnapshot is a transport shape for basic server observability.
type TelemetrySnapshot struct {
	UptimeSeconds        int64            `json:"uptimeSeconds"`
	TotalRequests        int64            `json:"totalRequests"`
	RequestsByType       map[string]int64 `json:"requestsByType"`
	ErrorsByCode         map[string]int64 `json:"errorsByCode"`
	HandlerAvgLatencyMs  map[string]int64 `json:"handlerAvgLatencyMs"`
	HandlerLastLatencyMs map[string]int64 `json:"handlerLastLatencyMs"`
}

type latencyAggregate struct {
	totalNanos int64
	count      int64
	lastNanos  int64
}

// Telemetry stores in-memory request/error/latency counters.
type Telemetry struct {
	startedAt time.Time
	m         sync.Mutex
	totalReq  int64
	byType    map[MessageType]int64
	byCode    map[ErrorCode]int64
	latency   map[MessageType]latencyAggregate
}

// NewTelemetry creates telemetry with zeroed counters.
func NewTelemetry() *Telemetry {
	return &Telemetry{
		startedAt: time.Now().UTC(),
		byType:    map[MessageType]int64{},
		byCode:    map[ErrorCode]int64{},
		latency:   map[MessageType]latencyAggregate{},
	}
}

// ObserveRequest records one processed request and its handler latency.
func (t *Telemetry) ObserveRequest(mt MessageType, d time.Duration) {
	t.m.Lock()
	defer t.m.Unlock()
	t.totalReq++
	t.byType[mt]++
	agg := t.latency[mt]
	agg.count++
	agg.totalNanos += d.Nanoseconds()
	agg.lastNanos = d.Nanoseconds()
	t.latency[mt] = agg
}

// ObserveError records one protocol error code.
func (t *Telemetry) ObserveError(code ErrorCode) {
	t.m.Lock()
	defer t.m.Unlock()
	t.byCode[code]++
}

// Snapshot returns a stable copy for HTTP exposure.
func (t *Telemetry) Snapshot() TelemetrySnapshot {
	t.m.Lock()
	defer t.m.Unlock()
	out := TelemetrySnapshot{
		UptimeSeconds:        int64(time.Since(t.startedAt).Seconds()),
		TotalRequests:        t.totalReq,
		RequestsByType:       map[string]int64{},
		ErrorsByCode:         map[string]int64{},
		HandlerAvgLatencyMs:  map[string]int64{},
		HandlerLastLatencyMs: map[string]int64{},
	}
	for k, v := range t.byType {
		out.RequestsByType[string(k)] = v
	}
	for k, v := range t.byCode {
		out.ErrorsByCode[string(k)] = v
	}
	for k, v := range t.latency {
		if v.count > 0 {
			out.HandlerAvgLatencyMs[string(k)] = (v.totalNanos / v.count) / int64(time.Millisecond)
		}
		out.HandlerLastLatencyMs[string(k)] = v.lastNanos / int64(time.Millisecond)
	}
	return out
}

// HandleMetrics serves JSON telemetry in a deterministic key order.
func (s *Server) HandleMetrics(w http.ResponseWriter, _ *http.Request) {
	if s.telemetry == nil {
		http.Error(w, "telemetry unavailable", http.StatusServiceUnavailable)
		return
	}
	snap := s.telemetry.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(stableSnapshot(snap))
}

func stableSnapshot(in TelemetrySnapshot) TelemetrySnapshot {
	out := in
	out.RequestsByType = stableMap(in.RequestsByType)
	out.ErrorsByCode = stableMap(in.ErrorsByCode)
	out.HandlerAvgLatencyMs = stableMap(in.HandlerAvgLatencyMs)
	out.HandlerLastLatencyMs = stableMap(in.HandlerLastLatencyMs)
	return out
}

func stableMap(m map[string]int64) map[string]int64 {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]int64, len(m))
	for _, k := range keys {
		out[k] = m[k]
	}
	return out
}
