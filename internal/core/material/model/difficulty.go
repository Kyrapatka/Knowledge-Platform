package model

type Difficulty string

const (
	DifficultyEasy   Difficulty = "easy"
	DifficultyMedium Difficulty = "medium"
	DifficultyHard   Difficulty = "hard"
)

func (d Difficulty) Valid() bool {
	return d == DifficultyEasy || d == DifficultyMedium || d == DifficultyHard
}
