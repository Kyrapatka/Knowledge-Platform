package interview

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"math"
	"regexp"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid interview request")
var ErrConflict = errors.New("interview profile changed")
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,95}$`)

type QuestionConcept struct {
	Slug   string  `json:"slug"`
	Role   string  `json:"role"`
	Weight float64 `json:"weight"`
}
type Profile struct {
	MaterialID          uuid.UUID         `json:"material_id"`
	FolderID            uuid.UUID         `json:"folder_id"`
	SeedKey             *string           `json:"seed_key"`
	Domain              string            `json:"domain"`
	Frequency           int               `json:"frequency"`
	FrequencyConfidence float64           `json:"frequency_confidence"`
	InterviewDifficulty int               `json:"interview_difficulty"`
	Specificity         int               `json:"specificity"`
	RootWeight          int               `json:"root_weight"`
	Status              string            `json:"status"`
	ProfileVersion      int               `json:"profile_version"`
	Concepts            []QuestionConcept `json:"concepts" gorm:"-"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}
type ProfileRequest struct {
	Profile
	ExpectedVersion int  `json:"expected_version"`
	Ungraded        bool `json:"ungraded"`
}
type AliasConstraints struct {
	RequiresAny    []string `json:"requires_any,omitempty"`
	RequiresDomain string   `json:"requires_domain,omitempty"`
}
type Alias struct {
	Alias       string           `json:"alias"`
	Language    string           `json:"language"`
	Weight      float64          `json:"weight"`
	WholeWord   bool             `json:"whole_word"`
	Constraints AliasConstraints `json:"constraints"`
}
type Concept struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	DisplayName string    `json:"display_name"`
	Domain      string    `json:"domain"`
	Topic       string    `json:"topic"`
	Version     int       `json:"version"`
	Aliases     []Alias   `json:"aliases" gorm:"-"`
}
type Edge struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Relation string  `json:"relation"`
	Weight   float64 `json:"weight"`
}
type Catalog struct {
	Concepts []Concept `json:"concepts"`
	Edges    []Edge    `json:"edges"`
}

func (p *Profile) validate() error {
	if p.Frequency < 1 || p.Frequency > 10 || p.InterviewDifficulty < 1 || p.InterviewDifficulty > 5 || p.Specificity < 1 || p.Specificity > 5 || p.RootWeight < 0 || p.RootWeight > 10 || math.IsNaN(p.FrequencyConfidence) || p.FrequencyConfidence < 0 || p.FrequencyConfidence > 1 {
		return fmt.Errorf("%w: profile metrics are out of range", ErrInvalid)
	}
	if p.Status != "draft" && p.Status != "ready" && p.Status != "archived" {
		return fmt.Errorf("%w: status must be draft, ready or archived", ErrInvalid)
	}
	if len(p.Domain) > 96 || len(p.Concepts) > 64 || (p.SeedKey != nil && (len(*p.SeedKey) > 100 || strings.TrimSpace(*p.SeedKey) == "")) {
		return ErrInvalid
	}
	seen := map[string]bool{}
	primary, tested := 0, 0
	for i, c := range p.Concepts {
		if c.Role == "primary" {
			primary++
		}
		if c.Role == "tested" {
			tested++
		}
		if !slugPattern.MatchString(c.Slug) || (c.Role != "primary" && c.Role != "tested" && c.Role != "hook" && c.Role != "prerequisite") || seen[c.Slug+":"+c.Role] {
			return fmt.Errorf("%w: invalid or duplicate concept", ErrInvalid)
		}
		seen[c.Slug+":"+c.Role] = true
		if c.Weight == 0 {
			p.Concepts[i].Weight = 1
		} else if c.Weight < 0 || c.Weight > 2 || math.IsNaN(c.Weight) {
			return ErrInvalid
		}
	}
	if p.Status == "ready" && (primary != 1 || tested < 1) {
		return fmt.Errorf("%w: ready questions need exactly one primary and at least one tested concept", ErrInvalid)
	}
	return nil
}
func jsonBytes(v any) []byte { b, _ := json.Marshal(v); return b }
