package server

import (
	"encoding/json"
	"fmt"
	"strings"
)

const clientTraceLogMaxRunes = 8000

// compactClientTraceLogLine turns a browser client_trace JSON batch into one short log line.
// Full snapshots are not echoed; use the client's JSON export for deep debugging.
func compactClientTraceLogLine(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	n := len(text)
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		prefix := text
		if len(prefix) > 160 {
			prefix = prefix[:160] + "..."
		}
		return fmt.Sprintf("bytes=%d unmarshal_err=%v prefix=%q", n, err, prefix)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "entries=%d bytes=%d ", len(items), n)
	maxShow := 16
	start := 0
	if len(items) > maxShow {
		start = len(items) - maxShow
		fmt.Fprintf(&b, "last_%d: ", maxShow)
	}
	for i := start; i < len(items); i++ {
		if i > start {
			b.WriteString(" || ")
		}
		b.WriteString(summarizeOneClientTraceEntry(items[i]))
	}
	out := b.String()
	r := []rune(out)
	if len(r) > clientTraceLogMaxRunes {
		return string(r[:clientTraceLogMaxRunes]) + "...[log_cap]"
	}
	return out
}

func summarizeOneClientTraceEntry(raw json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "?"
	}
	ts, _ := m["ts"].(string)
	if len(ts) > 19 {
		ts = ts[:19]
	}
	dir, _ := m["dir"].(string)
	if env, ok := m["envelope"].(map[string]any); ok {
		t, _ := env["type"].(string)
		id, _ := env["id"].(string)
		if t == "state_snapshot" {
			room, turn, tn, pend := snapshotPayloadBrief(env["payload"])
			return fmt.Sprintf("%s <%s snap room=%s turn=%s n=%s pend=%s id=%s", ts, dir, room, turn, tn, pend, id)
		}
		return fmt.Sprintf("%s <%s %s id=%s", ts, dir, t, id)
	}
	if t, ok := m["type"].(string); ok {
		return fmt.Sprintf("%s >%s %s", ts, dir, t)
	}
	return ts + ":evt"
}

func snapshotPayloadBrief(v any) (room, turn, turnNum, pending string) {
	pay, ok := v.(map[string]any)
	if !ok {
		return "", "", "", ""
	}
	if r, ok := pay["roomId"].(string); ok {
		room = r
	}
	if r, ok := pay["turnPlayer"].(string); ok {
		turn = r
	}
	if v, ok := pay["turnNumber"].(float64); ok {
		turnNum = fmt.Sprintf("%.0f", v)
	}
	pc, ok := pay["pendingCapture"].(map[string]any)
	if !ok {
		return room, turn, turnNum, "off"
	}
	if active, ok := pc["active"].(bool); ok && active {
		fr, fc, tr, tc := pickInt(pc, "fromRow"), pickInt(pc, "fromCol"), pickInt(pc, "toRow"), pickInt(pc, "toCol")
		return room, turn, turnNum, fmt.Sprintf("cap %d,%d-%d,%d", fr, fc, tr, tc)
	}
	return room, turn, turnNum, "off"
}

func pickInt(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return -1
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return -1
	}
}
