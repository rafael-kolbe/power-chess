package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"power-chess/internal/gameplay"
)

// Sleeve colors must match assets under web/public/sleeves/.
const (
	SleeveBlue  = "blue"
	SleeveGreen = "green"
	SleevePink  = "pink"
	SleeveRed   = "red"
)

var validSleeveColors = map[string]struct{}{
	SleeveBlue: {}, SleeveGreen: {}, SleevePink: {}, SleeveRed: {},
}

// ErrTooManyDecks is returned when a user already has MaxSavedDecksPerUser decks.
var ErrTooManyDecks = errors.New("maximum saved decks reached")

// ErrDeckNotFound is returned when a deck id does not exist for that user.
var ErrDeckNotFound = errors.New("deck not found")

// ErrNoLobbyDeck is returned when the user has no deck selected and none can be inferred.
var ErrNoLobbyDeck = errors.New("no lobby deck selected")

// DeckService persists and validates user decks.
type DeckService struct {
	db          *gorm.DB
	userInMatch func(userID uint64) bool
}

// NewDeckService wires deck persistence. userInMatch should report true while the account is bound to an active room.
func NewDeckService(db *gorm.DB, userInMatch func(userID uint64) bool) *DeckService {
	if userInMatch == nil {
		userInMatch = func(uint64) bool { return false }
	}
	return &DeckService{db: db, userInMatch: userInMatch}
}

// EnsureDefaultDeckForUser creates the seeded "Default" deck when the user has zero decks, and sets lobby selection.
func (s *DeckService) EnsureDefaultDeckForUser(userID uint64) error {
	var n int64
	if err := s.db.Model(&userDeckModel{}).Where("user_id = ?", userID).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return s.ensureLobbyDeckPointsAtExisting(userID)
	}
	ids := gameplay.DefaultDeckPresetCardIDs()
	return s.createDeckRecord(userID, gameplay.DefaultDeckDisplayName, ids, defaultSeedSkill(), defaultSeedSleeve(), true)
}

// ensureLobbyDeckPointsAtExisting sets LobbyDeckID if null and user has decks.
func (s *DeckService) ensureLobbyDeckPointsAtExisting(userID uint64) error {
	var u userModel
	if err := s.db.Where("id = ?", userID).Take(&u).Error; err != nil {
		return err
	}
	if u.LobbyDeckID != nil {
		var exists int64
		_ = s.db.Model(&userDeckModel{}).Where("id = ? AND user_id = ?", *u.LobbyDeckID, userID).Count(&exists).Error
		if exists > 0 {
			return nil
		}
	}
	var first userDeckModel
	if err := s.db.Where("user_id = ?", userID).Order("id ASC").Take(&first).Error; err != nil {
		return err
	}
	return s.db.Model(&userModel{}).Where("id = ?", userID).Update("lobby_deck_id", first.ID).Error
}

func defaultSeedSkill() gameplay.PlayerSkillID {
	return gameplay.PlayerSkillID("reinforcements")
}

func defaultSeedSleeve() string {
	return SleeveBlue
}

func (s *DeckService) createDeckRecord(userID uint64, name string, ids []gameplay.CardID, skill gameplay.PlayerSkillID, sleeve string, setAsLobby bool) error {
	raw, err := json.Marshal(cardIDsToStrings(ids))
	if err != nil {
		return err
	}
	d := userDeckModel{
		UserID:        userID,
		Name:          strings.TrimSpace(name),
		CardIDsJSON:   raw,
		PlayerSkillID: string(skill),
		SleeveColor:   sleeve,
	}
	if err := s.db.Create(&d).Error; err != nil {
		return err
	}
	if setAsLobby {
		return s.db.Model(&userModel{}).Where("id = ?", userID).Update("lobby_deck_id", d.ID).Error
	}
	return nil
}

