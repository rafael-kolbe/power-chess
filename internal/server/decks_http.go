package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"power-chess/internal/gameplay"
)

type deckJSON struct {
	ID            uint64   `json:"id"`
	Name          string   `json:"name"`
	CardIDs       []string `json:"cardIds"`
	PlayerSkillID string   `json:"playerSkillId"`
	SleeveColor   string   `json:"sleeveColor"`
}

type deckCreateRequest struct {
	Name          string   `json:"name"`
	CardIDs       []string `json:"cardIds"`
	PlayerSkillID string   `json:"playerSkillId"`
	SleeveColor   string   `json:"sleeveColor"`
}

type deckValidateRequest struct {
	CardIDs []string `json:"cardIds"`
}

type lobbyDeckRequest struct {
	DeckID uint64 `json:"deckId"`
}

// handleMeLobbyDeck sets the user's lobby deck (PUT /api/me/lobby-deck).
func (s *Server) handleMeLobbyDeck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.decks == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "decks_unavailable")
		return
	}
	u, ok := s.authUserFromHTTP(w, r)
	if !ok {
		return
	}
	var body lobbyDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if body.DeckID == 0 {
		writeAuthError(w, http.StatusBadRequest, "deck_id_required")
		return
	}
	if err := s.decks.SetLobbyDeck(u.ID, body.DeckID); err != nil {
		if errors.Is(err, ErrDeckNotFound) {
			writeAuthError(w, http.StatusNotFound, "deck_not_found")
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleDeckValidate POST /api/decks/validate — checks 20-card legality without saving.
func (s *Server) handleDeckValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.decks == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "decks_unavailable")
		return
	}
	if _, ok := s.authUserFromHTTP(w, r); !ok {
		return
	}
	var body deckValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	ids := make([]gameplay.CardID, len(body.CardIDs))
	for i, x := range body.CardIDs {
		ids[i] = gameplay.CardID(x)
	}
	if err := gameplay.ValidateDeckComposition(ids); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"valid": "false", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"valid": "true"})
}

func (s *Server) handleDecksCollection(w http.ResponseWriter, r *http.Request) {
	if s.decks == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "decks_unavailable")
		return
	}
	u, ok := s.authUserFromHTTP(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := s.decks.ListDecks(u.ID)
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "list_failed")
			return
		}
		lobbyID := u.LobbyDeckID
		out := make([]deckJSON, 0, len(rows))
		for _, row := range rows {
			ids, _ := parseCardIDsJSON(row.CardIDsJSON)
			sids := make([]string, len(ids))
			for i, id := range ids {
				sids[i] = string(id)
			}
			out = append(out, deckJSON{
				ID:            row.ID,
				Name:          row.Name,
				CardIDs:       sids,
				PlayerSkillID: row.PlayerSkillID,
				SleeveColor:   row.SleeveColor,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"decks": out, "lobbyDeckId": lobbyID})
	case http.MethodPost:
		var body deckCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAuthError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		ids := make([]gameplay.CardID, len(body.CardIDs))
		for i, x := range body.CardIDs {
			ids[i] = gameplay.CardID(x)
		}
		d, err := s.decks.CreateDeck(u.ID, body.Name, ids, gameplay.PlayerSkillID(body.PlayerSkillID), body.SleeveColor)
		if errors.Is(err, ErrTooManyDecks) {
			writeAuthError(w, http.StatusConflict, "too_many_decks")
			return
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		cardStrs, _ := parseCardIDsJSON(d.CardIDsJSON)
		sids := make([]string, len(cardStrs))
		for i, id := range cardStrs {
			sids[i] = string(id)
		}
		writeJSON(w, http.StatusCreated, deckJSON{
			ID:            d.ID,
			Name:          d.Name,
			CardIDs:       sids,
			PlayerSkillID: d.PlayerSkillID,
			SleeveColor:   d.SleeveColor,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDeckByID(w http.ResponseWriter, r *http.Request, deckID uint64) {
	if s.decks == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "decks_unavailable")
		return
	}
	u, ok := s.authUserFromHTTP(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := s.decks.ListDecks(u.ID)
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "list_failed")
			return
		}
		for _, row := range rows {
			if row.ID != deckID {
				continue
			}
			ids, _ := parseCardIDsJSON(row.CardIDsJSON)
			sids := make([]string, len(ids))
			for i, id := range ids {
				sids[i] = string(id)
			}
			writeJSON(w, http.StatusOK, deckJSON{
				ID:            row.ID,
				Name:          row.Name,
				CardIDs:       sids,
				PlayerSkillID: row.PlayerSkillID,
				SleeveColor:   row.SleeveColor,
			})
			return
		}
		writeAuthError(w, http.StatusNotFound, "deck_not_found")
	case http.MethodPut:
		var body deckCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAuthError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		ids := make([]gameplay.CardID, len(body.CardIDs))
		for i, x := range body.CardIDs {
			ids[i] = gameplay.CardID(x)
		}
		if err := s.decks.UpdateDeck(u.ID, deckID, body.Name, ids, gameplay.PlayerSkillID(body.PlayerSkillID), body.SleeveColor); err != nil {
			if errors.Is(err, ErrDeckNotFound) {
				writeAuthError(w, http.StatusNotFound, "deck_not_found")
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if err := s.decks.DeleteDeck(u.ID, deckID); err != nil {
			if errors.Is(err, ErrDeckNotFound) {
				writeAuthError(w, http.StatusNotFound, "deck_not_found")
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// authUserFromHTTP returns the authenticated user for REST handlers (requires s.auth).
func (s *Server) authUserFromHTTP(w http.ResponseWriter, r *http.Request) (*userModel, bool) {
	if s.auth == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "auth_unavailable")
		return nil, false
	}
	raw := authTokenFromHTTP(r)
	claims, err := s.auth.ParseToken(raw)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "invalid_token")
		return nil, false
	}
	u, err := s.auth.UserByID(claims.UserID)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "user_not_found")
		return nil, false
	}
	return u, true
}

// handleDecksPath routes /api/decks/... subpaths (single deck by id).
func (s *Server) handleDecksPath(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/decks/")
	if rest == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseUint(rest, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.handleDeckByID(w, r, id)
}

// userInActiveRoom reports whether the user currently has a websocket session joined to a room.
func (s *Server) userInActiveRoom(userID uint64) bool {
	if userID == 0 {
		return false
	}
	s.userRoomMu.Lock()
	defer s.userRoomMu.Unlock()
	_, ok := s.userRoom[userID]
	return ok
}
