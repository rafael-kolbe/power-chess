package gameplay

import "testing"

func TestBeginOpeningPhaseDrawsThree(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := BeginOpeningPhase(s); err != nil {
		t.Fatalf("BeginOpeningPhase: %v", err)
	}
	if len(s.Players[PlayerA].Hand) != 3 || len(s.Players[PlayerB].Hand) != 3 {
		t.Fatalf("expected 3 initial cards in hand")
	}
}

func TestDrawCardManaAndHandLimit(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	if err := BeginOpeningPhase(s); err != nil {
		t.Fatalf("BeginOpeningPhase: %v", err)
	}
	if _, err := s.ConfirmMulligan(PlayerA, nil); err != nil {
		t.Fatalf("ConfirmMulligan A: %v", err)
	}
	if _, err := s.ConfirmMulligan(PlayerB, nil); err != nil {
		t.Fatalf("ConfirmMulligan B: %v", err)
	}
	p := s.Players[PlayerA]
	p.Mana = 10
	for i := 0; i < 2; i++ {
		if err := s.DrawCard(PlayerA); err != nil {
			t.Fatalf("draw should succeed: %v", err)
		}
	}
	if len(p.Hand) != 5 {
		t.Fatalf("expected hand size 5, got %d", len(p.Hand))
	}
	if err := s.DrawCard(PlayerA); err == nil {
		t.Fatalf("expected hand limit error")
	}
}

func TestActivateCardConsumesManaAndAddsEnergized(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	p := s.Players[PlayerA]
	p.Hand = []CardInstance{{InstanceID: "x1", CardID: "double-turn", ManaCost: 6, Ignition: 2, Cooldown: 9}}
	p.Mana = 10
	card := p.Hand[0]
	if err := s.ActivateCard(PlayerA, 0); err != nil {
		t.Fatalf("activate failed: %v", err)
	}
	if !p.Ignition.Occupied {
		t.Fatalf("ignition slot should be occupied")
	}
	if p.EnergizedMana != card.ManaCost {
		t.Fatalf("expected energized mana %d, got %d", card.ManaCost, p.EnergizedMana)
	}
}

func TestSpecialAbilityConsumesAndScalesPool(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	p := s.Players[PlayerA]
	if err := s.SelectPlayerSkill(PlayerA, "reinforcements"); err != nil {
		t.Fatalf("select player skill failed: %v", err)
	}
	p.EnergizedMana = p.MaxEnergizedMana
	before := p.MaxEnergizedMana
	if err := s.ActivateSpecialAbility(PlayerA); err != nil {
		t.Fatalf("activate special failed: %v", err)
	}
	if p.EnergizedMana != 0 {
		t.Fatalf("energized mana must be reset")
	}
	if p.MaxEnergizedMana != before+10 {
		t.Fatalf("expected max energized mana increase by 10")
	}
	if s.CurrentTurn != PlayerB {
		t.Fatalf("special ability should consume turn")
	}
}

func TestPlayerSkillMustBeSelectedBeforeMatchStart(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	if err := s.SelectPlayerSkill(PlayerA, "reinforcements"); err != nil {
		t.Fatalf("expected valid selection: %v", err)
	}
	if err := s.StartTurn(PlayerA); err != nil {
		t.Fatalf("start turn failed: %v", err)
	}
	if err := s.SelectPlayerSkill(PlayerB, "dimension-shift"); err == nil {
		t.Fatalf("expected selection to fail after match start")
	}
}

func TestTickIgnitionOnlyOnActivatorTurn(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}
	card := CardInstance{InstanceID: "x1", CardID: "double-turn", ManaCost: 1, Ignition: 2, Cooldown: 1}
	s.Players[PlayerA].Hand = []CardInstance{card}
	s.Players[PlayerA].Mana = 10
	if err := s.ActivateCard(PlayerA, 0); err != nil {
		t.Fatalf("ActivateCard: %v", err)
	}
	if s.Players[PlayerA].Ignition.TurnsRemaining != 2 {
		t.Fatalf("expected ignition turns 2, got %d", s.Players[PlayerA].Ignition.TurnsRemaining)
	}
	// Opponent's turn start must not tick ignition.
	s.CurrentTurn = PlayerB
	if err := s.StartTurn(PlayerB); err != nil {
		t.Fatalf("StartTurn B: %v", err)
	}
	if s.Players[PlayerA].Ignition.TurnsRemaining != 2 {
		t.Fatalf("ignition must not tick on opponent turn, got %d", s.Players[PlayerA].Ignition.TurnsRemaining)
	}
	// Activator's turn ticks ignition.
	s.CurrentTurn = PlayerA
	if err := s.StartTurn(PlayerA); err != nil {
		t.Fatalf("StartTurn A: %v", err)
	}
	if s.Players[PlayerA].Ignition.TurnsRemaining != 1 {
		t.Fatalf("ignition should tick on activator turn, got %d", s.Players[PlayerA].Ignition.TurnsRemaining)
	}
}

func TestResurrectFromOpponentGraveyard(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	s.AddToGraveyard(PlayerB, PieceRef{Color: "black", Type: "rook"})
	piece, err := s.ResurrectFromGraveyard(PlayerA, PlayerB, 0)
	if err != nil {
		t.Fatalf("resurrect should work: %v", err)
	}
	if piece.Type != "rook" {
		t.Fatalf("unexpected resurrected piece: %s", piece.Type)
	}
}
