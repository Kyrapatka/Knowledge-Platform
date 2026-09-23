package database

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
)

// SafeConnectionError keeps the original cause for errors.Is/As, but never
// renders a driver message: malformed DSNs can be echoed in full by pgx.
// This is only for infrastructure connection errors, not query/domain errors.
func SafeConnectionError(err error) error {
	if err == nil {
		return nil
	}
	return connectionError{cause: err}
}

type connectionError struct{ cause error }

func (e connectionError) Unwrap() error { return e.cause }
func (e connectionError) Error() string {
	if errors.Is(e.cause, context.DeadlineExceeded) {
		return "database connection timed out"
	}
	if errors.Is(e.cause, context.Canceled) {
		return "database connection canceled"
	}
	var parse *pgconn.ParseConfigError
	if errors.As(e.cause, &parse) {
		return "invalid database connection configuration"
	}
	var pg *pgconn.PgError
	if errors.As(e.cause, &pg) && len(pg.Code) == 5 {
		valid := true
		for _, r := range pg.Code {
			if !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z') {
				valid = false
				break
			}
		}
		if valid {
			return "database connection failed (SQLSTATE " + pg.Code + ")"
		}
	}
	return "database connection failed (driver details redacted)"
}
