package gameplay

type PlayerSkillID string
type PlayerSkillType string

const (
	PlayerSkillTypePlayerSkill PlayerSkillType = "Player Skill"
)

type PlayerSkillDefinition struct {
	ID          PlayerSkillID
	Name        string
	Type        PlayerSkillType
	Description string
	Example     string
}

// InitialPlayerSkills returns the canonical set of selectable player skills.
func InitialPlayerSkills() []PlayerSkillDefinition {
	return []PlayerSkillDefinition{
		{
			ID:          "reinforcements",
			Name:        "Reinforcements",
			Type:        PlayerSkillTypePlayerSkill,
			Description: "Summon the equivalent of 6 material pieces to your side in their home square if available.",
			Example:     "1. You spend all your energized mana to activate \"Reinforcements\".\n2. You summon a rook at a1 and a pawn at b2. (5+1 material)",
		},
		{
			ID:          "march-forward",
			Name:        "March Forward!",
			Type:        PlayerSkillTypePlayerSkill,
			Description: "Move all your pawns forward one square, pushing any opponent's pieces in the way back one square, except the opponent's king. If a piece were to be pushed off the board, capture it instead.",
			Example:     "1. You have pawns on e4, f5 and h7.\n2. Opponent have a pawn on e5 and f6, also a knight on e6 and a rook on h8.\n3. You activate \"March Forward!\".\n4. The pawn on e4 moves to e5, pushing the opponent's pawn on e5 to e6, and the knight on e6 to e7.\n5. The pawn on f5 moves to f6, pushing the opponent's pawn on f6 to f7.\n6. The pawn on h7 moves to h8, pushing the opponent's rook on h8 out of the board and capturing it.\n7. The pawn now on h8 can promote.",
		},
		{
			ID:          "limitless-potential",
			Name:        "Limitless Potential",
			Type:        PlayerSkillTypePlayerSkill,
			Description: "Increase your maximum mana by 5 permanently and gain +1 mana every turn until the end of the game, you gain double the energized mana for the next 3 activations of \"Power\" cards.",
			Example:     "1. You activate \"Limitless Potential\".\n2. Your maximum mana is increased from 10 to 15.\n3. Next turn you gain 2 mana at the start of the turn.\n4. You activate \"Knight Touch\" for 3 cost but gains 6 energized mana.",
		},
		{
			ID:          "dimension-shift",
			Name:        "Dimension Shift",
			Type:        PlayerSkillTypePlayerSkill,
			Description: "Choose a column, every piece on this column is moved to another column in the same row, if there is no available squares the piece is captured instead. Then choose a row, every piece on this row is teleported back to their closest home square, if there is no available squares the piece is captured instead. The square that is crossed by both column and row selected becomes hollow and cannot have a piece on it anymore for the rest of the game, pieces also cannot move through it. (Every player chooses the location for their pieces)",
			Example:     "1. You have pawns on e4, f5 and h7.\n2. Opponent have pawns on e5, f6 and a rook on h8.\n3. You activate \"Dimension Shift\".\n4. You choose the column e.\n5. The pawn from e4 goes to g4. And the pawn from e5 goes to g5.\n6. You choose the row 5.\n7. The pawn from f5 goes to f2. And the pawn from g5 goes to g7.\n8. The e5 square is now hollow and cannot have a piece on it anymore for the rest of the game, pieces also cannot move through it.",
		},
	}
}

