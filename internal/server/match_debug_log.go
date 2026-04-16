package server

import (
	"log"
	"strings"
	"testing"
)

// sanitizeMatchLogSegment reduces arbitrary strings to safe filename fragments.
func sanitizeMatchLogSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unknown"
	}
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}

// matchDebugLogLine writes one UTC-prefixed line to the process log when ADMIN_DEBUG_MATCH is on.
// Session logs should capture stdout (e.g. Docker entrypoint tee); per-room files are not used.
func (s *Server) matchDebugLogLine(roomID string, line string) {
	if s == nil || !s.adminDebugMatch || testing.Testing() {
		return
	}
	rid := sanitizeMatchLogSegment(roomID)
	if rid == "" || rid == "unknown" {
		rid = "prejoin"
	}
	log.Printf("match_debug room=%s %s", rid, line)
}
