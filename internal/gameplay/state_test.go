package gameplay

import "testing"

func TestNewMatchStateInitialDraw(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Players[PlayerA].Hand) != 3 || len(s.Players[PlayerB].Hand) != 3 {
		t.Fatalf("expected 3 initial cards in hand")
	}
}

func TestDrawCardManaAndHandLimit(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
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
	p.Hand = []CardInstance{{InstanceID: "x1", CardID: "double-turn", ManaCost: 4, Ignition: 1, Cooldown: 5}}
	p.Mana = 10
	card := p.Hand[0]
	if err := s.ActivateCard(PlayerA, 0); err != nil {
		t.Fatalf("activate failed: %v", err)
	}
	if !s.IgnitionSlot.Occupied {
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

func TestTimeoutStrikesAndLoss(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	for i := 0; i < 2; i++ {
		lost, err := s.HandleTurnTimeout(PlayerA)
		if err != nil || lost {
			t.Fatalf("unexpected timeout result at strike %d", i+1)
		}
		_ = s.StartTurn(PlayerB)
		_ = s.EndTurn(PlayerB)
	}
	lost, err := s.HandleTurnTimeout(PlayerA)
	if err != nil {
		t.Fatalf("timeout error: %v", err)
	}
	if !lost {
		t.Fatalf("player should lose on third strike")
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
