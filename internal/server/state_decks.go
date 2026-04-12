package server

import (
	"time"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

func defaultLobbySkill() gameplay.PlayerSkillID {
	return gameplay.PlayerSkillID("reinforcements")
}

func normalizeLobbySkill(s gameplay.PlayerSkillID) gameplay.PlayerSkillID {
	if gameplay.ValidPlayerSkillID(s) {
		return s
	}
	return defaultLobbySkill()
}

// loadDeckSkillAndSleeveForSeat returns deck instances, skill, and sleeve color for a seat (starter deck for guests).
func (r *RoomSession) loadDeckSkillAndSleeveForSeat(srv *Server, pid gameplay.PlayerID) ([]gameplay.CardInstance, gameplay.PlayerSkillID, string) {
	if srv == nil || srv.decks == nil {
		return gameplay.StarterDeck(), defaultLobbySkill(), DefaultSleeveColor("")
	}
	uid := uint64(0)
	if r.authUIDByPlayer != nil {
		uid = r.authUIDByPlayer[pid]
	}
	if uid == 0 {
		return gameplay.StarterDeck(), defaultLobbySkill(), DefaultSleeveColor("")
	}
	inst, sk, sleeve, err := srv.decks.DeckInstancesSkillAndSleeveForLobby(uid)
	if err != nil {
		return gameplay.StarterDeck(), defaultLobbySkill(), DefaultSleeveColor("")
	}
	return inst, sk, DefaultSleeveColor(sleeve)
}

// MaybeRebuildEngineWithSavedDecks replaces the engine once both players are connected and decks were not applied yet.
func (r *RoomSession) MaybeRebuildEngineWithSavedDecks(srv *Server) error {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if r.matchEnded {
		return nil
	}
	if r.connectedByPlayer[gameplay.PlayerA] == 0 || r.connectedByPlayer[gameplay.PlayerB] == 0 {
		return nil
	}
	if srv != nil && srv.decks != nil && !r.deckMatchInitialized {
		deckA, skA, sleeveA := r.loadDeckSkillAndSleeveForSeat(srv, gameplay.PlayerA)
		deckB, skB, sleeveB := r.loadDeckSkillAndSleeveForSeat(srv, gameplay.PlayerB)
		skA = normalizeLobbySkill(skA)
		skB = normalizeLobbySkill(skB)
		newState, err := gameplay.NewMatchState(deckA, deckB)
		if err != nil {
			return err
		}
		if err := newState.SelectPlayerSkill(gameplay.PlayerA, skA); err != nil {
			return err
		}
		if err := newState.SelectPlayerSkill(gameplay.PlayerB, skB); err != nil {
			return err
		}
		r.Engine = match.NewEngine(newState, chess.NewGame())
		r.sleeveByPlayer[gameplay.PlayerA] = sleeveA
		r.sleeveByPlayer[gameplay.PlayerB] = sleeveB
		r.deckMatchInitialized = true
	}
	return r.beginOpeningPhaseIfNeededUnsafe()
}

func (r *RoomSession) resetMatchEngineFromSavedDecksUnsafe(srv *Server) {
	deckA, skA, sleeveA := r.loadDeckSkillAndSleeveForSeat(srv, gameplay.PlayerA)
	deckB, skB, sleeveB := r.loadDeckSkillAndSleeveForSeat(srv, gameplay.PlayerB)
	skA = normalizeLobbySkill(skA)
	skB = normalizeLobbySkill(skB)
	newState, err := gameplay.NewMatchState(deckA, deckB)
	if err != nil {
		return
	}
	_ = newState.SelectPlayerSkill(gameplay.PlayerA, skA)
	_ = newState.SelectPlayerSkill(gameplay.PlayerB, skB)
	r.Engine = match.NewEngine(newState, chess.NewGame())
	r.sleeveByPlayer[gameplay.PlayerA] = sleeveA
	r.sleeveByPlayer[gameplay.PlayerB] = sleeveB
	if r.connectedByPlayer[gameplay.PlayerA] > 0 && r.connectedByPlayer[gameplay.PlayerB] > 0 {
		_ = gameplay.BeginOpeningPhase(r.Engine.State)
		r.startMulliganDeadlineUnsafe(time.Now().UTC())
	}
}

// beginOpeningPhaseIfNeededUnsafe shuffles and deals opening hands when both players are seated and the match has not begun.
// Caller must hold r.stateM.
func (r *RoomSession) beginOpeningPhaseIfNeededUnsafe() error {
	if r.matchEnded {
		return nil
	}
	if r.connectedByPlayer[gameplay.PlayerA] == 0 || r.connectedByPlayer[gameplay.PlayerB] == 0 {
		return nil
	}
	s := r.Engine.State
	if s.MulliganPhaseActive {
		return nil
	}
	if s.Started {
		return nil
	}
	if len(s.Players[gameplay.PlayerA].Hand) != 0 || len(s.Players[gameplay.PlayerB].Hand) != 0 {
		return nil
	}
	if err := gameplay.BeginOpeningPhase(s); err != nil {
		return err
	}
	r.startMulliganDeadlineUnsafe(time.Now().UTC())
	return nil
}
