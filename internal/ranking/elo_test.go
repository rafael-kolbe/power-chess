package ranking

import "testing"

func TestExpectedScoreSymmetry(t *testing.T) {
	a := ExpectedScore(1200, 1200)
	if a != 0.5 {
		t.Fatalf("expected 0.5, got %f", a)
	}
}

func TestUpdateELOWin(t *testing.T) {
	a, b := UpdateELO(1200, 1200, Win, 32)
	if a <= 1200 || b >= 1200 {
		t.Fatalf("winner should gain rating and loser should lose rating")
	}
}

func TestUpdateELODraw(t *testing.T) {
	a, b := UpdateELO(1400, 1200, Draw, 32)
	if a >= 1400 || b <= 1200 {
		t.Fatalf("higher rated player should lose points on draw vs lower rated")
	}
}
