package ranking

import "math"

type Result float64

const (
	Loss Result = 0.0
	Draw Result = 0.5
	Win  Result = 1.0
)

func ExpectedScore(player, opponent int) float64 {
	return 1.0 / (1.0 + math.Pow(10, float64(opponent-player)/400.0))
}

func UpdateELO(player, opponent int, result Result, k float64) (newPlayer int, newOpponent int) {
	expPlayer := ExpectedScore(player, opponent)
	expOpp := ExpectedScore(opponent, player)
	newPlayerF := float64(player) + k*(float64(result)-expPlayer)
	newOpponentF := float64(opponent) + k*((1.0-float64(result))-expOpp)
	return int(math.Round(newPlayerF)), int(math.Round(newOpponentF))
}
