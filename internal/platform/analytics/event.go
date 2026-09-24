package analytics

import (
	"github.com/google/uuid"
	"time"
)

type Name string

const (
	UserRegistered            Name = "user_registered"
	UserLoggedIn              Name = "user_logged_in"
	LoginFailed               Name = "login_failed"
	FolderCreated             Name = "folder_created"
	FolderImported            Name = "folder_imported"
	FolderDeleted             Name = "folder_deleted"
	MaterialCreated           Name = "material_created"
	MaterialDeleted           Name = "material_deleted"
	TrainingStarted           Name = "training_started"
	TrainingAnswered          Name = "training_answered"
	TrainingCompleted         Name = "training_completed"
	TrainingAbandoned         Name = "training_abandoned"
	TrainingSkipped           Name = "training_skipped"
	TrainingRollback          Name = "training_rollback"
	RehabStarted              Name = "rehab_started"
	RehabCompleted            Name = "rehab_completed"
	CramStarted               Name = "cram_started"
	CramCompleted             Name = "cram_completed"
	InterviewStarted          Name = "interview_started"
	InterviewQuestionAnswered Name = "interview_question_answered"
	InterviewCompleted        Name = "interview_completed"
)

// Event is a closed, allowlisted schema: no arbitrary payload, answers or secrets.
// Pointer fields represent unavailable dimensions as NULL, not fabricated zeros.
// Producers transfer an immutable snapshot to Publish.
type Event struct {
	EventID                  string     `json:"event_id"`
	EventName                Name       `json:"event_name"`
	OccurredAt               time.Time  `json:"occurred_at"`
	UserID                   string     `json:"user_id"`
	SessionID                string     `json:"session_id"`
	AppVersion               string     `json:"app_version"`
	AlgorithmVersion         string     `json:"algorithm_version"`
	ExperimentGroup          string     `json:"experiment_group"`
	FolderID                 string     `json:"folder_id"`
	MaterialID               string     `json:"material_id"`
	Template                 string     `json:"template"`
	Topic                    *string    `json:"topic"`
	Subtopic                 *string    `json:"subtopic"`
	Difficulty               *string    `json:"difficulty"`
	Mode                     string     `json:"mode"`
	Result                   string     `json:"result"`
	ReviewKind               string     `json:"review_kind"`
	AnswerTimeMS             *int64     `json:"answer_time_ms"`
	StageBefore              *int       `json:"stage_before"`
	StageAfter               *int       `json:"stage_after"`
	ConsecutiveCorrectBefore *int       `json:"consecutive_correct_before"`
	ConsecutiveCorrectAfter  *int       `json:"consecutive_correct_after"`
	WrongCountBefore         *int       `json:"wrong_count_before"`
	WrongCountAfter          *int       `json:"wrong_count_after"`
	RehabActiveBefore        *bool      `json:"rehab_active_before"`
	RehabActiveAfter         *bool      `json:"rehab_active_after"`
	RetrievabilityBefore     *float64   `json:"retrievability_before"`
	RetrievabilityAfter      *float64   `json:"retrievability_after"`
	StabilityBefore          *float64   `json:"stability_before"`
	StabilityAfter           *float64   `json:"stability_after"`
	NextReviewBefore         *time.Time `json:"next_review_before"`
	NextReviewAfter          *time.Time `json:"next_review_after"`
	LearnedBefore            *bool      `json:"learned_before"`
	LearnedAfter             *bool      `json:"learned_after"`
	ReviewCredit             *bool      `json:"review_credit"`
	RelatedEventID           string     `json:"related_event_id"`
}

func New(name Name, user uuid.UUID) Event {
	u := ""
	if user != uuid.Nil {
		u = user.String()
	}
	return Event{EventID: uuid.NewString(), EventName: name, OccurredAt: time.Now().UTC(), UserID: u}
}
func Ptr[T any](v T) *T { return &v }
func (n Name) valid() bool {
	switch n {
	case UserRegistered, UserLoggedIn, LoginFailed, FolderCreated, FolderImported, FolderDeleted, MaterialCreated, MaterialDeleted, TrainingStarted, TrainingAnswered, TrainingCompleted, TrainingAbandoned, TrainingSkipped, TrainingRollback, RehabStarted, RehabCompleted, CramStarted, CramCompleted, InterviewStarted, InterviewQuestionAnswered, InterviewCompleted:
		return true
	}
	return false
}
