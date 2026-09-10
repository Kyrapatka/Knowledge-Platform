// migrate applies pending upward migrations using DATABASE_URL from .env.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	_ = godotenv.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer conn.Close(ctx)
	// Hold a session-level lock for this runner; individual migrations are atomic.
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(734291065)"); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock(734291065)")
	if _, err = conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL)"); err != nil {
		return err
	}
	var version int64
	var dirty bool
	err = conn.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations LIMIT 1").Scan(&version, &dirty)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if dirty {
		return fmt.Errorf("migration %d is dirty; inspect the database before continuing", version)
	}
	files, err := filepath.Glob("migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no migrations found; run from the repository root")
	}
	sort.Strings(files)
	for _, file := range files {
		number, err := strconv.ParseInt(strings.Split(filepath.Base(file), "_")[0], 10, 64)
		if err != nil {
			return err
		}
		if number <= version {
			continue
		}
		if number != version+1 {
			return fmt.Errorf("expected migration %d, found %d", version+1, number)
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(content)); err == nil {
			_, err = tx.Exec(ctx, "DELETE FROM schema_migrations")
		}
		if err == nil {
			_, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version,dirty) VALUES($1,false)", number)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s rolled back: %w", file, err)
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		version = number
		fmt.Printf("Applied %s\n", filepath.Base(file))
	}
	fmt.Printf("Database is ready at migration %d.\n", version)
	return nil
}
