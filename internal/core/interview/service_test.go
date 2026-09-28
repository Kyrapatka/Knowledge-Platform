package interview

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type profileMemory struct {
	Transaction
	template  string
	values    map[string]*string
	old       Profile
	written   *Profile
	committed bool
}

func (m *profileMemory) Transact(_ context.Context, _ uuid.UUID, fn func(Transaction) error) error {
	err := fn(m)
	m.committed = err == nil
	return err
}
func (m *profileMemory) FolderTemplate(uuid.UUID) (string, error) { return m.template, nil }
func (m *profileMemory) MaterialValues(uuid.UUID, uuid.UUID) (map[string]*string, error) {
	return m.values, nil
}
func (m *profileMemory) Profile(uuid.UUID) (Profile, error) { return m.old, nil }
func (m *profileMemory) PutProfile(p Profile) error         { m.written = &p; return nil }

func TestServiceReadyContentAndVersion(t *testing.T) {
	question, answer, placeholder := "Question", "Answer", "."
	for _, tt := range []struct {
		name   string
		values map[string]*string
		valid  bool
	}{
		{"missing question", map[string]*string{"answer": &answer}, false},
		{"placeholder answer", map[string]*string{"question": &question, "answer": &placeholder}, false},
		{"short answer", map[string]*string{"question": &question, "short_answer": &answer}, true},
		{"full answer", map[string]*string{"question": &question, "answer": &answer}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := &profileMemory{template: "interview_questions", values: tt.values}
			p := Profile{Frequency: 5, FrequencyConfidence: .5, InterviewDifficulty: 2, Specificity: 2, RootWeight: 5, Status: "ready", Concepts: []QuestionConcept{{Slug: "go", Role: "primary"}, {Slug: "go", Role: "tested"}}}
			out, err := NewService(m).SaveProfile(context.Background(), uuid.New(), uuid.New(), uuid.New(), ProfileRequest{Profile: p})
			if tt.valid {
				if err != nil || !m.committed || m.written == nil || out.ProfileVersion != 1 {
					t.Fatalf("save: %+v %v", out, err)
				}
			} else if !errors.Is(err, ErrInvalid) || m.committed || m.written != nil {
				t.Fatalf("invalid profile persisted: %v", err)
			}
		})
	}
}
func TestServiceRejectsStaleProfileAndWrongFolder(t *testing.T) {
	m := &profileMemory{template: "interview_questions", old: Profile{ProfileVersion: 2}}
	p := Profile{Status: "draft", Frequency: 5, InterviewDifficulty: 2, Specificity: 2}
	s := NewService(m)
	_, err := s.SaveProfile(context.Background(), uuid.New(), uuid.New(), uuid.New(), ProfileRequest{Profile: p, ExpectedVersion: 1})
	if !errors.Is(err, ErrConflict) || m.written != nil {
		t.Fatalf("stale save: %v", err)
	}
	m.template = "english_words"
	_, err = s.SaveProfile(context.Background(), uuid.New(), uuid.New(), uuid.New(), ProfileRequest{Profile: p, ExpectedVersion: 2})
	if !errors.Is(err, ErrInvalid) || m.written != nil {
		t.Fatalf("wrong folder: %v", err)
	}
}
