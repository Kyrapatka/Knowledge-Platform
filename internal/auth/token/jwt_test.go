package token

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	testSecret = "01234567890123456789012345678901"
	testIssuer = "knowledge-platform"
)

func TestJWTManager_CreateAndParseAccessToken(t *testing.T) {
	manager := newTestJWTManager(t)

	userID := uuid.New()
	issuedAt := time.Now().UTC().Add(-time.Minute)

	accessToken, err := manager.CreateAccessToken(
		context.Background(),
		userID,
		issuedAt,
	)
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}

	if accessToken.Value == "" {
		t.Fatal("access token value must not be empty")
	}

	expectedExpiresAt := issuedAt.Add(15 * time.Minute)

	if !accessToken.ExpiresAt.Equal(expectedExpiresAt) {
		t.Fatalf(
			"unexpected expiration time: got %s, want %s",
			accessToken.ExpiresAt,
			expectedExpiresAt,
		)
	}

	claims, err := manager.ParseAccessToken(
		context.Background(),
		accessToken.Value,
	)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf(
			"unexpected user ID: got %s, want %s",
			claims.UserID,
			userID,
		)
	}
}

func TestJWTManager_CreateAccessTokenEmptyUserID(t *testing.T) {
	manager := newTestJWTManager(t)

	_, err := manager.CreateAccessToken(
		context.Background(),
		uuid.Nil,
		time.Now().UTC(),
	)
	if !errors.Is(err, ErrInvalidAccessToken) {
		t.Fatalf(
			"expected ErrInvalidAccessToken, got %v",
			err,
		)
	}
}

func TestJWTManager_ParseExpiredAccessToken(t *testing.T) {
	manager, err := NewJWTManager(
		testSecret,
		testIssuer,
		time.Minute,
	)
	if err != nil {
		t.Fatalf("create JWT manager: %v", err)
	}

	issuedAt := time.Now().UTC().Add(-2 * time.Minute)

	accessToken, err := manager.CreateAccessToken(
		context.Background(),
		uuid.New(),
		issuedAt,
	)
	if err != nil {
		t.Fatalf("create expired access token: %v", err)
	}

	_, err = manager.ParseAccessToken(
		context.Background(),
		accessToken.Value,
	)
	if !errors.Is(err, ErrInvalidAccessToken) {
		t.Fatalf(
			"expected ErrInvalidAccessToken, got %v",
			err,
		)
	}
}

func TestJWTManager_ParseAccessTokenWithWrongSecret(t *testing.T) {
	creator := newTestJWTManager(t)

	parser, err := NewJWTManager(
		"abcdefghijklmnopqrstuvwxyz123456",
		testIssuer,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("create parser JWT manager: %v", err)
	}

	accessToken, err := creator.CreateAccessToken(
		context.Background(),
		uuid.New(),
		time.Now().UTC().Add(-time.Minute),
	)
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}

	_, err = parser.ParseAccessToken(
		context.Background(),
		accessToken.Value,
	)
	if !errors.Is(err, ErrInvalidAccessToken) {
		t.Fatalf(
			"expected ErrInvalidAccessToken, got %v",
			err,
		)
	}
}

func TestJWTManager_ParseAccessTokenWithWrongIssuer(t *testing.T) {
	creator := newTestJWTManager(t)

	parser, err := NewJWTManager(
		testSecret,
		"another-service",
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("create parser JWT manager: %v", err)
	}

	accessToken, err := creator.CreateAccessToken(
		context.Background(),
		uuid.New(),
		time.Now().UTC().Add(-time.Minute),
	)
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}

	_, err = parser.ParseAccessToken(
		context.Background(),
		accessToken.Value,
	)
	if !errors.Is(err, ErrInvalidAccessToken) {
		t.Fatalf(
			"expected ErrInvalidAccessToken, got %v",
			err,
		)
	}
}

