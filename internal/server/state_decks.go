package server

import (
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
		return gameplay.StarterDeck(), defaultLobbySkill(), ""
	}
	uid := uint64(0)
	if r.authUIDByPlayer != nil {
		uid = r.authUIDByPlayer[pid]
	}
	if uid == 0 {
		return gameplay.StarterDeck(), defaultLobbySkill(), ""
	}
	inst, sk, sleeve, err := srv.decks.DeckInstancesSkillAndSleeveForLobby(uid)
	if err != nil {
		return gameplay.StarterDeck(), defaultLobbySkill(), ""
	}
	return inst, sk, sleeve
}

// MaybeRebuildEngineWithSavedDecks replaces the engine once both players are connected and decks were not applied yet.
func (r *RoomSession) MaybeRebuildEngineWithSavedDecks(srv *Server) error {
	if srv == nil || srv.decks == nil {
		return nil
	}
	r.stateM.Lock()
	defer r.stateM.Unlock()
	if r.matchEnded || r.deckMatchInitialized {
		return nil
	}
	if r.connectedByPlayer[gameplay.PlayerA] == 0 || r.connectedByPlayer[gameplay.PlayerB] == 0 {
		return nil
	}
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
	return nil
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
}
