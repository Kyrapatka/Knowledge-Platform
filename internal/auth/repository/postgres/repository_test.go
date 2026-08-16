package postgres_test

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	autherrors "github.com/Kyrapatka/knowledge-platform/internal/auth"
	authmodel "github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	authrepo "github.com/Kyrapatka/knowledge-platform/internal/auth/repository"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	if err := loadTestEnvironment(); err != nil {
		_, _ = os.Stderr.WriteString(
			"load test environment: " + err.Error() + "\n",
		)

		os.Exit(1)
	}

	os.Exit(m.Run())
}

func loadTestEnvironment() error {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New(
			"determine repository test file path",
		)
	}

	projectRoot := filepath.Clean(
		filepath.Join(
			filepath.Dir(currentFile),
			"../../../..",
		),
	)

	envPath := filepath.Join(projectRoot, ".env")

	if err := godotenv.Load(envPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	return nil
}

func TestUserRepository(t *testing.T) {
	db := prepareTestDatabase(t)

	userRepository := postgres.NewUserRepository(db)
	ctx := context.Background()

	params := authrepo.CreateUserParams{
		ID:                 uuid.New(),
		Nickname:           "Nikita",
		NicknameNormalized: "nikita",
		PasswordHash:       "test-password-hash",
	}

	createdUser, err := userRepository.Create(ctx, params)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if createdUser.ID != params.ID {
		t.Fatalf(
			"unexpected user ID: got %s, want %s",
			createdUser.ID,
			params.ID,
		)
	}

	if createdUser.Nickname != params.Nickname {
		t.Fatalf(
			"unexpected nickname: got %q, want %q",
			createdUser.Nickname,
			params.Nickname,
		)
	}

	if createdUser.NicknameNormalized != params.NicknameNormalized {
		t.Fatalf(
			"unexpected normalized nickname: got %q, want %q",
			createdUser.NicknameNormalized,
			params.NicknameNormalized,
		)
	}

	if createdUser.PasswordHash != params.PasswordHash {
		t.Fatal("unexpected password hash")
	}

	if createdUser.Status != authmodel.UserStatusActive {
		t.Fatalf(
			"unexpected status: got %q, want %q",
			createdUser.Status,
			authmodel.UserStatusActive,
		)
	}

	if createdUser.CreatedAt.IsZero() {
		t.Fatal("created_at must not be zero")
	}

	userByID, err := userRepository.GetByID(
		ctx,
		createdUser.ID,
	)
	if err != nil {
		t.Fatalf("get user by ID: %v", err)
	}

	if userByID.ID != createdUser.ID {
		t.Fatalf(
			"unexpected user ID: got %s, want %s",
			userByID.ID,
			createdUser.ID,
		)
	}

	userByNickname, err := userRepository.GetByNickname(
		ctx,
		createdUser.NicknameNormalized,
	)
	if err != nil {
		t.Fatalf("get user by nickname: %v", err)
	}

	if userByNickname.ID != createdUser.ID {
		t.Fatalf(
			"unexpected user ID: got %s, want %s",
			userByNickname.ID,
			createdUser.ID,
		)
	}

	_, err = userRepository.Create(
		ctx,
		authrepo.CreateUserParams{
			ID:                 uuid.New(),
			Nickname:           "NIKITA",
			NicknameNormalized: "nikita",
			PasswordHash:       "another-password-hash",
		},
	)
	if !errors.Is(err, autherrors.ErrNicknameTaken) {
		t.Fatalf(
			"expected ErrNicknameTaken, got %v",
			err,
		)
	}

	_, err = userRepository.GetByID(ctx, uuid.New())
	if !errors.Is(err, autherrors.ErrUserNotFound) {
		t.Fatalf(
			"expected ErrUserNotFound, got %v",
			err,
		)
	}

	_, err = userRepository.GetByNickname(
		ctx,
		"unknown-user",
	)
	if !errors.Is(err, autherrors.ErrUserNotFound) {
		t.Fatalf(
			"expected ErrUserNotFound, got %v",
			err,
		)
	}
}

