package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testManager(t *testing.T) *manager {
	t.Helper()
	dir := t.TempDir()
	return &manager{project: "test", image: "test-backend", statePath: filepath.Join(dir, "state.json"), overridePath: filepath.Join(dir, "override.json"), timeout: time.Millisecond, ports: map[string]string{"backend": "8080"}, ready: func(string) bool { return true }}
}
func TestRollbackCompatibilityRefusesSchemaChanges(t *testing.T) {
	for _, previous := range []*snapshot{nil, {Image: "old", Schema: "22:false"}, {Image: "old", Schema: "23:true"}} {
		if rollbackCompatible(previous, "23:false") == nil {
			t.Fatal("accepted unavailable/incompatible previous image")
		}
	}
	if err := rollbackCompatible(&snapshot{Image: "old", Schema: "23:false"}, "23:false"); err != nil {
		t.Fatal(err)
	}
}
func TestRollbackRunsOnlyAPIAndKeepsDatabase(t *testing.T) {
	m := testManager(t)
	m.state.Previous = &snapshot{Image: "sha256:old", Version: "old", Schema: "23:false"}
	var commands []string
	m.run = func(_ bool, args ...string) (string, error) {
		c := strings.Join(args, " ")
		commands = append(commands, c)
		switch {
		case strings.Contains(c, "psql"):
			return "23:false", nil
		case strings.Contains(c, "ps -q backend"):
			return "container", nil
		case strings.Contains(c, "{{.Image}}"):
			return "sha256:old", nil
		case strings.Contains(c, "{{.State.Status}}"):
			return "running", nil
		}
		return "", nil
	}
	if err := m.rollback(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(m.overridePath)
	var override struct {
		Services map[string]struct{ Command []string }
	}
	if err := json.Unmarshal(raw, &override); err != nil {
		t.Fatal(err)
	}
	if got := override.Services["backend"].Command; len(got) != 1 || got[0] != "./api" {
		t.Fatalf("old migration runner enabled: %v", got)
	}
	for _, c := range commands {
		if strings.Contains(c, "migrate") || strings.Contains(c, " down") || strings.Contains(c, " build") {
			t.Fatal("rollback touched schema or rebuilt", c)
		}
	}
	if m.state.Current.Image != "sha256:old" || !m.state.APIOnly {
		t.Fatal("rollback not saved")
	}
}
func TestRollbackRefusesBeforeContainerMutation(t *testing.T) {
	m := testManager(t)
	m.state.Previous = &snapshot{Image: "old", Schema: "22:false"}
	m.run = func(_ bool, args ...string) (string, error) {
		if !strings.Contains(strings.Join(args, " "), "psql") {
			t.Fatal("mutated before compatibility check", args)
		}
		return "23:false", nil
	}
	if m.rollback() == nil {
		t.Fatal("expected incompatibility")
	}
}
func TestFailedBuildPreservesWorkingImage(t *testing.T) {
	m := testManager(t)
	m.run = func(_ bool, args ...string) (string, error) {
		c := strings.Join(args, " ")
		switch {
		case strings.Contains(c, "ps -q backend"):
			return "container", nil
		case strings.Contains(c, ".State.Health"):
			return "healthy", nil
		case strings.Contains(c, "{{.Image}}"):
			return "sha256:working", nil
		case strings.Contains(c, "psql"):
			return "23:false", nil
		case strings.HasPrefix(c, "docker build"):
			return "", errors.New("controlled build failure")
		case strings.Contains(c, "up -d") || strings.Contains(c, ":current"):
			t.Fatal("replaced running app after failed build", c)
		}
		return "version", nil
	}
	if m.deploy() == nil {
		t.Fatal("failed deploy returned success")
	}
	var saved deployment
	raw, _ := os.ReadFile(m.statePath)
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Previous == nil || saved.Previous.Image != "sha256:working" {
		t.Fatal("lost rollback image")
	}
}
func TestReadinessDoesNotAcceptAnotherImage(t *testing.T) {
	m := testManager(t)
	m.run = func(_ bool, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "{{.Image}}") {
			return "old", nil
		}
		return "running", nil
	}
	if m.waitReady("new") == nil {
		t.Fatal("accepted old process readiness")
	}
}

func TestObservabilityRefusesPendingDeploy(t *testing.T) {
	m := testManager(t)
	m.state.Pending = &snapshot{Image: "unverified"}
	m.run = func(_ bool, args ...string) (string, error) {
		t.Fatal("started service during unfinished deploy", args)
		return "", nil
	}
	if m.observability() == nil {
		t.Fatal("accepted unfinished deployment")
	}
}

func TestFailedReadinessPreservesRollbackState(t *testing.T) {
	m := testManager(t)
	m.state.Previous = &snapshot{Image: "sha256:working", Schema: "23:false"}
	m.ready = func(string) bool { return false }
	m.run = func(_ bool, args ...string) (string, error) {
		c := strings.Join(args, " ")
		switch {
		case strings.Contains(c, "ps -q backend"):
			return "candidate-container", nil
		case strings.Contains(c, "{{.State.Status}}"):
			return "exited", nil
		case strings.Contains(c, ".State.Health"):
			return "unhealthy", nil
		case strings.Contains(c, "{{.Id}}"), strings.Contains(c, "{{.Image}}"):
			return "sha256:candidate", nil
		}
		return "", nil
	}
	if m.deploy() == nil {
		t.Fatal("accepted failed readiness")
	}
	var saved deployment
	raw, err := os.ReadFile(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Previous == nil || saved.Previous.Image != "sha256:working" || saved.Pending == nil {
		t.Fatal("readiness failure lost previous or pending state")
	}
}
