package graph

import (
	"fmt"
	"github.com/google/uuid"
	"math"
)

type Config struct {
	MaxRoots            int     `json:"max_roots"`
	MaxDepthPerBranch   int     `json:"max_depth_per_branch"`
	MaxForksPerRoot     int     `json:"max_forks_per_root"`
	QuestionLimit       int     `json:"question_limit"`
	Temperature         float64 `json:"temperature"`
	MaxDetectedConcepts int     `json:"max_detected_concepts"`
	CrossTopicPenalty   float64 `json:"cross_topic_penalty"`
	EarlyReviewPolicy   string  `json:"early_review_policy"`
	StoreRawAnswer      bool    `json:"store_raw_answer"`
}

func DefaultConfig() Config { return Config{3, 10, 2, 24, .85, 6, .35, "no_credit", false} }
func (c Config) Validate() error {
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	if c.MaxRoots < 1 || c.MaxRoots > 10 || c.MaxDepthPerBranch < 1 || c.MaxDepthPerBranch > 20 || c.MaxForksPerRoot < 0 || c.MaxForksPerRoot > 10 || c.QuestionLimit < 1 || c.QuestionLimit > 100 || !finite(c.Temperature) || c.Temperature <= 0 || c.Temperature > 5 || c.MaxDetectedConcepts < 1 || c.MaxDetectedConcepts > 20 || !finite(c.CrossTopicPenalty) || c.CrossTopicPenalty < 0 || c.CrossTopicPenalty > 20 || c.EarlyReviewPolicy != "no_credit" {
		return fmt.Errorf("invalid graph configuration")
	}
	return nil
}

type Source struct {
	FolderID uuid.UUID `json:"folder_id"`
	Topics   []string  `json:"topics,omitempty"`
}
type Link struct {
	Slug   string  `json:"slug"`
	Role   string  `json:"role"`
	Weight float64 `json:"weight"`
}
type Candidate struct {
	MaterialID          uuid.UUID `json:"material_id"`
	FolderID            uuid.UUID `json:"folder_id"`
	SeedKey             string    `json:"seed_key,omitempty"`
	Question            string    `json:"question"`
	Domain              string    `json:"domain"`
	Topic               string    `json:"topic"`
	Status              string    `json:"status"`
	Frequency           int       `json:"frequency"`
	FrequencyConfidence float64   `json:"frequency_confidence"`
	InterviewDifficulty int       `json:"interview_difficulty"`
	Specificity         int       `json:"specificity"`
	RootWeight          int       `json:"root_weight"`
	ProfileVersion      int       `json:"profile_version"`
	HasAnswer           bool      `json:"has_answer"`
	Due                 bool      `json:"due"`
	New                 bool      `json:"new"`
	LearningNeed        float64   `json:"learning_need"`
	Concepts            []Link    `json:"concepts" gorm:"-"`
}
type Constraints struct {
	RequiresAny    []string `json:"requires_any,omitempty"`
	RequiresDomain string   `json:"requires_domain,omitempty"`
}
type Alias struct {
	ConceptID            uuid.UUID
	Slug, Text, Language string
	Weight               float64
	WholeWord            bool
	Constraints          Constraints
}
type Edge struct {
	From, To, Relation string
	Weight             float64
}
type Catalog struct {
	Aliases           []Alias
	Edges             []Edge
	DocumentFrequency map[string]int
	QuestionCount     int
}
type Match struct {
	ConceptID uuid.UUID `json:"concept_id"`
	Slug      string    `json:"slug"`
	Alias     string    `json:"alias"`
	Start     int       `json:"start"`
	End       int       `json:"end"`
	Strength  float64   `json:"strength"`
	Source    string    `json:"source"`
}
type Score struct {
	MaterialID uuid.UUID          `json:"material_id"`
	SeedKey    string             `json:"seed_key,omitempty"`
	Question   string             `json:"question"`
	Score      float64            `json:"score"`
	Reason     string             `json:"reason"`
	Tier       string             `json:"tier"`
	Components map[string]float64 `json:"components"`
}
type FrontierEntry struct {
	OriginMaterialID uuid.UUID `json:"origin_material_id"`
	MaterialID       uuid.UUID `json:"material_id"`
	RootIndex        int       `json:"root_index"`
	Branch           string    `json:"branch"`
	SourceDepth      int       `json:"source_depth"`
	Reason           string    `json:"reason"`
	Status           string    `json:"status"`
	Score            float64   `json:"score"`
}
type State struct {
	StrategyVersion  int             `json:"strategy_version"`
	Version          int             `json:"version"`
	RandomSeed       int64           `json:"random_seed"`
	RandomIndex      uint64          `json:"random_index"`
	CurrentRoot      int             `json:"current_root"`
	CurrentBranch    string          `json:"current_branch"`
	CurrentDepth     int             `json:"current_depth"`
	RootsUsed        int             `json:"roots_used"`
	QuestionsAsked   int             `json:"questions_asked"`
	Config           Config          `json:"config"`
	Sources          []Source        `json:"sources"`
	AskedMaterialIDs []uuid.UUID     `json:"asked_material_ids"`
	RecentConcepts   []string        `json:"recent_concepts"`
	Frontier         []FrontierEntry `json:"frontier"`
	ForksUsed        map[int]int     `json:"forks_used"`
	LastMaterialID   *uuid.UUID      `json:"last_material_id,omitempty"`
	StopReason       string          `json:"stop_reason"`
}
type Selection struct {
	Candidate    *Candidate `json:"selected,omitempty"`
	State        State      `json:"state"`
	Matches      []Match    `json:"detected_concepts"`
	Scores       []Score    `json:"candidates"`
	Reason       string     `json:"selection_reason"`
	ReviewCredit bool       `json:"review_credit"`
}
