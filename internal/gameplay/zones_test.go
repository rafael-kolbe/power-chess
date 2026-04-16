package gameplay

import "testing"

func TestZoneServiceMoveCardBetweenSlices(t *testing.T) {
	svc := NewZoneService()
	from := []CardInstance{
		{InstanceID: "c1", CardID: "energy-gain"},
		{InstanceID: "c2", CardID: "retaliate"},
	}
	to := []CardInstance{}

	moved, nextFrom, nextTo, err := svc.MoveCardBetweenSlices(from, to, 1)
	if err != nil {
		t.Fatalf("MoveCardBetweenSlices returned error: %v", err)
	}
	if moved.InstanceID != "c2" {
		t.Fatalf("expected moved card c2, got %s", moved.InstanceID)
	}
	if len(nextFrom) != 1 || nextFrom[0].InstanceID != "c1" {
		t.Fatalf("unexpected source result: %#v", nextFrom)
	}
	if len(nextTo) != 1 || nextTo[0].InstanceID != "c2" {
		t.Fatalf("unexpected destination result: %#v", nextTo)
	}
}

func TestZoneServiceMoveCardBetweenSlicesRejectsInvalidIndex(t *testing.T) {
	svc := NewZoneService()
	_, _, _, err := svc.MoveCardBetweenSlices([]CardInstance{{InstanceID: "c1"}}, nil, 4)
	if err == nil {
		t.Fatalf("expected invalid source index error")
	}
}

func TestZoneServiceSendCardToCooldownRoutesByTurns(t *testing.T) {
	svc := NewZoneService()
	p := &PlayerState{}
	cooldownCard := CardInstance{InstanceID: "c1", CardID: "counterattack", Cooldown: 2}
	continuousCard := CardInstance{InstanceID: "c2", CardID: "life-drain", Cooldown: 0}
	powerCard := CardInstance{InstanceID: "c3", CardID: "energy-gain", Cooldown: 0}

	svc.SendCardToCooldown(p, cooldownCard)
	svc.SendCardToCooldown(p, continuousCard)
	svc.SendCardToCooldown(p, powerCard)

	if len(p.Cooldowns) != 1 {
		t.Fatalf("expected 1 cooldown entry, got %d", len(p.Cooldowns))
	}
	if len(p.Banished) != 1 || p.Banished[0].InstanceID != "c2" {
		t.Fatalf("continuous card must be banished")
	}
	if len(p.Deck) != 1 || p.Deck[0].InstanceID != "c3" {
		t.Fatalf("non-continuous card must return to deck")
	}
}
