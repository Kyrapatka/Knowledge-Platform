package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

const (
	maxOpenConnections = 20
	maxIdleConnections = 10
	connectionLifetime = 30 * time.Minute
	connectionTimeout  = 5 * time.Second
)

type Postgres struct {
	GORM *gorm.DB
	SQL  *sql.DB
}

func OpenPostgres(databaseURL string) (*Postgres, error) {
	gormDB, err := gorm.Open(
		postgres.Open(databaseURL),
		&gorm.Config{
			TranslateError: true,
			// Default GORM error logs interpolate SQL parameters, including auth
			// hashes/tokens. Return errors to the boundary; do not log them twice.
			Logger: gormlog.Discard,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL connection: %w", SafeConnectionError(err))
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying SQL database: %w", err)
	}

	sqlDB.SetMaxOpenConns(maxOpenConnections)
	sqlDB.SetMaxIdleConns(maxIdleConnections)
	sqlDB.SetConnMaxLifetime(connectionLifetime)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		connectionTimeout,
	)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()

		return nil, fmt.Errorf("ping PostgreSQL: %w", SafeConnectionError(err))
	}

	return &Postgres{
		GORM: gormDB,
		SQL:  sqlDB,
	}, nil
}

func (p *Postgres) Close() error {
	if p == nil || p.SQL == nil {
		return nil
	}

	return p.SQL.Close()
}
