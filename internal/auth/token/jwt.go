package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	minimumSecretLength = 32
	refreshTokenLength  = 32
	tokenTypeAccess     = "access"
)

var (
	ErrInvalidAccessToken = errors.New("invalid access token")
	ErrInvalidTokenConfig = errors.New("invalid token configuration")
)

type JWTManager struct {
	accessSecret []byte
	issuer       string
	accessTTL    time.Duration
}

type accessTokenClaims struct {
	TokenType string `json:"token_type"`

	jwt.RegisteredClaims
}

func NewJWTManager(
	accessSecret string,
	issuer string,
	accessTTL time.Duration,
) (*JWTManager, error) {
	if len(accessSecret) < minimumSecretLength {
		return nil, fmt.Errorf(
			"%w: access secret must contain at least %d bytes",
			ErrInvalidTokenConfig,
			minimumSecretLength,
		)
	}

	if issuer == "" {
		return nil, fmt.Errorf(
			"%w: issuer is required",
			ErrInvalidTokenConfig,
		)
	}

	if accessTTL <= 0 {
		return nil, fmt.Errorf(
			"%w: access TTL must be positive",
			ErrInvalidTokenConfig,
		)
	}

	return &JWTManager{
		accessSecret: []byte(accessSecret),
		issuer:       issuer,
		accessTTL:    accessTTL,
	}, nil
}

func (m *JWTManager) CreateAccessToken(
	_ context.Context,
	userID uuid.UUID,
	issuedAt time.Time,
) (AccessToken, error) {
	if userID == uuid.Nil {
		return AccessToken{}, fmt.Errorf(
			"%w: user ID is empty",
			ErrInvalidAccessToken,
		)
	}

	issuedAt = issuedAt.UTC()
	expiresAt := issuedAt.Add(m.accessTTL)

	claims := accessTokenClaims{
		TokenType: tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			ID:        uuid.NewString(),
		},
	}

	jwtToken := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	signedToken, err := jwtToken.SignedString(m.accessSecret)
	if err != nil {
		return AccessToken{}, fmt.Errorf(
			"sign access token: %w",
			err,
		)
	}

	return AccessToken{
		Value:     signedToken,
		ExpiresAt: expiresAt,
	}, nil
}

func (m *JWTManager) ParseAccessToken(
	_ context.Context,
	tokenValue string,
) (AccessTokenClaims, error) {
	if tokenValue == "" {
		return AccessTokenClaims{}, ErrInvalidAccessToken
	}

	claims := &accessTokenClaims{}

	parsedToken, err := jwt.ParseWithClaims(
		tokenValue,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidAccessToken
			}

			return m.accessSecret, nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
		}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return AccessTokenClaims{}, fmt.Errorf(
			"%w: %v",
			ErrInvalidAccessToken,
			err,
		)
	}

	if !parsedToken.Valid {
		return AccessTokenClaims{}, ErrInvalidAccessToken
	}

	if claims.TokenType != tokenTypeAccess {
		return AccessTokenClaims{}, ErrInvalidAccessToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil || userID == uuid.Nil {
		return AccessTokenClaims{}, ErrInvalidAccessToken
	}

	return AccessTokenClaims{
		UserID: userID,
	}, nil
}

func (m *JWTManager) CreateRefreshToken(
	_ context.Context,
) (RefreshToken, error) {
	randomBytes := make([]byte, refreshTokenLength)

	if _, err := rand.Read(randomBytes); err != nil {
		return RefreshToken{}, fmt.Errorf(
			"generate refresh token: %w",
			err,
		)
	}

	tokenValue := base64.RawURLEncoding.EncodeToString(randomBytes)

	return RefreshToken{
		Value: tokenValue,
		Hash:  m.HashRefreshToken(tokenValue),
	}, nil
}

func (m *JWTManager) HashRefreshToken(tokenValue string) string {
	hash := sha256.Sum256([]byte(tokenValue))

	return hex.EncodeToString(hash[:])
}

var _ Manager = (*JWTManager)(nil)
