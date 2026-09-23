package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"testing"
)

func TestConnectionErrorsPreserveCauseWithoutSecrets(t *testing.T) {
	secret := "do-not-log-this-password"
	_, parseErr := pgconn.ParseConfig("postgres://user:" + secret + "@localhost:invalid/db")
	if parseErr == nil {
		t.Fatal("expected bad DSN")
	}
	pg := &pgconn.PgError{Code: "28P01", Message: secret, Detail: secret}
	for _, cause := range []error{parseErr, pg, errors.New(secret), context.DeadlineExceeded, context.Canceled} {
		err := fmt.Errorf("open PostgreSQL: %w", SafeConnectionError(cause))
		if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "postgres://") {
			t.Fatal("connection details leaked")
		}
		if !errors.Is(err, cause) {
			t.Fatal("lost cause")
		}
	}
	var original *pgconn.PgError
	if !errors.As(SafeConnectionError(pg), &original) || original != pg {
		t.Fatal("lost driver type")
	}
	if !strings.Contains(SafeConnectionError(pg).Error(), "28P01") {
		t.Fatal("lost safe SQLSTATE")
	}
	if SafeConnectionError(nil) != nil {
		t.Fatal("nil error changed")
	}
}
