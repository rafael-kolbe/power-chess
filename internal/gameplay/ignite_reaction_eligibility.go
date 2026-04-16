package gameplay

// EligibleForIgniteReactionAUTO reports whether pid could queue at least one opening response
// in an ignite_reaction window: Retribution always qualifies under economy rules; Counter
// qualifies only when MaybeCaptureAttemptOnIgnition is true for the card in the opponent's
// ignition slot.
//
// Per-card text conditions are not evaluated; only type and economy rules apply.
func EligibleForIgniteReactionAUTO(s *MatchState, pid PlayerID) bool {
	if EligibleForOpeningRetributionAUTO(s, pid) {
		return true
	}
	opp := OppositePlayer(pid)
	if oppSlot := s.Players[opp].Ignition; oppSlot.Occupied && MaybeCaptureAttemptOnIgnition(oppSlot.Card.CardID) {
		return EligibleForCaptureCounterReactionAUTO(s, pid)
	}
	return false
}
