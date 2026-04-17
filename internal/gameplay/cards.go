package gameplay

//go:generate go run ../../cmd/export-card-metadata -out ../../web/card-metadata.gen.js

type CardType string

const (
	CardTypePower       CardType = "Power"
	CardTypeRetribution CardType = "Retribution"
	CardTypeCounter     CardType = "Counter"
	CardTypeContinuous  CardType = "Continuous"
	// CardTypeDisruption marks cards that can be activated on the owner's turn OR as a reaction
	// in any ignite_reaction window (except in response to a Counter card), provided the opponent
	// has a card in their ignition slot.
	CardTypeDisruption CardType = "Disruption"
)

type CardDefinition struct {
	ID          CardID
	Name        string
	Type        CardType
	Description string
	Cost        int
	Ignition    int
	Cooldown    int
	Targets     int
	// EffectDuration is how many of the owner's turns an on-board effect lasts (e.g. movement grant). Zero means unused / instant for this card.
	EffectDuration int
	Example        string
	Limit          int
	// MaybeCaptureAttemptOnIgnition marks whether this Power or Continuous card's
	// resolved ignition can directly cause a capture (card-driven "maybe capture attempt"),
	// distinct from chess `capture_attempt`. When true, the server includes Counter in
	// ignite_reaction eligibleTypes alongside Retribution. Keep false until resolvers
	// implement ignition-driven captures (e.g. Backstab).
	MaybeCaptureAttemptOnIgnition bool
}

const DefaultCardLimit = 3

