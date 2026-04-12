package gameplay

import (
	"errors"
	"fmt"
)

const (
	DefaultTurnSeconds          = 30
	DefaultMaxMana              = 10
	DefaultMaxEnergizedMana     = 20
	DefaultMaxHandSize          = 5
	DefaultDeckSize             = 20
	DefaultInitialDraw          = 3
	DefaultDrawCardManaCost     = 2
	DefaultExtraManaPerTurnCap  = 1
	DefaultStrikeLimit          = 3
	DefaultIgnitionSlotCapacity = 1
)

type PlayerID string

const (
	PlayerA PlayerID = "A"
	PlayerB PlayerID = "B"
)

type CardID string

// CardInstance is a concrete card copy that can move between deck/hand/ignition/cooldown.
type CardInstance struct {
	InstanceID string
	CardID     CardID
	ManaCost   int
	Ignition   int
	Cooldown   int
}

// PieceRef is a lightweight piece descriptor used by graveyard logic.
type PieceRef struct {
	Color string
	Type  string
}

// IgnitionSlot stores the currently igniting card (single slot).
type IgnitionSlot struct {
	Card            CardInstance
	TurnsRemaining  int
	Occupied        bool
	ActivationOwner PlayerID
}

// CooldownEntry tracks a card and remaining cooldown turns.
type CooldownEntry struct {
	Card           CardInstance
	TurnsRemaining int
}

// PlayerState holds all mutable match data for one player.
type PlayerState struct {
	ID            PlayerID
	SelectedSkill PlayerSkillID

	Mana                int
	MaxMana             int
	EnergizedMana       int
	MaxEnergizedMana    int
	ExtraManaTurnCap    int
	ExtraManaGainedTurn int

	Deck      []CardInstance
	Hand      []CardInstance
	Cooldowns []CooldownEntry
	Banished  []CardInstance

	Graveyard []PieceRef
	Strikes   int
}

// MatchState is the main gameplay aggregate for turns, resources and card lifecycle.
type MatchState struct {
	Players map[PlayerID]*PlayerState

	CurrentTurn PlayerID
	TurnNumber  int
	TurnSeconds int

	IgnitionSlot  IgnitionSlot
	ResolvedQueue []ResolvedIgnitionEvent
	Started       bool

	// MulliganPhaseActive is true while players may return cards and redraw (opening only).
	MulliganPhaseActive bool `json:"mulliganPhaseActive,omitempty"`
	// MulliganConfirmed records whether each player locked in their mulligan choice.
	MulliganConfirmed map[PlayerID]bool `json:"mulliganConfirmed,omitempty"`
	// MulliganReturnedCount stores how many cards each player returned (-1 until they confirm).
	MulliganReturnedCount map[PlayerID]int `json:"mulliganReturnedCount,omitempty"`
}

// ResolvedIgnitionEvent reports the latest ignition completion result.
type ResolvedIgnitionEvent struct {
	Owner   PlayerID
	Card    CardInstance
	Success bool
}

// NewMatchState creates an initialized match with default rules and full decks (no opening draw).
// Call BeginOpeningPhase once both players are present to shuffle, draw opening hands, and start mulligan.
func NewMatchState(deckA, deckB []CardInstance) (*MatchState, error) {
	if len(deckA) != DefaultDeckSize || len(deckB) != DefaultDeckSize {
		return nil, fmt.Errorf("deck size must be %d", DefaultDeckSize)
	}
	s := &MatchState{
		Players: map[PlayerID]*PlayerState{
			PlayerA: {
				ID:               PlayerA,
				MaxMana:          DefaultMaxMana,
				MaxEnergizedMana: DefaultMaxEnergizedMana,
				ExtraManaTurnCap: DefaultExtraManaPerTurnCap,
				Deck:             append([]CardInstance(nil), deckA...),
			},
			PlayerB: {
				ID:               PlayerB,
				MaxMana:          DefaultMaxMana,
				MaxEnergizedMana: DefaultMaxEnergizedMana,
				ExtraManaTurnCap: DefaultExtraManaPerTurnCap,
				Deck:             append([]CardInstance(nil), deckB...),
			},
		},
		CurrentTurn: PlayerA,
		TurnNumber:  1,
		TurnSeconds: DefaultTurnSeconds,
	}
	return s, nil
}

// StartTurn applies start-of-turn updates for the active player (+1 mana, respecting max mana pool).
func (s *MatchState) StartTurn(pid PlayerID) error {
	if s.CurrentTurn != pid {
		return errors.New("cannot start another player's turn")
	}
	p := s.Players[pid]
	s.Started = true
	p.ExtraManaGainedTurn = 0
	s.addMana(pid, 1)
	s.tickCooldowns(pid)
	s.tickIgnition(pid)
	return nil
}

