package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	trainingpg "github.com/Kyrapatka/knowledge-platform/internal/core/training/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/service"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"github.com/google/uuid"
)

type publishFunc func(context.Context, analytics.Event)

func (f publishFunc) Publish(ctx context.Context, e analytics.Event) { f(ctx, e) }

func TestAnalyticsAfterRealCommitAndNeverAfterRollback(t *testing.T) {
	f := newFixture(t)
	f.material(t, "private question text")
	s := service.NewServiceWithClock(trainingpg.NewRuntimeStore(f.db), func() time.Time { return f.now })
	var events []analytics.Event
	s.SetPublisher(publishFunc(func(ctx context.Context, e analytics.Event) {
		if e.EventName == analytics.TrainingAnswered {
			var n int64
			if err := f.db.Table("training_events").Where("id=?", e.EventID).Count(&n).Error; err != nil || n != 1 {
				t.Error("published before committed event visible", err)
			}
		}
		events = append(events, e)
	}))
	p, err := s.CreatePlan(context.Background(), f.user, service.CreatePlanRequest{SourceFolderIDs: []uuid.UUID{f.folder}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.StartSession(context.Background(), f.user, p.ID)
	if err != nil || v.Current == nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventName != analytics.TrainingStarted {
		t.Fatal("missing start")
	}
	events = nil
	f.exec(t, `CREATE FUNCTION reject_analytics_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test commit failure'; END $$`)
	f.exec(t, `CREATE CONSTRAINT TRIGGER reject_analytics_commit AFTER INSERT ON training_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_analytics_commit()`)
	req := actionFor(v.Current, algorithm.Correct)
	if _, err = s.Act(context.Background(), f.user, v.Session.ID, req); err == nil {
		t.Fatal("expected deferred commit failure")
	}
	if len(events) != 0 {
		t.Fatal("analytics escaped rolled back transaction")
	}
	f.exec(t, `DROP TRIGGER reject_analytics_commit ON training_events`)
	out, err := s.Act(context.Background(), f.user, v.Session.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventID != out.Event.ID.String() || events[0].Template != "custom_origin" || events[0].StageBefore == nil {
		t.Fatalf("missing rich committed event: %+v", events)
	}
	if _, err = s.Act(context.Background(), f.user, v.Session.ID, req); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatal("receipt replay published duplicate")
	}
}
