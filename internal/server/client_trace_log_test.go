package server

import (
	"strings"
	"testing"
)

func TestCompactClientTraceLogLineSummarizesSnapshotBatch(t *testing.T) {
	raw := `[{"ts":"2026-04-14T02:35:13.113Z","dir":"in","envelope":{"type":"state_snapshot","id":"","payload":{"roomId":"138","turnPlayer":"A","turnNumber":3,"matchEnded":false,"pendingCapture":{"active":true,"fromRow":1,"fromCol":2,"toRow":3,"toCol":4}}}}]`
	line := compactClientTraceLogLine(raw)
	if !strings.Contains(line, "entries=1") || !strings.Contains(line, "snap room=138") {
		t.Fatalf("expected compact summary, got: %s", line)
	}
	if !strings.Contains(line, "pend=cap 1,2-3,4") {
		t.Fatalf("expected pending capture coords in summary, got: %s", line)
	}
}

func TestCompactClientTraceLogLineNonJSON(t *testing.T) {
	line := compactClientTraceLogLine("not-json")
	if !strings.Contains(line, "unmarshal_err") {
		t.Fatalf("expected unmarshal note, got: %s", line)
	}
}