// SelectPlayerSkill sets a player's permanent skill before the match starts.
func (s *MatchState) SelectPlayerSkill(pid PlayerID, skillID PlayerSkillID) error {
	if s.MulliganPhaseActive {
		return errors.New("cannot select player skill during mulligan")
	}
	if s.Started {
		return errors.New("cannot select player skill after match start")
	}
	if !hasSkill(skillID) {
		return errors.New("invalid player skill")
	}
	s.Players[pid].SelectedSkill = skillID
	return nil
}

// EndTurn switches active player and increments turn counter when needed.
func (s *MatchState) EndTurn(pid PlayerID) error {
	if s.CurrentTurn != pid {
		return errors.New("cannot end another player's turn")
	}
	if pid == PlayerA {
		s.CurrentTurn = PlayerB
	} else {
		s.CurrentTurn = PlayerA
		s.TurnNumber++
	}
	return nil
}

// HandleTurnTimeout applies a strike to the active player and returns whether they lost.
func (s *MatchState) HandleTurnTimeout(pid PlayerID) (lost bool, err error) {
	if s.CurrentTurn != pid {
		return false, errors.New("timeout for non-current player")
	}
	p := s.Players[pid]
	p.Strikes++
	if p.Strikes >= DefaultStrikeLimit {
		return true, nil
	}
	if err := s.EndTurn(pid); err != nil {
		return false, err
	}
	return false, nil
}

// GrantCaptureBonusMana grants capped extra mana for capture-triggered bonuses.
func (s *MatchState) GrantCaptureBonusMana(pid PlayerID) {
	p := s.Players[pid]
	if p.ExtraManaGainedTurn >= p.ExtraManaTurnCap {
		return
	}
	p.ExtraManaGainedTurn++
	s.addMana(pid, 1)
}

// GrantManaForChessCapture adds one mana for each chess capture (including en passant), respecting max mana.
func (s *MatchState) GrantManaForChessCapture(pid PlayerID) {
	s.addMana(pid, 1)
}

// DrawCard pays draw cost and moves one card from deck to hand.
func (s *MatchState) DrawCard(pid PlayerID) error {
	p := s.Players[pid]
	if p.Mana < DefaultDrawCardManaCost {
		return errors.New("not enough mana")
	}
	if len(p.Hand) >= DefaultMaxHandSize {
		return errors.New("hand is full")
	}
	p.Mana -= DefaultDrawCardManaCost
	return s.drawCardNoCost(pid)
}

func (s *MatchState) drawCardNoCost(pid PlayerID) error {
	p := s.Players[pid]
	if len(p.Deck) == 0 {
		return errors.New("deck is empty")
	}
	if len(p.Hand) >= DefaultMaxHandSize {
		return errors.New("hand is full")
	}
	card := p.Deck[0]
	p.Deck = p.Deck[1:]
	p.Hand = append(p.Hand, card)
	return nil
}

// ActivateCard consumes mana, energizes mana pool and places a card in ignition slot.
func (s *MatchState) ActivateCard(pid PlayerID, handIndex int) error {
	if s.CurrentTurn != pid {
		return errors.New("not current player's turn")
	}
	p := s.Players[pid]
	if handIndex < 0 || handIndex >= len(p.Hand) {
		return errors.New("invalid hand index")
	}
	card := p.Hand[handIndex]
	if s.IgnitionSlot.Occupied && card.CardID != "save-it-for-later" {
		return errors.New("ignition slot occupied")
	}
	if card.CardID == "save-it-for-later" {
		if err := s.handleSaveItForLater(pid); err != nil {
			return err
		}
	}
	if p.Mana < card.ManaCost {
		return errors.New("not enough mana")
	}
	p.Mana -= card.ManaCost
	s.addEnergizedMana(pid, card.ManaCost)

	p.Hand = append(p.Hand[:handIndex], p.Hand[handIndex+1:]...)
	s.IgnitionSlot = IgnitionSlot{
		Card:            card,
		TurnsRemaining:  card.Ignition,
		Occupied:        true,
		ActivationOwner: pid,
	}
	if card.Ignition == 0 {
		return s.ResolveIgnition(true)
	}
	return nil
}

// ConsumeCardFromHand removes a card from hand after paying its mana cost.
func (s *MatchState) ConsumeCardFromHand(pid PlayerID, handIndex int) (CardInstance, error) {
	p := s.Players[pid]
	if handIndex < 0 || handIndex >= len(p.Hand) {
		return CardInstance{}, errors.New("invalid hand index")
	}
	card := p.Hand[handIndex]
	if p.Mana < card.ManaCost {
		return CardInstance{}, errors.New("not enough mana")
	}
	p.Mana -= card.ManaCost
	s.addEnergizedMana(pid, card.ManaCost)
	p.Hand = append(p.Hand[:handIndex], p.Hand[handIndex+1:]...)
	return card, nil
}

// SendCardToCooldown pushes a card into cooldown tracking.
func (s *MatchState) SendCardToCooldown(pid PlayerID, card CardInstance) {
	p := s.Players[pid]
	p.Cooldowns = append(p.Cooldowns, CooldownEntry{
		Card:           card,
		TurnsRemaining: card.Cooldown,
	})
}

