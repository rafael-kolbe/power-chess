package server

import "strings"

// Canonical reaction preference values for capture (and future) reaction windows.
const (
	ReactionModeOff  = "off"
	ReactionModeOn   = "on"
	ReactionModeAuto = "auto"
)

// NormalizeReactionMode maps client input to canonical off/on/auto. Unknown values default to on
// (preserve prior behavior: always offer reaction windows).
func NormalizeReactionMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case ReactionModeOff, "false", "0":
		return ReactionModeOff
	case ReactionModeAuto:
		return ReactionModeAuto
	default:
		return ReactionModeOn
	}
}
