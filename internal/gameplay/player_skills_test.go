package gameplay

import "testing"

func TestInitialPlayerSkillsCountAndType(t *testing.T) {
	skills := InitialPlayerSkills()
	if len(skills) != 4 {
		t.Fatalf("expected 4 player skills, got %d", len(skills))
	}
	for _, s := range skills {
		if s.Type != PlayerSkillTypePlayerSkill {
			t.Fatalf("invalid skill type for %s: %s", s.ID, string(s.Type))
		}
	}
}