// ResolveIgnition finalizes ignition and moves card to cooldown.
func (s *MatchState) ResolveIgnition(success bool) error {
	if !s.IgnitionSlot.Occupied {
		return errors.New("ignition slot is empty")
	}
	owner := s.IgnitionSlot.ActivationOwner
	p := s.Players[owner]

	// Effect application is owned by power resolvers later.
	_ = success
	p.Cooldowns = append(p.Cooldowns, CooldownEntry{
		Card:           s.IgnitionSlot.Card,
		TurnsRemaining: s.IgnitionSlot.Card.Cooldown,
	})
	s.ResolvedQueue = append(s.ResolvedQueue, ResolvedIgnitionEvent{
		Owner:   owner,
		Card:    s.IgnitionSlot.Card,
		Success: success,
	})
	s.IgnitionSlot = IgnitionSlot{}
	return nil
}

// PopResolvedIgnitions returns and clears all resolved ignition events.
func (s *MatchState) PopResolvedIgnitions() []ResolvedIgnitionEvent {
	ev := append([]ResolvedIgnitionEvent(nil), s.ResolvedQueue...)
	s.ResolvedQueue = nil
	return ev
}

// tickIgnition decrements the ignition counter only when the player starting the turn
// is the one who activated the card (ActivationOwner). Opponent turn starts do not tick it.
func (s *MatchState) tickIgnition(pid PlayerID) {
	if !s.IgnitionSlot.Occupied {
		return
	}
	if s.IgnitionSlot.ActivationOwner != pid {
		return
	}
	if s.IgnitionSlot.TurnsRemaining > 0 {
		s.IgnitionSlot.TurnsRemaining--
	}
	if s.IgnitionSlot.TurnsRemaining == 0 {
		_ = s.ResolveIgnition(true)
	}
}

func (s *MatchState) tickCooldowns(pid PlayerID) {
	p := s.Players[pid]
	next := p.Cooldowns[:0]
	for _, cd := range p.Cooldowns {
		if cd.TurnsRemaining > 0 {
			cd.TurnsRemaining--
		}
		if cd.TurnsRemaining == 0 {
			p.Deck = append(p.Deck, cd.Card)
			continue
		}
		next = append(next, cd)
	}
	p.Cooldowns = next
}

// AddToGraveyard records a captured piece in the owner's graveyard.
func (s *MatchState) AddToGraveyard(owner PlayerID, piece PieceRef) {
	s.Players[owner].Graveyard = append(s.Players[owner].Graveyard, piece)
}

// ResurrectFromGraveyard removes and returns a piece from any selected graveyard.
func (s *MatchState) ResurrectFromGraveyard(caster PlayerID, fromOwner PlayerID, index int) (PieceRef, error) {
	_ = caster // Power permission checks live in effect resolvers.
	g := s.Players[fromOwner].Graveyard
	if index < 0 || index >= len(g) {
		return PieceRef{}, errors.New("invalid graveyard index")
	}
	piece := g[index]
	s.Players[fromOwner].Graveyard = append(g[:index], g[index+1:]...)
	return piece, nil
}

// ActivateSpecialAbility spends full energized pool and increases next activation threshold by 10.
func (s *MatchState) ActivateSpecialAbility(pid PlayerID) error {
	if s.CurrentTurn != pid {
		return errors.New("player skill can only be activated on your turn")
	}
	p := s.Players[pid]
	if p.SelectedSkill == "" {
		return errors.New("player skill not selected")
	}
	if p.EnergizedMana < p.MaxEnergizedMana {
		return errors.New("not enough energized mana")
	}
	p.EnergizedMana = 0
	p.MaxEnergizedMana += 10
	return s.EndTurn(pid)
}

func (s *MatchState) handleSaveItForLater(pid PlayerID) error {
	if !s.IgnitionSlot.Occupied {
		return errors.New("save-it-for-later requires occupied ignition slot")
	}
	targetOwner := s.IgnitionSlot.ActivationOwner
	targetCard := s.IgnitionSlot.Card
	s.IgnitionSlot = IgnitionSlot{}
	s.Players[targetOwner].Hand = append(s.Players[targetOwner].Hand, targetCard)
	s.addMana(pid, targetCard.ManaCost)
	return nil
}

func hasSkill(id PlayerSkillID) bool {
	return ValidPlayerSkillID(id)
}

func (s *MatchState) addMana(pid PlayerID, amount int) {
	p := s.Players[pid]
	p.Mana += amount
	if p.Mana > p.MaxMana {
		p.Mana = p.MaxMana
	}
}

func (s *MatchState) addEnergizedMana(pid PlayerID, amount int) {
	p := s.Players[pid]
	p.EnergizedMana += amount
	if p.EnergizedMana > p.MaxEnergizedMana {
		p.EnergizedMana = p.MaxEnergizedMana
	}
}