func cardIDsToStrings(ids []gameplay.CardID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

func parseCardIDsJSON(raw []byte) ([]gameplay.CardID, error) {
	var strs []string
	if err := json.Unmarshal(raw, &strs); err != nil {
		return nil, err
	}
	out := make([]gameplay.CardID, len(strs))
	for i, s := range strs {
		out[i] = gameplay.CardID(s)
	}
	return out, nil
}

// BackfillDefaultDecksForUsersWithout inserts the seeded deck for every user that has zero decks.
func (s *DeckService) BackfillDefaultDecksForUsersWithout(ctx context.Context) error {
	var users []userModel
	if err := s.db.WithContext(ctx).Find(&users).Error; err != nil {
		return err
	}
	for _, u := range users {
		var n int64
		if err := s.db.Model(&userDeckModel{}).Where("user_id = ?", u.ID).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			if err := s.EnsureDefaultDeckForUser(u.ID); err != nil {
				return fmt.Errorf("backfill user %d: %w", u.ID, err)
			}
		}
	}
	return nil
}

// DeckCount returns how many decks the user has saved.
func (s *DeckService) DeckCount(userID uint64) (int64, error) {
	var n int64
	err := s.db.Model(&userDeckModel{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

// UserHasAnyDeck reports whether the user can enter a match (requires at least one saved deck).
func (s *DeckService) UserHasAnyDeck(userID uint64) (bool, error) {
	n, err := s.DeckCount(userID)
	return n > 0, err
}

// CreateDeck validates and stores a new deck (max 10 per user).
func (s *DeckService) CreateDeck(userID uint64, name string, ids []gameplay.CardID, skill gameplay.PlayerSkillID, sleeve string) (*userDeckModel, error) {
	n, err := s.DeckCount(userID)
	if err != nil {
		return nil, err
	}
	if n >= gameplay.MaxSavedDecksPerUser {
		return nil, ErrTooManyDecks
	}
	if err := gameplay.ValidateDeckComposition(ids); err != nil {
		return nil, err
	}
	if !gameplay.ValidPlayerSkillID(skill) {
		return nil, fmt.Errorf("invalid player skill id")
	}
	if !validSleeve(sleeve) {
		return nil, fmt.Errorf("invalid sleeve color")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("deck name is required")
	}
	raw, err := json.Marshal(cardIDsToStrings(ids))
	if err != nil {
		return nil, err
	}
	d := userDeckModel{
		UserID:        userID,
		Name:          strings.TrimSpace(name),
		CardIDsJSON:   raw,
		PlayerSkillID: string(skill),
		SleeveColor:   sleeve,
	}
	if err := s.db.Create(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func validSleeve(s string) bool {
	_, ok := validSleeveColors[strings.TrimSpace(s)]
	return ok
}

// DefaultSleeveColor returns s when it is a known sleeve asset key; otherwise SleeveBlue.
func DefaultSleeveColor(s string) string {
	s = strings.TrimSpace(s)
	if validSleeve(s) {
		return s
	}
	return SleeveBlue
}

// UpdateDeck replaces deck contents and metadata for an owned deck.
func (s *DeckService) UpdateDeck(userID, deckID uint64, name string, ids []gameplay.CardID, skill gameplay.PlayerSkillID, sleeve string) error {
	if s.userInMatch(userID) {
		return fmt.Errorf("cannot edit deck while a match is in progress")
	}
	if err := gameplay.ValidateDeckComposition(ids); err != nil {
		return err
	}
	if !gameplay.ValidPlayerSkillID(skill) {
		return fmt.Errorf("invalid player skill id")
	}
	if !validSleeve(sleeve) {
		return fmt.Errorf("invalid sleeve color")
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("deck name is required")
	}
	var d userDeckModel
	if err := s.db.Where("id = ? AND user_id = ?", deckID, userID).Take(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeckNotFound
		}
		return err
	}
	raw, err := json.Marshal(cardIDsToStrings(ids))
	if err != nil {
		return err
	}
	d.Name = strings.TrimSpace(name)
	d.CardIDsJSON = raw
	d.PlayerSkillID = string(skill)
	d.SleeveColor = sleeve
	return s.db.Save(&d).Error
}

// DeleteDeck removes a deck; re-points lobby deck if needed.
func (s *DeckService) DeleteDeck(userID, deckID uint64) error {
	if s.userInMatch(userID) {
		return fmt.Errorf("cannot delete deck while a match is in progress")
	}
	var u userModel
	if err := s.db.Where("id = ?", userID).Take(&u).Error; err != nil {
		return err
	}
	res := s.db.Where("id = ? AND user_id = ?", deckID, userID).Delete(&userDeckModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDeckNotFound
	}
	if u.LobbyDeckID != nil && *u.LobbyDeckID == deckID {
		var next userDeckModel
		err := s.db.Where("user_id = ?", userID).Order("id ASC").Take(&next).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.db.Model(&userModel{}).Where("id = ?", userID).Update("lobby_deck_id", nil).Error
		}
		if err != nil {
			return err
		}
		return s.db.Model(&userModel{}).Where("id = ?", userID).Update("lobby_deck_id", next.ID).Error
	}
	return nil
}

// SetLobbyDeck sets which deck will be used for the next match.
func (s *DeckService) SetLobbyDeck(userID, deckID uint64) error {
	if s.userInMatch(userID) {
		return fmt.Errorf("cannot change lobby deck while a match is in progress")
	}
	var d userDeckModel
	if err := s.db.Where("id = ? AND user_id = ?", deckID, userID).Take(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeckNotFound
		}
		return err
	}
	return s.db.Model(&userModel{}).Where("id = ?", userID).Update("lobby_deck_id", deckID).Error
}

// ListDecks returns all decks for a user ordered by id ascending.
func (s *DeckService) ListDecks(userID uint64) ([]userDeckModel, error) {
	var rows []userDeckModel
	err := s.db.Where("user_id = ?", userID).Order("id ASC").Find(&rows).Error
	return rows, err
}

// DeckInstancesAndSkillForLobby resolves the user's lobby deck to instances and skill for match setup.
func (s *DeckService) DeckInstancesAndSkillForLobby(userID uint64) ([]gameplay.CardInstance, gameplay.PlayerSkillID, error) {
	inst, skill, _, err := s.DeckInstancesSkillAndSleeveForLobby(userID)
	return inst, skill, err
}

// DeckInstancesSkillAndSleeveForLobby resolves the user's lobby deck to instances, skill, and sleeve color.
func (s *DeckService) DeckInstancesSkillAndSleeveForLobby(userID uint64) ([]gameplay.CardInstance, gameplay.PlayerSkillID, string, error) {
	var u userModel
	if err := s.db.Where("id = ?", userID).Take(&u).Error; err != nil {
		return nil, "", "", err
	}
	if u.LobbyDeckID == nil {
		return nil, "", "", ErrNoLobbyDeck
	}
	inst, skill, sleeve, err := s.DeckInstancesSkillAndSleeveForDeck(userID, *u.LobbyDeckID)
	return inst, skill, sleeve, err
}

// DeckInstancesForDeck loads a specific owned deck by id.
func (s *DeckService) DeckInstancesForDeck(userID, deckID uint64) ([]gameplay.CardInstance, gameplay.PlayerSkillID, error) {
	inst, skill, _, err := s.DeckInstancesSkillAndSleeveForDeck(userID, deckID)
	return inst, skill, err
}

// DeckInstancesSkillAndSleeveForDeck loads a specific owned deck returning instances, skill, and sleeve color.
func (s *DeckService) DeckInstancesSkillAndSleeveForDeck(userID, deckID uint64) ([]gameplay.CardInstance, gameplay.PlayerSkillID, string, error) {
	var d userDeckModel
	if err := s.db.Where("id = ? AND user_id = ?", deckID, userID).Take(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "", ErrDeckNotFound
		}
		return nil, "", "", err
	}
	ids, err := parseCardIDsJSON(d.CardIDsJSON)
	if err != nil {
		return nil, "", "", err
	}
	inst, err := gameplay.DeckInstancesFromCardIDs(ids)
	if err != nil {
		return nil, "", "", err
	}
	return inst, gameplay.PlayerSkillID(d.PlayerSkillID), d.SleeveColor, nil
}
