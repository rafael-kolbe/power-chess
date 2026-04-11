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

// loadDeckAndSkillForSeat returns deck instances and skill for a seat (starter deck for guests).
func (r *RoomSession) loadDeckAndSkillForSeat(srv *Server, pid gameplay.PlayerID) ([]gameplay.CardInstance, gameplay.PlayerSkillID) {
	if srv == nil || srv.decks == nil {
		return gameplay.StarterDeck(), defaultLobbySkill()
	}
	uid := uint64(0)
	if r.authUIDByPlayer != nil {
		uid = r.authUIDByPlayer[pid]
	}
	if uid == 0 {
		return gameplay.StarterDeck(), defaultLobbySkill()
	}
	inst, sk, err := srv.decks.DeckInstancesAndSkillForLobby(uid)
	if err != nil {
		return gameplay.StarterDeck(), defaultLobbySkill()
	}
	return inst, sk
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
	deckA, skA := r.loadDeckAndSkillForSeat(srv, gameplay.PlayerA)
	deckB, skB := r.loadDeckAndSkillForSeat(srv, gameplay.PlayerB)
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
	r.deckMatchInitialized = true
	return nil
}

func (r *RoomSession) resetMatchEngineFromSavedDecksUnsafe(srv *Server) {
	deckA, skA := r.loadDeckAndSkillForSeat(srv, gameplay.PlayerA)
	deckB, skB := r.loadDeckAndSkillForSeat(srv, gameplay.PlayerB)
	skA = normalizeLobbySkill(skA)
	skB = normalizeLobbySkill(skB)
	newState, err := gameplay.NewMatchState(deckA, deckB)
	if err != nil {
		return
	}
	_ = newState.SelectPlayerSkill(gameplay.PlayerA, skA)
	_ = newState.SelectPlayerSkill(gameplay.PlayerB, skB)
	r.Engine = match.NewEngine(newState, chess.NewGame())
}
