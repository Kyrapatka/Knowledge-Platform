// migrate runs atomic migrations; smoke uses a disposable schema in a test database.
package main

import (
	"context"
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/database"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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
	action := "up"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}
	if len(os.Args) > 2 || (action != "up" && action != "down" && action != "status" && action != "smoke") {
		return fmt.Errorf("usage: migrate [up|down|status|smoke]; down rolls back exactly one migration")
	}
	key := "DATABASE_URL"
	if action == "smoke" {
		key = "TRAINING_TEST_DATABASE_URL"
	}
	dsn := os.Getenv(key)
	if dsn == "" {
		return fmt.Errorf("%s is required", key)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect to database: %w", database.SafeConnectionError(err))
	}
	defer conn.Close(ctx)
	if action == "smoke" {
		return smoke(ctx, conn, "migrations")
	}
	return migrate(ctx, conn, "migrations", action)
}

func smoke(ctx context.Context, conn *pgx.Conn, directory string) error {
	// Only the fresh, random schema created here is ever removed.
	schema := "migration_smoke_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := conn.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			log.Printf("migration smoke schema cleanup failed")
		}
	}()
	if _, err := conn.Exec(ctx, "SET search_path TO "+quoted); err != nil {
		return err
	}
	if err := migrate(ctx, conn, directory, "up"); err != nil {
		return err
	}
	// Latest (23) is conditionally reversible: the freshly migrated schema is empty.
	// Review this allowlist when adding a migration instead of assuming every down is safe.
	var version int64
	if err := conn.QueryRow(ctx, "SELECT version FROM schema_migrations").Scan(&version); err != nil {
		return err
	}
	if version == 23 {
		if err := migrate(ctx, conn, directory, "down"); err != nil {
			return err
		}
		if err := migrate(ctx, conn, directory, "up"); err != nil {
			return err
		}
	}
	fmt.Println("Migration smoke passed in an isolated empty schema.")
	return nil
}

func migrationFiles(directory string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(directory, "*.up.sql"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no migrations found; run from repository root")
	}
	sort.Strings(files)
	for i, file := range files {
		n, err := strconv.Atoi(strings.Split(filepath.Base(file), "_")[0])
		if err != nil || n != i+1 {
			return nil, fmt.Errorf("non-contiguous migration sequence at %s", file)
		}
		if _, err := os.Stat(strings.TrimSuffix(file, ".up.sql") + ".down.sql"); err != nil {
			return nil, fmt.Errorf("missing down pair: %s", file)
		}
	}
	return files, nil
}

func migrate(ctx context.Context, conn *pgx.Conn, directory, action string) error {
	files, err := migrationFiles(directory)
	if err != nil {
		return err
	}
	// The advisory lock serializes runners; DDL and version changes commit together.
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(734291065)"); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock(734291065)")
	if _, err = conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL)"); err != nil {
		return err
	}
	var version int64
	var dirty bool
	err = conn.QueryRow(ctx, "SELECT version,dirty FROM schema_migrations LIMIT 1").Scan(&version, &dirty)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if dirty {
		return fmt.Errorf("migration %d is dirty; inspect the database before continuing", version)
	}
	if version < 0 || version > int64(len(files)) {
		return fmt.Errorf("database version %d is outside this checkout's migrations", version)
	}
	if action == "status" {
		fmt.Printf("Database migration: %d; available: %d; dirty: false.\n", version, len(files))
		return nil
	}
	if action == "down" {
		if version == 0 {
			return fmt.Errorf("database has no migration to roll back")
		}
		return apply(ctx, conn, strings.TrimSuffix(files[version-1], ".up.sql")+".down.sql", version-1)
	}
	for i := int(version); i < len(files); i++ {
		if err := apply(ctx, conn, files[i], int64(i+1)); err != nil {
			return err
		}
	}
	fmt.Printf("Database is ready at migration %d.\n", len(files))
	return nil
}
func apply(ctx context.Context, conn *pgx.Conn, file string, version int64) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, string(content)); err == nil {
		_, err = tx.Exec(ctx, "DELETE FROM schema_migrations")
	}
	if err == nil && version > 0 {
		_, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version,dirty) VALUES($1,false)", version)
	}
	if err != nil {
		return fmt.Errorf("migration %s rolled back: %w", filepath.Base(file), err)
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Printf("Applied %s\n", filepath.Base(file))
	return nil
}
