package model

import (
	"github.com/google/uuid"
	"time"
)

type PracticeMode string

const (
	PracticeWorked      PracticeMode = "worked"
	PracticeFaded       PracticeMode = "faded"
	PracticeIndependent PracticeMode = "independent"
	PracticeMixed       PracticeMode = "mixed"
	PracticeMaintenance PracticeMode = "maintenance"
)

type FormulaExercise struct {
	ID         uuid.UUID `json:"id"`
	MaterialID uuid.UUID `json:"material_id"`
	Problem    string    `json:"problem"`
	Answer     string    `json:"answer"`
	Solution   string    `json:"solution"`
	Hint       string    `json:"hint"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