// InitialCardCatalog mirrors Cards.md and is the single source of truth for initial cards.
func InitialCardCatalog() []CardDefinition {
	return []CardDefinition{
		{ID: "knight-touch", Name: "Knight Touch", Type: CardTypePower, Description: "Give any piece you control, except the king or a knight, the ability to move as if it were a knight for one turn.", Cost: 3, Ignition: 0, Cooldown: 2, Targets: 1, EffectDuration: 1, Example: "1. You have a pawn on e4.\n2. You activate \"Knight Touch\".\n3. You move the pawn to f6.", Limit: DefaultCardLimit},
		{ID: "double-turn", Name: "Double Turn", Type: CardTypePower, Description: "Give yourself 1 extra move for one turn.", Cost: 6, Ignition: 2, Cooldown: 9, Targets: 0, EffectDuration: 1, Example: "1. You have a pawn on e4.\n2. You activate \"Double Turn\".\n3. Ignition succeeds 2 turns from now.\n4. You move the pawn to e5.\n5. Then you capture a pawn on f6 with the pawn on e5.", Limit: DefaultCardLimit},
		{ID: "mana-burn", Name: "Mana Burn", Type: CardTypeRetribution, Description: "Target an ignited card from your opponent, burn x mana from your opponent, x being the mana cost of the target card.", Cost: 1, Ignition: 0, Cooldown: 3, Example: "1. Opponent activates \"Knight Touch\"\n2. You activate Mana Burn as retribution.\n3. You burn 3 mana from your opponent.", Limit: DefaultCardLimit},
		{ID: "energy-gain", Name: "Energy Gain", Type: CardTypePower, Description: "Gain 4 mana.", Cost: 0, Ignition: 1, Cooldown: 2, Example: "1. You currently have 2 mana.\n2. You activate \"Energy Gain\".\n3. Ignition succeeds the next turn.\n4. You gain 4 mana.", Limit: DefaultCardLimit},
		{ID: "piece-swap", Name: "Piece Swap", Type: CardTypePower, Description: "Swap positions of one piece you control with one piece your opponent controls up to 2 squares apart, except the king.", Cost: 6, Ignition: 1, Cooldown: 6, Example: "1. You control a pawn on a3.\n2. You activate \"Piece Swap\".\n3. Ignition succeeds the next turn.\n4. Opponent controls a rook on a1.\n5. Next turn you swap the pawn on a3 with the rook on a1.\n6. The pawn on a3 is now on a1 and the rook on a1 is now on a3.\n7. The pawn now on a1 can promote.", Limit: DefaultCardLimit},
		{ID: "mind-control", Name: "Mind Control", Type: CardTypePower, Description: "Target a piece your opponent controls, except the king or a queen, and take control of it for three turns. If the piece is captured, it goes to your opponent's captures.", Cost: 7, Ignition: 2, Cooldown: 10, Example: "1. You activate \"Mind Control\".\n2. Ignition succeeds 2 turns later.\n3. Opponent controls a rook on a1.\n4. You take control of the rook on a1 for 3 turns.", Limit: DefaultCardLimit},
		{ID: "zip-line", Name: "Zip Line", Type: CardTypePower, Description: "Target a piece you control, except the king, move it to an empty square in the same row.", Cost: 4, Ignition: 0, Cooldown: 4, Example: "1. You control a bishop on b2.\n2. You activate \"Zip Line\".\n3. The bishop on b2 moves to g2.", Limit: DefaultCardLimit},
		{ID: "sacrifice-of-the-masses", Name: "Sacrifice of the Masses", Type: CardTypePower, Description: "Target a pawn you control, sacrifice it to gain 6 mana and draw 2 cards.", Cost: 0, Ignition: 0, Cooldown: 10, Example: "1. You control a pawn on a3.\n2. You activate \"Sacrifice of the Masses\".\n3. The pawn on a3 is sacrificed and you gain 6 mana and draw 2 cards.", Limit: DefaultCardLimit},
		{ID: "archmage-arsenal", Name: "Archmage Arsenal", Type: CardTypePower, Description: "Search your deck for a \"Power\" card that costs 3 mana or less, except \"Archmage Arsenal\", add it to your hand.", Cost: 1, Ignition: 0, Cooldown: 2, Example: "1. You activate \"Archmage Arsenal\".\n2. You search for \"Knight Touch\" in your deck and add it to your hand.", Limit: DefaultCardLimit},
		{ID: "rook-touch", Name: "Rook Touch", Type: CardTypePower, Description: "Give any piece you control, except the king or a rook, the ability to move as if it were a rook for one turn. If the target is a pawn, it may move only 1 square when using this movement.", Cost: 3, Ignition: 0, Cooldown: 2, Targets: 1, EffectDuration: 1, Example: "1. You have a knight on b2.\n2. You activate \"Rook Touch\".\n3. You move the knight to b7.", Limit: DefaultCardLimit},
		{ID: "bishop-touch", Name: "Bishop Touch", Type: CardTypePower, Description: "Give any piece you control, except the king or a bishop, the ability to move as if it were a bishop for one turn. If the target is a pawn, it may move only 1 square when using this movement.", Cost: 3, Ignition: 0, Cooldown: 2, Targets: 1, EffectDuration: 1, Example: "1. You have a rook on b2.\n2. You activate \"Bishop Touch\".\n3. You move the rook to c3.", Limit: DefaultCardLimit},
		{ID: "retaliate", Name: "Retaliate", Type: CardTypeRetribution, Description: "Target a card on your opponent's cooldown slot, burn x mana from your opponent, x being the mana cost of the targeted card, and if you do, activate that card (in your ignition slot) for yourself.", Cost: 2, Ignition: 0, Cooldown: 9, Example: "1. Opponent has a \"Knight Touch\" on their cooldown slot.\n2. Opponent activates any \"Power\" card.\n3. You activate \"Retaliate\" as retribution.\n4. You target the opponent's \"Knight Touch\", burning 3 mana from your opponent successfully.\n5. You then activate the \"Knight Touch\" for yourself, buffing your rook on a1.\n6. On your next turn you move the rook on a1 to c2.", Limit: DefaultCardLimit},
		{ID: "counterattack", Name: "Counterattack", Type: CardTypeCounter, Description: "If a piece you control would be captured by a piece buffed by a \"Power\" card this turn, capture the attacking piece instead.", Cost: 1, Ignition: 0, Cooldown: 6, Example: "1. Opponent activates \"Rook Touch\" on his knight on e4.\n2. Your opponent attempts to capture your queen on e6.\n3. You activate \"Counterattack\".\n4. The attacking knight is captured instead.", Limit: DefaultCardLimit},
		{ID: "blockade", Name: "Blockade", Type: CardTypeCounter, Description: "If a piece you control would be captured by the activation of a \"Counter\" card this turn while attacking, negate the effect of the \"Counter\" card, then return the attacking piece to its original position and choose another piece to move this turn.", Cost: 0, Ignition: 0, Cooldown: 3, Example: "1. You move your knight on e4 that was buffed by \"Rook Touch\" to e6 in an attempt to capture the opponent's queen.\n2. Opponent activates \"Counterattack\" on your knight.\n3. You activate \"Blockade\".\n4. The effect of \"Counterattack\" is negated and your knight stays on e4.\n5. You choose another piece to move this turn.", Limit: DefaultCardLimit},
		{ID: "backstab", Name: "Backstab", Type: CardTypePower, Description: "Target a pawn you control that is currently facing an opponent's piece with an empty square behind it, jump over the piece and capture it, and if you do, gain 3 mana.", Cost: 1, Ignition: 1, Cooldown: 7, Example: "1. You activate \"Backstab\".\n2. Ignition succeeds the next turn.\n3. You have a pawn on e4.\n4. Opponent has a knight on e5 with an empty square behind it.\n5. The pawn on e4 jumps over the knight on e5 to e6 and captures it.\n6. You gain 3 mana.", Limit: DefaultCardLimit},
		{ID: "stop-right-there", Name: "Stop Right There!", Type: CardTypeRetribution, Description: "Target an ignited card from your opponent and negate its effect.", Cost: 3, Ignition: 0, Cooldown: 5, Example: "1. Opponent activates \"Knight Touch\"\n2. You activate \"Stop Right There!\" as retribution.\n3. The effect of \"Knight Touch\" is negated.", Limit: DefaultCardLimit},
		{ID: "renewal", Name: "Renewal", Type: CardTypePower, Description: "Consume up to 10 energized mana and gain half the amount as mana.", Cost: 0, Ignition: 1, Cooldown: 2, Example: "1. You have 10 energized mana.\n2. You activate \"Renewal\".\n3. Ignition succeeds the next turn.\n4. You consume 10 energized mana and gain 5 mana.", Limit: DefaultCardLimit},
		{ID: "life-drain", Name: "Life Drain", Type: CardTypeContinuous, Description: "While on the ignition slot, every capture you make drains 1 mana from your opponent. When this card's ignition ends, banish this card.", Cost: 3, Ignition: 5, Cooldown: 0, Example: "1. You activate \"Life Drain\".\n2. You capture any piece this turn.\n3. You gain 1 mana from the capture + 1 mana from your opponent's mana pool.", Limit: DefaultCardLimit},
		{ID: "clairvoyance", Name: "Clairvoyance", Type: CardTypeContinuous, Description: "While on the ignition slot, reveal your opponent's hand. When this card's ignition ends, banish this card.", Cost: 7, Ignition: 3, Cooldown: 0, Example: "1. You activate \"Clairvoyance\".\n2. Your opponent's hand is revealed to you and stays revealed until this card's ignition ends.", Limit: DefaultCardLimit},
		{ID: "thunderstorm", Name: "Thunderstorm", Type: CardTypePower, Description: "Randomly choose 10 squares on the board, for each piece you control that is on one of the chosen squares, gain 2 mana, but they cannot move this turn, for each piece your opponent controls that is on one of the chosen squares, burn 2 mana from your opponent, and they cannot move until the end of the next turn.", Cost: 10, Ignition: 1, Cooldown: 10, Example: "1. You activate \"Thunderstorm\".\n2. Ignition succeeds the next turn.\n3. You randomly choose 10 squares on the board.\n4. 3 pieces you control are hit, you gain 6 mana and they cannot move this turn.\n5. 4 pieces your opponent controls are hit, you burn 8 mana from your opponent and those pieces cannot move until the end of the next turn.", Limit: DefaultCardLimit},
		{ID: "extinguish", Name: "Extinguish", Type: CardTypeDisruption, Description: "Target a card on your opponent's ignition slot and negate its effect.", Cost: 2, Ignition: 0, Cooldown: 2, Example: "1. Opponent has a \"Double Turn\" in their ignition slot.\n2. You activate \"Extinguish\".\n3. The effect of \"Double Turn\" is negated and sent to their cooldown slot.", Limit: DefaultCardLimit},
		{ID: "save-it-for-later", Name: "Save It For Later", Type: CardTypeRetribution, Description: "This card can only be activated if you have a card in your ignition slot. Target that card and move it back to your hand, gaining the mana cost of the card as mana.", Cost: 0, Ignition: 0, Cooldown: 10, Example: "1. You have \"Double Turn\" in your ignition slot.\n2. Your opponent activates \"Extinguish\" on your \"Double Turn\".\n3. You activate \"Save It For Later\" as retribution.\n4. The \"Double Turn\" card is moved back to your hand and you gain 6 mana.", Limit: DefaultCardLimit},
	}
}

// CardDefinitionByID returns a card definition by ID.
func CardDefinitionByID(id CardID) (CardDefinition, bool) {
	for _, c := range InitialCardCatalog() {
		if c.ID == id {
			return c, true
		}
	}
	return CardDefinition{}, false
}

// CardRequiresTargetPieces reports whether a card requires one or more board-piece targets at ignite time.
func CardRequiresTargetPieces(id CardID) bool {
	def, ok := CardDefinitionByID(id)
	return ok && def.Targets > 0
}

// StarterDeck currently uses exactly 20 initial cards from Cards.md.
func StarterDeck() []CardInstance {
	catalog := InitialCardCatalog()
	out := make([]CardInstance, 0, DefaultDeckSize)
	for i, c := range catalog {
		if len(out) == DefaultDeckSize {
			break
		}
		out = append(out, CardInstance{
			InstanceID: instanceID(i + 1),
			CardID:     c.ID,
			ManaCost:   c.Cost,
			Ignition:   c.Ignition,
			Cooldown:   c.Cooldown,
		})
	}
	return out
}

func instanceID(n int) string {
	return "c" + itoa(n)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	buf := [16]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	return string(buf[i:])
}
