package materialimport

import (
	"errors"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/google/uuid"
)

const MaxItems = 1000
const MaxBody = 10 << 20

var ErrConflict = errors.New("import command already used with different payload")
var ErrInvalid = errors.New("invalid material import")

type ProfileInput struct {
	Frequency  *int     `json:"frequency,omitempty"`
	Difficulty *int     `json:"difficulty,omitempty"`
	LevelMin   *int     `json:"level_min,omitempty"`
	LevelMax   *int     `json:"level_max,omitempty"`
	Topic      string   `json:"topic,omitempty"`
	Subtopic   string   `json:"subtopic,omitempty"`
	Concepts   []string `json:"concepts,omitempty"`
}
type Item struct {
	ClientID        string                   `json:"client_id"`
	Values          map[string]*string       `json:"values"`
	Metadata        map[string]*string       `json:"metadata,omitempty"`
	Difficulty      materialmodel.Difficulty `json:"difficulty,omitempty"`
	Profile         *ProfileInput            `json:"profile,omitempty"`
	DuplicatePolicy string                   `json:"duplicate_policy,omitempty"`
}
type Request struct {
	CommandID       uuid.UUID `json:"command_id"`
	DuplicatePolicy string    `json:"duplicate_policy,omitempty"`
	Items           []Item    `json:"items"`
}
type Issue struct {
	ClientID string `json:"client_id,omitempty"`
	Path     string `json:"path"`
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}
type ItemResult struct {
	ClientID      string    `json:"client_id"`
	Status        string    `json:"status"`
	MaterialID    uuid.UUID `json:"material_id,omitempty"`
	DuplicateOf   string    `json:"duplicate_of,omitempty"`
	DuplicateKind string    `json:"duplicate_kind,omitempty"`
}
type Result struct {
	Valid   bool         `json:"valid"`
	Created int          `json:"created"`
	Skipped int          `json:"skipped"`
	Failed  int          `json:"failed"`
	Issues  []Issue      `json:"issues"`
	Items   []ItemResult `json:"items"`
}
type ValidationError struct{ Result Result }

func (e *ValidationError) Error() string { return "material import validation failed" }
