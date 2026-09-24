package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

type ClickHouseConfig struct{ Address, Database, User, Password string }
type ClickHouse struct {
	endpoint       string
	user, password string
	client         *http.Client
	transport      *http.Transport
}

func NewClickHouse(cfg ClickHouseConfig) (*ClickHouse, error) {
	u, err := url.Parse(cfg.Address)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || !regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,63}$`).MatchString(cfg.Database) {
		return nil, errors.New("invalid ClickHouse endpoint or database configuration")
	}
	q := u.Query()
	q.Set("database", cfg.Database)
	q.Set("query", "INSERT INTO analytics_events FORMAT JSONEachRow")
	q.Set("date_time_input_format", "best_effort")
	q.Set("wait_end_of_query", "1")
	u.RawQuery = q.Encode()
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxConnsPerHost = 1
	t.ResponseHeaderTimeout = 5 * time.Second
	return &ClickHouse{endpoint: u.String(), user: cfg.User, password: cfg.Password, transport: t, client: &http.Client{Transport: t, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *ClickHouse) Write(ctx context.Context, events []Event) error {
	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	for _, e := range events {
		if err := enc.Encode(e); err != nil {
			return errors.New("encode analytics batch")
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &body)
	if err != nil {
		return errors.New("create analytics request")
	}
	req.SetBasicAuth(c.user, c.password)
	req.Header.Set("Content-Type", "application/x-ndjson")
	res, err := c.client.Do(req)
	if err != nil {
		return errors.New("analytics transport failed")
	}
	defer res.Body.Close()
	// Do not retain or log server responses: they can echo inserted data.
	n, readErr := io.Copy(io.Discard, io.LimitReader(res.Body, 64*1024))
	// Successful synchronous INSERTs have no response body. Also catch errors
	// emitted after an HTTP 200 header without retaining their sensitive text.
	if res.StatusCode != http.StatusOK || readErr != nil || n != 0 {
		return errors.New("analytics insert failed")
	}
	return nil
}
func (c *ClickHouse) Close() error { c.transport.CloseIdleConnections(); return nil }
