package gameplay

// EligibleForIgniteReactionAUTO reports whether pid could queue at least one opening response
// in an ignite_reaction window. Eligible opener types are:
//   - Retribution: always considered when economy rules are met.
//   - Counter: only when MaybeCaptureAttemptOnIgnition is true for the card in the opponent's ignition slot.
//   - Disruption: only when the opponent has a card in their ignition slot and economy rules are met.
//
// Per-card text conditions are not evaluated; only type and economy rules apply.
func EligibleForIgniteReactionAUTO(s *MatchState, pid PlayerID) bool {
	if EligibleForOpeningRetributionAUTO(s, pid) {
		return true
	}
	if EligibleForDisruptionReactionAUTO(s, pid) {
		return true
	}
	opp := OppositePlayer(pid)
	if oppSlot := s.Players[opp].Ignition; oppSlot.Occupied && MaybeCaptureAttemptOnIgnition(oppSlot.Card.CardID) {
		return EligibleForCaptureCounterReactionAUTO(s, pid)
	}
	return false
}
