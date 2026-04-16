package server

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// graveyardPieceImportance returns a sort key for piece codes so the graveyard
// is ordered from most to least important: Q > R > B > N > P (King never captured).
func graveyardPieceImportance(code string) int {
	if len(code) < 2 {
		return 99
	}
	switch code[1] {
	case 'Q':
		return 0
	case 'R':
		return 1
	case 'B':
		return 2
	case 'N':
		return 3
	case 'P':
		return 4
	}
	return 5
}

// playerHUDState converts internal player state to transport-friendly HUD data.
// sleeve is the player's chosen sleeve color; viewerPID restricts which hand is included.
// reactionMode is off / on / auto for that seat.
func playerHUDState(pid gameplay.PlayerID, p *gameplay.PlayerState, sleeve string, viewerPID gameplay.PlayerID, reactionMode string) PlayerHUDState {
	// Full cooldown queue in preview (UI overlaps cards like the hand; no separate +N overflow tile).
	preview := make([]CooldownPreviewEntry, 0, len(p.Cooldowns))
	for _, cd := range p.Cooldowns {
		preview = append(preview, CooldownPreviewEntry{
			CardID:         string(cd.Card.CardID),
			ManaCost:       cd.Card.ManaCost,
			Ignition:       cd.Card.Ignition,
			Cooldown:       cd.Card.Cooldown,
			TurnsRemaining: cd.TurnsRemaining,
		})
	}

	// Build banished card list (most recently banished first).
	banished := make([]CardSnapshotEntry, 0, len(p.Banished))
	for i := len(p.Banished) - 1; i >= 0; i-- {
		c := p.Banished[i]
		banished = append(banished, CardSnapshotEntry{
			CardID:   string(c.CardID),
			ManaCost: c.ManaCost,
			Ignition: c.Ignition,
			Cooldown: c.Cooldown,
		})
	}

	// Build graveyard piece list ordered by importance.
	graveyard := make([]string, 0, len(p.Graveyard))
	for _, pr := range p.Graveyard {
		graveyard = append(graveyard, pr.Color+pr.Type)
	}
	// Stable sort by importance order.
	for i := 1; i < len(graveyard); i++ {
		for j := i; j > 0 && graveyardPieceImportance(graveyard[j]) < graveyardPieceImportance(graveyard[j-1]); j-- {
			graveyard[j], graveyard[j-1] = graveyard[j-1], graveyard[j]
		}
	}

	hud := PlayerHUDState{
		PlayerID:            string(pid),
		Mana:                p.Mana,
		MaxMana:             p.MaxMana,
		EnergizedMana:       p.EnergizedMana,
		MaxEnergized:        p.MaxEnergizedMana,
		HandCount:           len(p.Hand),
		CooldownCount:       len(p.Cooldowns),
		GraveyardCount:      len(p.Graveyard),
		DeckCount:           len(p.Deck),
		SleeveColor:         DefaultSleeveColor(sleeve),
		BanishedCards:       banished,
		GraveyardPieces:     graveyard,
		CooldownPreview:     preview,
		CooldownHiddenCount: 0,
		ReactionMode:        reactionMode,
	}
	if p.Ignition.Occupied {
		hud.IgnitionOn = true
		hud.IgnitionCard = string(p.Ignition.Card.CardID)
		hud.IgnitionTurnsRemaining = p.Ignition.TurnsRemaining
	}

	// Include hand only for the owning player.
	if viewerPID == pid {
		hand := make([]CardSnapshotEntry, 0, len(p.Hand))
		for _, c := range p.Hand {
			hand = append(hand, CardSnapshotEntry{
				CardID:   string(c.CardID),
				ManaCost: c.ManaCost,
				Ignition: c.Ignition,
				Cooldown: c.Cooldown,
			})
		}
		hud.Hand = hand
	}

	return hud
}

// serializeBoard converts engine board pieces to compact string identifiers.
func serializeBoard(board [8][8]chess.Piece) [8][8]string {
	out := [8][8]string{}
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			out[r][c] = pieceCode(board[r][c])
		}
	}
	return out
}

// pieceCode maps internal piece representation to transport code (e.g. "wK", "bP").
func pieceCode(p chess.Piece) string {
	if p.IsEmpty() {
		return ""
	}
	color := "w"
	if p.Color == chess.Black {
		color = "b"
	}
	pt := ""
	switch p.Type {
	case chess.Pawn:
		pt = "P"
	case chess.Knight:
		pt = "N"
	case chess.Bishop:
		pt = "B"
	case chess.Rook:
		pt = "R"
	case chess.Queen:
		pt = "Q"
	case chess.King:
		pt = "K"
	}
	return color + pt
}
