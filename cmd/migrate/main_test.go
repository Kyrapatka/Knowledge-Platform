package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestMigrationPairs(t *testing.T) {
	files, err := migrationFiles("../../migrations")
	if err != nil || len(files) != 26 {
		t.Fatalf("pairs: %d %v", len(files), err)
	}
}
func TestMigrationsRefuseLossyRollback(t *testing.T) {
	dsn := os.Getenv("TRAINING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TRAINING_TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("connect test PostgreSQL failed")
	}
	defer conn.Close(ctx)
	schema := pgx.Identifier{"migration_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")}.Sanitize()
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	if _, err = conn.Exec(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	if err = migrate(ctx, conn, "../../migrations", "up"); err != nil {
		t.Fatal(err)
	}
	user := uuid.New()
	if _, err = conn.Exec(ctx, "INSERT INTO users(id,nickname,nickname_normalized,password_hash) VALUES($1,'smoke','smoke','hash')", user); err != nil {
		t.Fatal(err)
	}
	command := uuid.New()
	if _, err = conn.Exec(ctx, "INSERT INTO material_import_commands(user_id,command_id,request_hash,response) VALUES($1,$2,'hash','{}')", user, command); err != nil {
		t.Fatal(err)
	}
	if err = migrate(ctx, conn, "../../migrations", "down"); err == nil {
		t.Fatal("removed import receipt")
	}
	var importVersion int
	if err = conn.QueryRow(ctx, "SELECT version FROM schema_migrations").Scan(&importVersion); err != nil || importVersion != 26 {
		t.Fatal("receipt rollback changed version", importVersion, err)
	}
	if _, err = conn.Exec(ctx, "DELETE FROM material_import_commands WHERE user_id=$1", user); err != nil {
		t.Fatal(err)
	}
	if err = migrate(ctx, conn, "../../migrations", "down"); err != nil {
		t.Fatal(err)
	}
	preset := uuid.New()
	if _, err = conn.Exec(ctx, "INSERT INTO mock_presets(id,user_id,name,created_at,updated_at) VALUES($1,$2,'Keep me',now(),now())", preset, user); err != nil {
		t.Fatal(err)
	}
	if err = migrate(ctx, conn, "../../migrations", "down"); err == nil {
		t.Fatal("removed saved preset")
	}
	var current int
	if err = conn.QueryRow(ctx, "SELECT version FROM schema_migrations").Scan(&current); err != nil || current != 25 {
		t.Fatal("failed preset rollback changed version", current, err)
	}
	var name string
	if err = conn.QueryRow(ctx, "SELECT name FROM mock_presets WHERE id=$1", preset).Scan(&name); err != nil || name != "Keep me" {
		t.Fatal("preset lost", err)
	}
	if _, err = conn.Exec(ctx, "DELETE FROM mock_presets WHERE id=$1", preset); err != nil {
		t.Fatal(err)
	}
	// Remove the newer empty tables before exercising migration 23's guard.
	for i := 0; i < 2; i++ {
		if err = migrate(ctx, conn, "../../migrations", "down"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = conn.Exec(ctx, "INSERT INTO training_sessions(id,user_id,status,started_at,created_at,selection_strategy) VALUES($1,$2,'active',now(),now(),'interview_graph_v1')", uuid.New(), user); err != nil {
		t.Fatal(err)
	}
	if err = migrate(ctx, conn, "../../migrations", "down"); err == nil {
		t.Fatal("removed planless history")
	}
	var version int
	if err = conn.QueryRow(ctx, "SELECT version FROM schema_migrations").Scan(&version); err != nil || version != 23 {
		t.Fatal("failed down changed version", err, version)
	}
	if _, err = conn.Exec(ctx, "INSERT INTO interview_bank_alias_policies(owner_id,seed_revision,scoped_aliases,ambiguous_aliases) VALUES($1,'test','[]','[]')", user); err != nil {
		t.Fatal(err)
	}
	if err = apply(ctx, conn, filepath.Join("../../migrations", "000021_interview_question_bank.down.sql"), 20); err == nil {
		t.Fatal("removed bank metadata")
	}
	var count int
	if err = conn.QueryRow(ctx, "SELECT count(*) FROM interview_bank_alias_policies").Scan(&count); err != nil || count != 1 {
		t.Fatal("failed rollback lost data", err)
	}
}