func TestSessionRepository_Lifecycle(t *testing.T) {
	db := prepareTestDatabase(t)

	userRepository := postgres.NewUserRepository(db)
	sessionRepository := postgres.NewSessionRepository(db)

	ctx := context.Background()

	user := createTestUser(
		t,
		ctx,
		userRepository,
	)

	ipAddress := netip.MustParseAddr("127.0.0.1")
	now := time.Now().UTC().Truncate(time.Microsecond)

	params := authrepo.CreateSessionParams{
		ID:               uuid.New(),
		UserID:           user.ID,
		RefreshTokenHash: "refresh-token-hash",
		UserAgent:        "integration-test",
		IPAddress:        &ipAddress,
		CreatedAt:        now,
		ExpiresAt:        now.Add(24 * time.Hour),
		LastUsedAt:       now,
	}

	createdSession, err := sessionRepository.Create(
		ctx,
		params,
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if createdSession.ID != params.ID {
		t.Fatalf(
			"unexpected session ID: got %s, want %s",
			createdSession.ID,
			params.ID,
		)
	}

	if createdSession.UserID != user.ID {
		t.Fatalf(
			"unexpected user ID: got %s, want %s",
			createdSession.UserID,
			user.ID,
		)
	}

	if createdSession.IPAddress == nil {
		t.Fatal("IP address must not be nil")
	}

	if *createdSession.IPAddress != ipAddress {
		t.Fatalf(
			"unexpected IP address: got %s, want %s",
			createdSession.IPAddress,
			ipAddress,
		)
	}

	storedSession, err := sessionRepository.GetByTokenHash(
		ctx,
		params.RefreshTokenHash,
	)
	if err != nil {
		t.Fatalf("get session by token hash: %v", err)
	}

	if !storedSession.IsActive(now) {
		t.Fatal("new session must be active")
	}

	updatedLastUsedAt := now.Add(time.Hour)

	err = sessionRepository.UpdateLastUsedAt(
		ctx,
		createdSession.ID,
		updatedLastUsedAt,
	)
	if err != nil {
		t.Fatalf("update last_used_at: %v", err)
	}

	storedSession, err = sessionRepository.GetByTokenHash(
		ctx,
		params.RefreshTokenHash,
	)
	if err != nil {
		t.Fatalf("get updated session: %v", err)
	}

	if !storedSession.LastUsedAt.Equal(updatedLastUsedAt) {
		t.Fatalf(
			"unexpected last_used_at: got %s, want %s",
			storedSession.LastUsedAt,
			updatedLastUsedAt,
		)
	}

	revokedAt := now.Add(2 * time.Hour)

	err = sessionRepository.Revoke(
		ctx,
		createdSession.ID,
		revokedAt,
	)
	if err != nil {
		t.Fatalf("revoke session: %v", err)
	}

	storedSession, err = sessionRepository.GetByTokenHash(
		ctx,
		params.RefreshTokenHash,
	)
	if err != nil {
		t.Fatalf("get revoked session: %v", err)
	}

	if !storedSession.IsRevoked() {
		t.Fatal("session must be revoked")
	}

	if storedSession.IsActive(revokedAt) {
		t.Fatal("revoked session must not be active")
	}

	err = sessionRepository.Revoke(
		ctx,
		createdSession.ID,
		revokedAt,
	)
	if !errors.Is(err, autherrors.ErrSessionNotFound) {
		t.Fatalf(
			"expected ErrSessionNotFound, got %v",
			err,
		)
	}
}

func TestSessionRepository_RevokeAllAndDeleteExpired(
	t *testing.T,
) {
	db := prepareTestDatabase(t)

	userRepository := postgres.NewUserRepository(db)
	sessionRepository := postgres.NewSessionRepository(db)

	ctx := context.Background()

	user := createTestUser(
		t,
		ctx,
		userRepository,
	)

	now := time.Now().UTC().Truncate(time.Microsecond)

	firstSession := createTestSession(
		t,
		ctx,
		sessionRepository,
		user.ID,
		"first-token-hash",
		now,
		now.Add(24*time.Hour),
	)

	secondSession := createTestSession(
		t,
		ctx,
		sessionRepository,
		user.ID,
		"second-token-hash",
		now,
		now.Add(24*time.Hour),
	)

	revokedAt := now.Add(time.Hour)

	err := sessionRepository.RevokeAllByUserID(
		ctx,
		user.ID,
		revokedAt,
	)
	if err != nil {
		t.Fatalf("revoke all sessions: %v", err)
	}

	firstStored, err := sessionRepository.GetByTokenHash(
		ctx,
		firstSession.RefreshTokenHash,
	)
	if err != nil {
		t.Fatalf("get first session: %v", err)
	}

	secondStored, err := sessionRepository.GetByTokenHash(
		ctx,
		secondSession.RefreshTokenHash,
	)
	if err != nil {
		t.Fatalf("get second session: %v", err)
	}

	if !firstStored.IsRevoked() {
		t.Fatal("first session must be revoked")
	}

	if !secondStored.IsRevoked() {
		t.Fatal("second session must be revoked")
	}

	createTestSession(
		t,
		ctx,
		sessionRepository,
		user.ID,
		"expired-token-hash",
		now.Add(-48*time.Hour),
		now.Add(-24*time.Hour),
	)

	createTestSession(
		t,
		ctx,
		sessionRepository,
		user.ID,
		"active-token-hash",
		now,
		now.Add(24*time.Hour),
	)

	deletedCount, err := sessionRepository.DeleteExpired(
		ctx,
		now,
	)
	if err != nil {
		t.Fatalf("delete expired sessions: %v", err)
	}

	if deletedCount != 1 {
		t.Fatalf(
			"unexpected deleted count: got %d, want 1",
			deletedCount,
		)
	}

	_, err = sessionRepository.GetByTokenHash(
		ctx,
		"expired-token-hash",
	)
	if !errors.Is(err, autherrors.ErrSessionNotFound) {
		t.Fatalf(
			"expected expired session to be deleted, got %v",
			err,
		)
	}

	_, err = sessionRepository.GetByTokenHash(
		ctx,
		"active-token-hash",
	)
	if err != nil {
		t.Fatalf(
			"active session must remain: %v",
			err,
		)
	}
}

func prepareTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	postgresDB, err := database.OpenPostgres(databaseURL)
	if err != nil {
		t.Fatalf("open test PostgreSQL: %v", err)
	}

	t.Cleanup(func() {
		if err := postgresDB.Close(); err != nil {
			t.Errorf("close test PostgreSQL: %v", err)
		}
	})

	err = postgresDB.GORM.AutoMigrate(
		&postgres.UserModel{},
		&postgres.AuthSessionModel{},
	)
	if err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	clearDatabase(t, postgresDB.GORM)

	t.Cleanup(func() {
		clearDatabase(t, postgresDB.GORM)
	})

	return postgresDB.GORM
}

