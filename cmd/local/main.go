// local manages only this Compose project's local application images and readiness.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type snapshot struct {
	Image   string `json:"image"`
	Version string `json:"version"`
	Schema  string `json:"schema"`
}
type deployment struct {
	Current  *snapshot `json:"current,omitempty"`
	Previous *snapshot `json:"previous,omitempty"`
	Pending  *snapshot `json:"pending,omitempty"`
	APIOnly  bool      `json:"api_only"`
}
type manager struct {
	project, image, statePath, overridePath string
	state                                   deployment
	ports                                   map[string]string
	run                                     func(bool, ...string) (string, error)
	ready                                   func(string) bool
	timeout                                 time.Duration
}

func command(stream bool, args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	if stream {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return "", cmd.Run()
	}
	// Captured config can include secrets. Never include its output in errors.
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s failed: %w", strings.Join(args[:min(len(args), 3)], " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Local workflow failed:", err)
		os.Exit(1)
	}
}
func run() error {
	_ = godotenv.Load()
	if len(os.Args) != 2 {
		return errors.New("usage: go run ./cmd/local [deploy|rollback|status|observability|down|check|full-check]")
	}
	m := &manager{run: command, timeout: 2 * time.Minute, ready: httpReady, ports: map[string]string{}}
	if value := os.Getenv("LOCAL_READY_TIMEOUT"); value != "" {
		d, err := time.ParseDuration(value)
		if err != nil || d <= 0 {
			return errors.New("invalid LOCAL_READY_TIMEOUT")
		}
		m.timeout = d
	}
	if _, err := m.run(false, "docker", "info", "--format", "{{.ServerVersion}}"); err != nil {
		return errors.New("Docker daemon unavailable. Start Docker Engine/Desktop yourself, then retry; no services were changed")
	}
	raw, err := m.run(false, "docker", "compose", "config", "--format", "json")
	if err != nil {
		return err
	}
	var cfg struct {
		Name     string
		Services map[string]struct {
			Ports []struct {
				Target    int
				Published string
			}
		}
	}
	if err = json.Unmarshal([]byte(raw), &cfg); err != nil {
		return errors.New("cannot parse Compose configuration")
	}
	m.project = cfg.Name
	m.image = cfg.Name + "-backend"
	for name, service := range cfg.Services {
		if len(service.Ports) > 0 {
			m.ports[name] = service.Ports[0].Published
		}
	}
	m.statePath = filepath.Join(".cache", "deployment-"+m.project+".json")
	m.overridePath = filepath.Join(".cache", "runtime-"+m.project+".json")
	if raw, err := os.ReadFile(m.statePath); err == nil {
		if err = json.Unmarshal(raw, &m.state); err != nil {
			return fmt.Errorf("invalid deployment state %s; inspect it before continuing", m.statePath)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	action := os.Args[1]
	if action == "status" {
		return m.status()
	}
	if action == "check" || action == "full-check" {
		return m.check(action == "full-check")
	}
	if err = os.MkdirAll(".cache", 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(m.statePath+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("deployment lock exists or cannot be created: %s.lock; check for another running workflow before removing a stale lock", m.statePath)
	}
	defer os.Remove(lock.Name())
	defer lock.Close()
	switch action {
	case "deploy":
		return m.deploy()
	case "rollback":
		return m.rollback()
	case "observability":
		return m.observability()
	case "down":
		_, err = m.compose(true, false, "down")
		return err
	default:
		return errors.New("unknown local workflow")
	}
}
func httpReady(url string) bool {
	client := http.Client{Timeout: 3 * time.Second}
	res, err := client.Get(url)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK
}
func (m *manager) url(service, path string) string {
	return "http://127.0.0.1:" + m.ports[service] + path
}
func (m *manager) compose(stream, override bool, args ...string) (string, error) {
	all := []string{"docker", "compose", "-p", m.project, "-f", "compose.yaml"}
	if override {
		all = append(all, "-f", m.overridePath)
	}
	return m.run(stream, append(all, args...)...)
}
func (m *manager) writeOverride(image string, apiOnly bool) error {
	backend := map[string]any{"image": image}
	if version, err := m.run(false, "docker", "image", "inspect", "--format", "{{index .Config.Labels \"org.opencontainers.image.version\"}}", image); err == nil && version != "" && version != "<no value>" {
		backend["environment"] = map[string]string{"APP_VERSION": version}
	}
	if apiOnly {
		backend["command"] = []string{"./api"}
	}
	data, _ := json.Marshal(map[string]any{"services": map[string]any{"backend": backend}})
	return os.WriteFile(m.overridePath, data, 0600)
}
func (m *manager) save() error {
	raw, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return err
	}
	temp := m.statePath + ".tmp"
	if err = os.WriteFile(temp, append(raw, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(temp, m.statePath)
}
func (m *manager) container() (string, error) { return m.compose(false, false, "ps", "-q", "backend") }
func (m *manager) inspect(container, format string) (string, error) {
	return m.run(false, "docker", "inspect", "--format", format, container)
}
func (m *manager) schema() (string, error) {
	value, err := m.compose(false, false, "exec", "-T", "postgres", "psql", "-U", "knowledge", "-d", "knowledge_platform", "-Atc", "SELECT version::text || ':' || dirty::text FROM public.schema_migrations")
	if err != nil {
		return "", errors.New("cannot read database migration version; inspect make migrate-status / docs/migrations.md")
	}
	if !strings.HasSuffix(value, ":false") || strings.Contains(value, "\n") {
		return "", errors.New("database migration state is missing or dirty; inspect docs/migrations.md")
	}
	return value, nil
}
func (m *manager) runningSnapshot() (*snapshot, error) {
	id, err := m.container()
	if err != nil || id == "" {
		return nil, nil
	}
	health, err := m.inspect(id, "{{if .State.Health}}{{.State.Health.Status}}{{end}}")
	if err != nil || health != "healthy" || !m.ready(m.url("backend", "/ready")) {
		return nil, nil
	}
	image, err := m.inspect(id, "{{.Image}}")
	if err != nil {
		return nil, err
	}
	version, err := m.run(false, "docker", "image", "inspect", "--format", "{{index .Config.Labels \"org.opencontainers.image.version\"}}", image)
	if err != nil {
		return nil, err
	}
	schema, err := m.schema()
	if err != nil {
		return nil, err
	}
	return &snapshot{Image: image, Version: version, Schema: schema}, nil
}
func (m *manager) waitReady(image string) error {
	deadline := time.Now().Add(m.timeout)
	for {
		id, err := m.container()
		if err == nil && id != "" {
			actual, _ := m.inspect(id, "{{.Image}}")
			status, _ := m.inspect(id, "{{.State.Status}}")
			if status == "exited" || status == "dead" {
				break
			}
			// Never mistake an old process or another listener for the new deployment.
			if actual == image && m.ready(m.url("backend", "/ready")) {
				return nil
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Second)
	}
	return errors.New("backend did not become ready; run docker compose ps -a and docker compose logs --tail=100 backend; make rollback-local remains available when a previous image exists")
}
func (m *manager) deploy() error {
	previous, err := m.runningSnapshot()
	if err != nil {
		return err
	}
	if previous != nil {
		if _, err = m.run(false, "docker", "image", "tag", previous.Image, m.image+":previous"); err != nil {
			return err
		}
		m.state.Previous = previous
		m.state.Current = previous
		if err = m.save(); err != nil {
			return err
		}
	}
	version := os.Getenv("LOCAL_VERSION")
	if version == "" {
		revision, _ := m.run(false, "git", "rev-parse", "--short", "HEAD")
		version = time.Now().UTC().Format("20060102T150405Z") + "-" + revision
	}
	fmt.Println("Building local application", version, "(no tests)")
	if _, err = m.run(true, "docker", "build", "--build-arg", "APP_VERSION="+version, "-t", m.image+":candidate", "."); err != nil {
		return fmt.Errorf("image build failed; running backend unchanged: %w", err)
	}
	image, err := m.run(false, "docker", "image", "inspect", "--format", "{{.Id}}", m.image+":candidate")
	if err != nil {
		return err
	}
	m.state.Pending = &snapshot{Image: image, Version: version}
	if err = m.save(); err != nil {
		return err
	}
	if _, err = m.run(false, "docker", "image", "tag", image, m.image+":current"); err != nil {
		return err
	}
	if err = m.writeOverride(m.image+":current", false); err != nil {
		return err
	}
	// The image CMD runs migrate once and starts API only if migration succeeds.
	if _, err = m.compose(true, true, "up", "-d", "--no-build"); err != nil {
		return err
	}
	if err = m.waitReady(image); err != nil {
		return err
	}
	schema, err := m.schema()
	if err != nil {
		return err
	}
	m.state.Current = &snapshot{Image: image, Version: version, Schema: schema}
	m.state.Pending = nil
	m.state.APIOnly = false
	if err = m.save(); err != nil {
		return err
	}
	fmt.Println("Deployment ready:", version, image)
	m.urls()
	return nil
}
func rollbackCompatible(previous *snapshot, currentSchema string) error {
	if previous == nil || previous.Image == "" {
		return errors.New("no previous working image recorded; complete a deploy before rollback")
	}
	if previous.Schema == "" || previous.Schema != currentSchema {
		return fmt.Errorf("application rollback refused: current schema %s differs from previous %s; no schema downgrade was attempted. Review docs/migrations.md and docs/local-deployment.md", currentSchema, previous.Schema)
	}
	return nil
}
func (m *manager) rollback() error {
	schema, err := m.schema()
	if err != nil {
		return err
	}
	if err = rollbackCompatible(m.state.Previous, schema); err != nil {
		return err
	}
	previous := *m.state.Previous
	if _, err = m.run(false, "docker", "image", "inspect", "--format", "{{.Id}}", previous.Image); err != nil {
		return errors.New("previous image missing; do not prune local current/previous images")
	}
	if _, err = m.run(false, "docker", "image", "tag", previous.Image, m.image+":current"); err != nil {
		return err
	}
	if err = m.writeOverride(m.image+":current", true); err != nil {
		return err
	}
	// Explicit API-only command prevents an older migration runner touching the DB.
	if _, err = m.compose(true, true, "up", "-d", "--no-build", "--no-deps", "--force-recreate", "backend"); err != nil {
		return err
	}
	if err = m.waitReady(previous.Image); err != nil {
		return err
	}
	m.state.Current = &previous
	m.state.Pending = nil
	m.state.APIOnly = true
	if err = m.save(); err != nil {
		return err
	}
	fmt.Println("Application rollback ready:", previous.Version, previous.Image, "; database unchanged")
	m.urls()
	return nil
}
func (m *manager) observability() error {
	if m.state.Pending != nil {
		return errors.New("unfinished deployment recorded; run make rollback-local or retry make deploy-local before starting observability")
	}
	image, err := m.run(false, "docker", "image", "inspect", "--format", "{{.Id}}", m.image+":current")
	if err != nil {
		if err = m.writeOverride(m.image+":current", false); err != nil {
			return err
		}
		if _, err = m.compose(true, true, "build", "backend"); err != nil {
			return err
		}
		image, err = m.run(false, "docker", "image", "inspect", "--format", "{{.Id}}", m.image+":current")
		if err != nil {
			return err
		}
	}
	if err = m.writeOverride(m.image+":current", m.state.APIOnly); err != nil {
		return err
	}
	if _, err = m.compose(true, true, "up", "-d", "--no-build"); err != nil {
		return err
	}
	if err = m.waitReady(image); err != nil {
		return err
	}
	deadline := time.Now().Add(m.timeout)
	for {
		id, _ := m.compose(false, false, "ps", "-aq", "clickhouse-init")
		status := ""
		if id != "" {
			status, _ = m.inspect(id, "{{.State.Status}}:{{.State.ExitCode}}")
		}
		if strings.HasPrefix(status, "exited:") && status != "exited:0" {
			return errors.New("ClickHouse initialization failed; run docker compose logs clickhouse-init")
		}
		if status == "exited:0" && m.ready(m.url("grafana", "/api/health")) && m.ready(m.url("prometheus", "/-/ready")) && m.ready(m.url("clickhouse", "/ping")) {
			break
		}
		if time.Now().After(deadline) {
			return errors.New("observability startup timed out; run docker compose ps -a and make observability-check")
		}
		time.Sleep(time.Second)
	}
	fmt.Println("Observability ready (no tests; use deploy-local to rebuild source)")
	m.urls()
	return nil
}
func (m *manager) urls() {
	for _, v := range [][3]string{{"Application", "backend", ""}, {"Liveness", "backend", "/live"}, {"Readiness", "backend", "/ready"}, {"Metrics", "backend", "/metrics"}, {"Swagger", "backend", "/swagger/"}, {"Grafana", "grafana", ""}, {"Prometheus", "prometheus", ""}} {
		fmt.Println(v[0]+":", m.url(v[1], v[2]))
	}
}
func (m *manager) status() error {
	if _, err := m.compose(true, false, "ps", "-a"); err != nil {
		return err
	}
	id, err := m.container()
	if err != nil || id == "" {
		return errors.New("backend is not running")
	}
	image, _ := m.inspect(id, "{{.Image}}")
	version, _ := m.run(false, "docker", "image", "inspect", "--format", "{{index .Config.Labels \"org.opencontainers.image.version\"}}", image)
	if version == "" || version == "<no value>" {
		version = "unknown (legacy image)"
	}
	fmt.Println("Backend image:", image, "version:", version)
	ok := true
	for _, path := range []string{"/live", "/ready"} {
		ready := m.ready(m.url("backend", path))
		fmt.Println(path, ready)
		ok = ok && ready
	}
	for _, v := range [][2]string{{"grafana", "/api/health"}, {"prometheus", "/-/ready"}} {
		fmt.Println(v[0], "reachable:", m.ready(m.url(v[0], v[1])))
	}
	if !ok {
		return errors.New("backend probes failed")
	}
	return nil
}
func (m *manager) check(full bool) error {
	python := os.Getenv("PYTHON")
	if python == "" {
		python = "python3"
		if runtime.GOOS == "windows" {
			python = "python"
		}
	}
	args := []string{python, "deploy/verify_observability.py", "--quick"}
	if full {
		args[2] = "--require-events"
	}
	if _, err := m.run(true, args...); err != nil {
		return err
	}
	if full {
		for _, script := range []string{"deploy/test_analytics_sql.py", "deploy/verify_analytics_resilience.py"} {
			if _, err := m.run(true, python, script); err != nil {
				return err
			}
		}
	}
	return nil
}