func TestJWTManager_ParseAccessTokenWithWrongAlgorithm(t *testing.T) {
	manager := newTestJWTManager(t)

	now := time.Now().UTC()

	claims := accessTokenClaims{
		TokenType: tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    testIssuer,
			Subject:   uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
			ID:        uuid.NewString(),
		},
	}

	jwtToken := jwt.NewWithClaims(
		jwt.SigningMethodHS384,
		claims,
	)

	tokenValue, err := jwtToken.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token with HS384: %v", err)
	}

	_, err = manager.ParseAccessToken(
		context.Background(),
		tokenValue,
	)
	if !errors.Is(err, ErrInvalidAccessToken) {
		t.Fatalf(
			"expected ErrInvalidAccessToken, got %v",
			err,
		)
	}
}

func TestJWTManager_ParseEmptyAccessToken(t *testing.T) {
	manager := newTestJWTManager(t)

	_, err := manager.ParseAccessToken(
		context.Background(),
		"",
	)
	if !errors.Is(err, ErrInvalidAccessToken) {
		t.Fatalf(
			"expected ErrInvalidAccessToken, got %v",
			err,
		)
	}
}

func TestNewJWTManager_InvalidConfiguration(t *testing.T) {
	testCases := []struct {
		name      string
		secret    string
		issuer    string
		accessTTL time.Duration
	}{
		{
			name:      "short secret",
			secret:    "short-secret",
			issuer:    testIssuer,
			accessTTL: 15 * time.Minute,
		},
		{
			name:      "empty issuer",
			secret:    testSecret,
			issuer:    "",
			accessTTL: 15 * time.Minute,
		},
		{
			name:      "zero TTL",
			secret:    testSecret,
			issuer:    testIssuer,
			accessTTL: 0,
		},
		{
			name:      "negative TTL",
			secret:    testSecret,
			issuer:    testIssuer,
			accessTTL: -time.Minute,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			manager, err := NewJWTManager(
				testCase.secret,
				testCase.issuer,
				testCase.accessTTL,
			)

			if manager != nil {
				t.Fatal("manager must be nil")
			}

			if !errors.Is(err, ErrInvalidTokenConfig) {
				t.Fatalf(
					"expected ErrInvalidTokenConfig, got %v",
					err,
				)
			}
		})
	}
}

func TestJWTManager_CreateRefreshToken(t *testing.T) {
	manager := newTestJWTManager(t)

	firstToken, err := manager.CreateRefreshToken(
		context.Background(),
	)
	if err != nil {
		t.Fatalf("create first refresh token: %v", err)
	}

	secondToken, err := manager.CreateRefreshToken(
		context.Background(),
	)
	if err != nil {
		t.Fatalf("create second refresh token: %v", err)
	}

	if firstToken.Value == "" {
		t.Fatal("first refresh token value must not be empty")
	}

	if firstToken.Hash == "" {
		t.Fatal("first refresh token hash must not be empty")
	}

	if firstToken.Value == firstToken.Hash {
		t.Fatal("refresh token value and hash must be different")
	}

	if firstToken.Value == secondToken.Value {
		t.Fatal("refresh token values must be different")
	}

	if firstToken.Hash == secondToken.Hash {
		t.Fatal("refresh token hashes must be different")
	}

	expectedHash := manager.HashRefreshToken(firstToken.Value)

	if firstToken.Hash != expectedHash {
		t.Fatalf(
			"unexpected refresh token hash: got %q, want %q",
			firstToken.Hash,
			expectedHash,
		)
	}
}

func TestJWTManager_HashRefreshTokenIsStable(t *testing.T) {
	manager := newTestJWTManager(t)

	const tokenValue = "refresh-token-value"

	firstHash := manager.HashRefreshToken(tokenValue)
	secondHash := manager.HashRefreshToken(tokenValue)

	if firstHash == "" {
		t.Fatal("refresh token hash must not be empty")
	}

	if firstHash != secondHash {
		t.Fatalf(
			"hash must be stable: first %q, second %q",
			firstHash,
			secondHash,
		)
	}

	differentHash := manager.HashRefreshToken(
		"different-refresh-token",
	)

	if firstHash == differentHash {
		t.Fatal("different refresh tokens must have different hashes")
	}
}

func newTestJWTManager(t *testing.T) *JWTManager {
	t.Helper()

	manager, err := NewJWTManager(
		testSecret,
		testIssuer,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("create JWT manager: %v", err)
	}

	return manager
}