func clearDatabase(
	t *testing.T,
	db *gorm.DB,
) {
	t.Helper()

	err := db.Exec(`
		TRUNCATE TABLE
			auth_sessions,
			users
		CASCADE
	`).Error
	if err != nil {
		t.Fatalf("clear test database: %v", err)
	}
}

func createTestUser(
	t *testing.T,
	ctx context.Context,
	repo authrepo.UserRepository,
) authmodel.User {
	t.Helper()

	user, err := repo.Create(
		ctx,
		authrepo.CreateUserParams{
			ID:                 uuid.New(),
			Nickname:           "test-user",
			NicknameNormalized: "test-user",
			PasswordHash:       "test-password-hash",
		},
	)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	return user
}

func createTestSession(
	t *testing.T,
	ctx context.Context,
	repo authrepo.SessionRepository,
	userID uuid.UUID,
	tokenHash string,
	createdAt time.Time,
	expiresAt time.Time,
) authmodel.AuthSession {
	t.Helper()

	session, err := repo.Create(
		ctx,
		authrepo.CreateSessionParams{
			ID:               uuid.New(),
			UserID:           userID,
			RefreshTokenHash: tokenHash,
			UserAgent:        "integration-test",
			CreatedAt:        createdAt,
			ExpiresAt:        expiresAt,
			LastUsedAt:       createdAt,
		},
	)
	if err != nil {
		t.Fatalf("create test session: %v", err)
	}

	return session
}
