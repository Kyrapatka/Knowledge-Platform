// Package dashboard exposes read-only, owner-scoped projections for the library
// and statistics. Reading these views never starts learning or refills a pool.
package dashboard

import (
	"time"

	folderhandler "github.com/Kyrapatka/knowledge-platform/internal/core/folder/handler"
	materialhandler "github.com/Kyrapatka/knowledge-platform/internal/core/material/handler"
	"github.com/google/uuid"
)

type Topic struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type PlanOption struct {
	ID               uuid.UUID `json:"id"`
	AlgorithmKey     string    `json:"algorithm_key"`
	AlgorithmVersion int       `json:"algorithm_version"`
	Track            string    `json:"track"`
	Status           string    `json:"status"`
	HorizonDays      int       `json:"horizon_days"`
	PoolSize         int       `json:"pool_size"`
	CreatedAt        time.Time `json:"created_at"`
}

type FolderSummary struct {
	folderhandler.FolderResponse
	MaterialCount  int64       `json:"material_count"`
	DueCount       int64       `json:"due_count"`
	LearningCount  int64       `json:"learning_count"`
	CompletedCount int64       `json:"completed_count"`
	Topics         []Topic     `json:"topics"`
	SelectedPlan   *PlanOption `json:"selected_plan"`
}

type LibraryTotals struct {
	FolderCount    int64 `json:"folder_count"`
	MaterialCount  int64 `json:"material_count"`
	DueCount       int64 `json:"due_count"`
	LearningCount  int64 `json:"learning_count"`
	CompletedCount int64 `json:"completed_count"`
}

type LibraryView struct {
	Folders []FolderSummary `json:"folders"`
	Totals  LibraryTotals   `json:"totals"`
}

type Progress struct {
	Version                 int        `json:"version"`
	Stage                   int        `json:"stage"`
	Track                   string     `json:"track"`
	PlanID                  *uuid.UUID `json:"plan_id"`
	AlgorithmKey            string     `json:"algorithm_key"`
	AlgorithmVersion        int        `json:"algorithm_version"`
	LearningStartedAt       time.Time  `json:"learning_started_at"`
	TargetAt                *time.Time `json:"target_at"`
	CompletedAt             *time.Time `json:"completed_at"`
	StageLastReviewAt       *time.Time `json:"stage_last_review_at"`
	StageReviewAt           *time.Time `json:"stage_review_at"`
	RehabActive             bool       `json:"rehab_active"`
	RehabReviewAt           *time.Time `json:"rehab_review_at"`
	RehabStep               int        `json:"rehab_step"`
	RehabConsecutiveCorrect int        `json:"rehab_consecutive_correct"`
	ExtraReviewAt           *time.Time `json:"extra_review_at"`
	NextReviewAt            *time.Time `json:"next_review_at"`
	ConsecutiveCorrect      int        `json:"consecutive_correct"`
	CorrectCount            int        `json:"correct_count"`
	WrongCount              int        `json:"wrong_count"`
	CanStartFinal           bool       `json:"can_start_final"`
}

type MaterialView struct {
	materialhandler.MaterialResponse
	Topic    string    `json:"topic"`
	Progress *Progress `json:"progress"`
}

type MaterialsView struct {
	Items        []MaterialView `json:"items"`
	Total        int64          `json:"total"`
	Limit        int            `json:"limit"`
	Offset       int            `json:"offset"`
	Topics       []Topic        `json:"topics"`
	SelectedPlan *PlanOption    `json:"selected_plan"`
	Plans        []PlanOption   `json:"plans"`
}

type Activity struct {
	Answers           int64 `json:"answers"`
	Correct           int64 `json:"correct"`
	Wrong             int64 `json:"wrong"`
	MaterialsReviewed int64 `json:"materials_reviewed"`
	Sessions          int64 `json:"sessions"`
	StagePromotions   int64 `json:"stage_promotions"`
}

type DailyActivity struct {
	Date string `json:"date"`
	Activity
}

type StatisticsTotals struct {
	Activity
	ActiveDays int64 `json:"active_days"`
}

type StatisticsView struct {
	Days     int              `json:"days"`
	Timezone string           `json:"timezone"`
	From     time.Time        `json:"from"`
	To       time.Time        `json:"to"`
	Totals   StatisticsTotals `json:"totals"`
	Daily    []DailyActivity  `json:"daily"`
}
