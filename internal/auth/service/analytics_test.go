package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
)

type authCapture struct{ events []analytics.Event }

func (p *authCapture) Publish(_ context.Context, e analytics.Event) { p.events = append(p.events, e) }
func TestLoginAnalyticsIsAnonymousAndNotInfrastructureFailure(t *testing.T) {
	p := &authCapture{}
	s := &Service{}
	s.SetPublisher(p)
	_, err := s.Login(context.Background(), LoginInput{Nickname: "secret-nickname"})
	if !errors.Is(err, ErrInvalidCredentials) || len(p.events) != 1 || p.events[0].UserID != "" || p.events[0].EventName != analytics.LoginFailed {
		t.Fatal("bad failure event")
	}
	raw, _ := json.Marshal(p.events)
	if strings.Contains(string(raw), "secret-nickname") {
		t.Fatal("identifier leaked")
	}
	s.users = &loginUserRepositoryFake{getByNicknameFunc: func(context.Context, string) (model.User, error) { return model.User{}, errors.New("database failed") }}
	_, _ = s.Login(context.Background(), LoginInput{Nickname: "secret-nickname", Password: "secret-password"})
	if len(p.events) != 1 {
		t.Fatal("DB error counted as failed credentials")
	}
}
func TestPanicDoesNotEmitSuccessfulLogin(t *testing.T) {
	p := &authCapture{}
	s := &Service{}
	s.SetPublisher(p)
	func() {
		defer func() { _ = recover() }()
		_, _ = s.Login(context.Background(), LoginInput{Nickname: "name", Password: "password"})
	}()
	if len(p.events) != 0 {
		t.Fatal("panic emitted successful login")
	}
}
