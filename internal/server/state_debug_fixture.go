package server

import (
	"fmt"
	"strings"
	"time"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

// parseDebugCardIDList trims and converts JSON string lists to CardID values.
func parseDebugCardIDList(src []string) ([]gameplay.CardID, error) {
	out := make([]gameplay.CardID, 0, len(src))
	for _, s := range src {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, fmt.Errorf("empty card id in list")
		}
		out = append(out, gameplay.CardID(s))
	}
	return out, nil
}

const (
	debugManaMin      = 0
	debugManaMaxCap   = 99
	debugEnergyMaxCap = 999
)

// applyDebugSideResources applies optional mana / energized overrides after deck and hand are set.
func applyDebugSideResources(p *gameplay.PlayerState, f *DebugSideFixture) error {
	if f.MaxMana != nil {
		if *f.MaxMana < 1 || *f.MaxMana > debugManaMaxCap {
			return fmt.Errorf("maxMana out of range (1-%d)", debugManaMaxCap)
		}
		p.MaxMana = *f.MaxMana
	}
	if f.Mana != nil {
		m := *f.Mana
		if m < debugManaMin || m > debugManaMaxCap {
			return fmt.Errorf("mana out of range (%d-%d)", debugManaMin, debugManaMaxCap)
		}
		if m > p.MaxMana {
			m = p.MaxMana
		}
		p.Mana = m
	}
	if f.MaxEnergized != nil {
		if *f.MaxEnergized < 1 || *f.MaxEnergized > debugEnergyMaxCap {
			return fmt.Errorf("maxEnergized out of range (1-%d)", debugEnergyMaxCap)
		}
		p.MaxEnergizedMana = *f.MaxEnergized
	}
	if f.EnergizedMana != nil {
		e := *f.EnergizedMana
		if e < 0 || e > debugEnergyMaxCap {
			return fmt.Errorf("energizedMana out of range (0-%d)", debugEnergyMaxCap)
		}
		if e > p.MaxEnergizedMana {
			e = p.MaxEnergizedMana
		}
		p.EnergizedMana = e
	}
	return nil
}

// ApplyDebugMatchFixture replaces the match engine with preset decks, hands, and optional resources for white (A) and black (B),
// then enters the mulligan phase without shuffling. Only valid before the first turn has started.
func (r *RoomSession) ApplyDebugMatchFixture(white, black *DebugSideFixture, srv *Server) error {
	_ = srv
	if r.matchEnded {
		return fmt.Errorf("match ended")
	}
	if r.connectedByPlayer[gameplay.PlayerA] == 0 || r.connectedByPlayer[gameplay.PlayerB] == 0 {
		return fmt.Errorf("waiting_for_opponent")
	}
	if r.Engine.State.Started {
		return fmt.Errorf("match already started")
	}
	if white == nil || black == nil {
		return fmt.Errorf("white and black fixtures are required")
	}
	deckW, err := parseDebugCardIDList(white.Deck)
	if err != nil {
		return err
	}
	handW, err := parseDebugCardIDList(white.Hand)
	if err != nil {
		return err
	}
	deckB, err := parseDebugCardIDList(black.Deck)
	if err != nil {
		return err
	}
	handB, err := parseDebugCardIDList(black.Hand)
	if err != nil {
		return err
	}
	old := r.Engine.State
	newState, err := gameplay.NewMatchStateWithPresetHands(deckW, deckB, handW, handB)
	if err != nil {
		return err
	}
	skA := normalizeLobbySkill(old.Players[gameplay.PlayerA].SelectedSkill)
	skB := normalizeLobbySkill(old.Players[gameplay.PlayerB].SelectedSkill)
	if err := newState.SelectPlayerSkill(gameplay.PlayerA, skA); err != nil {
		return err
	}
	if err := newState.SelectPlayerSkill(gameplay.PlayerB, skB); err != nil {
		return err
	}
	if err := applyDebugSideResources(newState.Players[gameplay.PlayerA], white); err != nil {
		return err
	}
	if err := applyDebugSideResources(newState.Players[gameplay.PlayerB], black); err != nil {
		return err
	}
	gameplay.EnterMulliganPhaseWithoutShuffle(newState)
	r.Engine = match.NewEngine(newState, chess.NewGame())
	r.deckMatchInitialized = true
	r.startMulliganDeadlineUnsafe(time.Now().UTC())
	return nil
}
