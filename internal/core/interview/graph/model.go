package graph

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"math"
)

type Config struct {
	InterviewMode       string             `json:"interview_mode"`
	DepthLevel          int                `json:"depth_level"`
	CustomWeights       map[string]float64 `json:"custom_weights,omitempty"`
	MaxRoots            int                `json:"max_roots"`
	MaxDepthPerBranch   int                `json:"max_depth_per_branch"`
	MaxForksPerRoot     int                `json:"max_forks_per_root"`
	QuestionLimit       int                `json:"question_limit"`
	Temperature         float64            `json:"temperature"`
	MaxDetectedConcepts int                `json:"max_detected_concepts"`
	CrossTopicPenalty   float64            `json:"cross_topic_penalty"`
	EarlyReviewPolicy   string             `json:"early_review_policy"`
	StoreRawAnswer      bool               `json:"store_raw_answer"`
	IncludeDraft        bool               `json:"include_draft"`
	Profile             string             `json:"profile"`
	Level               int                `json:"level"`
	MaxFrontierSize     int                `json:"max_frontier_size"`
}

func DefaultConfig() Config {
	return Config{InterviewMode: "real", DepthLevel: 1, MaxRoots: 6, MaxDepthPerBranch: 20, MaxForksPerRoot: 2, QuestionLimit: 24, Temperature: .85, MaxDetectedConcepts: 6, CrossTopicPenalty: .8, EarlyReviewPolicy: "no_credit", Profile: "all", Level: 3, MaxFrontierSize: 12}
}

// Fill absent JSON properties without treating explicitly supplied zero limits
// as defaults. This also keeps pre-bank persisted sessions resumable.
func (c *Config) UnmarshalJSON(data []byte) error {
	type plain Config
	v := plain(DefaultConfig())
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*c = Config(v)
	return nil
}
func (c Config) Validate() error {
	if c.InterviewMode != "" && c.InterviewMode != "real" && c.InterviewMode != "balanced" && c.InterviewMode != "custom" && c.InterviewMode != "deep" {
		return fmt.Errorf("unknown interview mode")
	}
	if c.InterviewMode == "deep" && (c.DepthLevel < 1 || c.DepthLevel > 3) {
		return fmt.Errorf("deep interview depth must be 1, 2 or 3")
	}
	for k, v := range c.CustomWeights {
		if !KnownTopic(k) || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) || v > 1000000 {
			return fmt.Errorf("invalid custom topic weight")
		}
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	if c.MaxRoots < 1 || c.MaxRoots > 10 || c.MaxDepthPerBranch < 1 || c.MaxDepthPerBranch > 20 || c.MaxForksPerRoot < 0 || c.MaxForksPerRoot > 10 || c.QuestionLimit < 1 || c.QuestionLimit > 100 || !finite(c.Temperature) || c.Temperature <= 0 || c.Temperature > 5 || c.MaxDetectedConcepts < 1 || c.MaxDetectedConcepts > 20 || !finite(c.CrossTopicPenalty) || c.CrossTopicPenalty < 0 || c.CrossTopicPenalty > 20 || c.EarlyReviewPolicy != "no_credit" {
		return fmt.Errorf("invalid graph configuration")
	}
	if c.Level < 1 || c.Level > 5 || len(c.Profile) == 0 || len(c.Profile) > 80 {
		return fmt.Errorf("invalid interview profile or level")
	}
	if c.MaxFrontierSize < 1 || c.MaxFrontierSize > 30 {
		return fmt.Errorf("invalid frontier limit")
	}
	for _, ch := range c.Profile {
		if ch != '_' && ch != '-' && (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
			return fmt.Errorf("invalid interview profile")
		}
	}
	return nil
}

type Source struct {
	FolderID uuid.UUID `json:"folder_id"`
	Topics   []string  `json:"topics,omitempty"`
}
type Link struct {
	Slug    string  `json:"slug"`
	Role    string  `json:"role"`
	Weight  float64 `json:"weight"`
	Ordinal int     `json:"ordinal"`
}
type Candidate struct {
	FolderTitle         string    `json:"folder_title,omitempty"`
	Keywords            string    `json:"keywords,omitempty"`
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
	FollowupWeight      int       `json:"followup_weight"`
	LevelMin            int       `json:"level_min"`
	LevelMax            int       `json:"level_max"`
	Subtopic            string    `json:"subtopic"`
	Profiles            []string  `json:"interview_profiles" gorm:"-"`
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
	AddedAt          int       `json:"added_at"`
}
type State struct {
	PracticeOnly      bool            `json:"practice_only"`
	Plan              *InterviewPlan  `json:"interview_plan,omitempty"`
	AnsweredQuestions int             `json:"answered_questions"`
	BranchAnswered    int             `json:"branch_answered"`
	CurrentRootID     uuid.UUID       `json:"current_root_id"`
	ShownRootIDs      []uuid.UUID     `json:"shown_root_ids"`
	SkippedRootIDs    []uuid.UUID     `json:"skipped_root_ids"`
	CompletedRootIDs  []uuid.UUID     `json:"completed_root_ids"`
	RootAreas         []string        `json:"root_areas"`
	RootConcepts      []string        `json:"root_concepts"`
	StrategyVersion   int             `json:"strategy_version"`
	Version           int             `json:"version"`
	RandomSeed        int64           `json:"random_seed"`
	RandomIndex       uint64          `json:"random_index"`
	CurrentRoot       int             `json:"current_root"`
	CurrentBranch     string          `json:"current_branch"`
	CurrentDepth      int             `json:"current_depth"`
	RootsUsed         int             `json:"roots_used"`
	QuestionsAsked    int             `json:"questions_asked"`
	Config            Config          `json:"config"`
	Sources           []Source        `json:"sources"`
	AskedMaterialIDs  []uuid.UUID     `json:"asked_material_ids"`
	RecentConcepts    []string        `json:"recent_concepts"`
	Frontier          []FrontierEntry `json:"frontier"`
	ForksUsed         map[int]int     `json:"forks_used"`
	LastMaterialID    *uuid.UUID      `json:"last_material_id,omitempty"`
	StopReason        string          `json:"stop_reason"`
}
type Selection struct {
	Candidate    *Candidate `json:"selected,omitempty"`
	State        State      `json:"state"`
	Matches      []Match    `json:"detected_concepts"`
	Scores       []Score    `json:"candidates"`
	Reason       string     `json:"selection_reason"`
	ReviewCredit bool       `json:"review_credit"`
}
