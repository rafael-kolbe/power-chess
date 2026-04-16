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
	DefaultIgnitionSlotCapacity = 1
	errNotEnoughMana            = "not enough mana"
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
	// Ignition is this player's ignition zone (one card at a time; independent of the opponent's).
	Ignition IgnitionSlot `json:"ignition,omitempty"`
}

// MatchState is the main gameplay aggregate for turns, resources and card lifecycle.
type MatchState struct {
	Players     map[PlayerID]*PlayerState
	CurrentTurn PlayerID
	TurnNumber  int
	TurnSeconds int

	// LegacyIgnitionSlot is only used when unmarshaling old persisted JSON (single global slot).
	// NormalizeLegacyIgnition copies it into Players[owner].Ignition and clears this field.
	LegacyIgnitionSlot IgnitionSlot `json:"ignitionSlot,omitempty"`

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

// OppositePlayer returns the other seat in a two-player match.
func OppositePlayer(pid PlayerID) PlayerID {
	if pid == PlayerA {
		return PlayerB
	}
	return PlayerA
}

// NormalizeLegacyIgnition migrates pre-per-player ignition persistence into Players[].Ignition.
func (s *MatchState) NormalizeLegacyIgnition() {
	if !s.LegacyIgnitionSlot.Occupied {
		return
	}
	owner := s.LegacyIgnitionSlot.ActivationOwner
	if owner != PlayerA && owner != PlayerB {
		owner = PlayerA
	}
	if p := s.Players[owner]; p != nil {
		p.Ignition = s.LegacyIgnitionSlot
	}
	s.LegacyIgnitionSlot = IgnitionSlot{}
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
		return errors.New(errNotEnoughMana)
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
	svc := NewZoneService()
	_, nextDeck, nextHand, err := svc.MoveCardBetweenSlices(p.Deck, p.Hand, 0)
	if err != nil {
		return err
	}
	p.Deck = nextDeck
	p.Hand = nextHand
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
	if p.Ignition.Occupied {
		return errors.New("ignition slot occupied")
	}
	if p.Mana < card.ManaCost {
		return errors.New(errNotEnoughMana)
	}
	p.Mana -= card.ManaCost
	s.addEnergizedMana(pid, card.ManaCost)

	svc := NewZoneService()
	_, nextHand, _, err := svc.MoveCardBetweenSlices(p.Hand, nil, handIndex)
	if err != nil {
		return err
	}
	p.Hand = nextHand
	p.Ignition = IgnitionSlot{
		Card:            card,
		TurnsRemaining:  card.Ignition,
		Occupied:        true,
		ActivationOwner: pid,
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
		return CardInstance{}, errors.New(errNotEnoughMana)
	}
	p.Mana -= card.ManaCost
	s.addEnergizedMana(pid, card.ManaCost)
	svc := NewZoneService()
	_, nextHand, _, err := svc.MoveCardBetweenSlices(p.Hand, nil, handIndex)
	if err != nil {
		return CardInstance{}, err
	}
	p.Hand = nextHand
	return card, nil
}

// SendCardToCooldown pushes a card into cooldown tracking, or routes it immediately when
// Cooldown is zero: Continuous cards are banished; other types return to the deck (see PROJECT.md).
func (s *MatchState) SendCardToCooldown(pid PlayerID, card CardInstance) {
	p := s.Players[pid]
	NewZoneService().SendCardToCooldown(p, card)
}

// ResolveIgnitionFor finalizes ignition for one player's slot and appends a success/failure event.
func (s *MatchState) ResolveIgnitionFor(owner PlayerID, success bool) error {
	p := s.Players[owner]
	if p == nil || !p.Ignition.Occupied {
		return errors.New("ignition slot is empty")
	}
	card := p.Ignition.Card
	// Effect application is owned by power resolvers later.
	_ = success
	s.ResolvedQueue = append(s.ResolvedQueue, ResolvedIgnitionEvent{
		Owner:   owner,
		Card:    card,
		Success: success,
	})
	p.Ignition = IgnitionSlot{}
	return nil
}

// PopResolvedIgnitions returns and clears all resolved ignition events.
func (s *MatchState) PopResolvedIgnitions() []ResolvedIgnitionEvent {
	ev := append([]ResolvedIgnitionEvent(nil), s.ResolvedQueue...)
	s.ResolvedQueue = nil
	return ev
}

// tickIgnition decrements this player's ignition counter at the start of their turn.
func (s *MatchState) tickIgnition(pid PlayerID) {
	p := s.Players[pid]
	if p == nil || !p.Ignition.Occupied {
		return
	}
	if p.Ignition.TurnsRemaining > 0 {
		p.Ignition.TurnsRemaining--
	}
	if p.Ignition.TurnsRemaining == 0 {
		_ = s.ResolveIgnitionFor(pid, true)
	}
}

func (s *MatchState) tickCooldowns(pid PlayerID) {
	p := s.Players[pid]
	zones := NewZoneService()
	next := p.Cooldowns[:0]
	for _, cd := range p.Cooldowns {
		if cd.TurnsRemaining > 0 {
			cd.TurnsRemaining--
		}
		if cd.TurnsRemaining == 0 {
			zones.RouteFinishedCooldown(p, cd.Card)
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
