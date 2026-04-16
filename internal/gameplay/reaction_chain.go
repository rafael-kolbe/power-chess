package gameplay

// MaybeCaptureAttemptOnIgnition reports whether the catalog marks this card as able to
// cause a capture directly from its ignition resolution (card-driven "maybe capture
// attempt"), as opposed to the chess move path that opens trigger "capture_attempt".
// When true, ignite_reaction may include Counter in eligibleTypes alongside Retribution.
// All cards currently leave this false in InitialCardCatalog until ignition capture effects exist.
func MaybeCaptureAttemptOnIgnition(cardID CardID) bool {
	if cardID == "" {
		return false
	}
	def, ok := CardDefinitionByID(cardID)
	return ok && def.MaybeCaptureAttemptOnIgnition
}

// CardClearsOpponentIgnitionForChain reports whether playing this card from hand during an
// ignite_reaction chain can negate or otherwise interact with the opponent's card in the shared
// ignition slot so another legal chain link may follow while that slot remains occupied.
//
// The catalog is authoritative; extend this set when new cards gain equivalent text.
func CardClearsOpponentIgnitionForChain(id CardID) bool {
	switch string(id) {
	case "extinguish", "stop-right-there", "mana-burn":
		return true
	default:
		return false
	}
}

// CardNegatesOpponentCounterOnCaptureChain reports whether this Counter can negate a stacked
// Counter on a capture_attempt chain (e.g. Blockade vs Counterattack).
func CardNegatesOpponentCounterOnCaptureChain(id CardID) bool {
	return id == "blockade"
}
