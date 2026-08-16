package config

import (
	"errors"
	"os"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultHTTPAddress    = ":8080"
	defaultJWTIssuer      = "knowledge-platform"
	defaultAccessTokenTTL = 15 * time.Minute
	defaultRefreshTTL     = 30 * 24 * time.Hour
)

type Config struct {
	DatabaseURL string

	HTTPAddress string

	JWTSecret       string
	JWTIssuer       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func Load() (Config, error) {
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}

	httpAddress := os.Getenv("HTTP_ADDRESS")
	if httpAddress == "" {
		httpAddress = defaultHTTPAddress
	}

	jwtIssuer := os.Getenv("JWT_ISSUER")
	if jwtIssuer == "" {
		jwtIssuer = defaultJWTIssuer
	}

	accessTokenTTL, err := durationFromEnv(
		"ACCESS_TOKEN_TTL",
		defaultAccessTokenTTL,
	)
	if err != nil {
		return Config{}, err
	}

	refreshTokenTTL, err := durationFromEnv(
		"REFRESH_TOKEN_TTL",
		defaultRefreshTTL,
	)
	if err != nil {
		return Config{}, err
	}

	return Config{
		DatabaseURL: databaseURL,

		HTTPAddress: httpAddress,

		JWTSecret:       jwtSecret,
		JWTIssuer:       jwtIssuer,
		AccessTokenTTL:  accessTokenTTL,
		RefreshTokenTTL: refreshTokenTTL,
	}, nil
}

func durationFromEnv(
	key string,
	defaultValue time.Duration,
) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, errors.New(
			key + " must be a valid duration",
		)
	}

	if duration <= 0 {
		return 0, errors.New(
			key + " must be positive",
		)
	}

	return duration, nil
}
