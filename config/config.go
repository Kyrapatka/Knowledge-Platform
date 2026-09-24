package config

import (
	"errors"
	"os"
	"slices"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultHTTPAddress         = ":8080"
	defaultHTTPShutdownTimeout = 15 * time.Second
	defaultJWTIssuer           = "knowledge-platform"
	defaultAccessTokenTTL      = 15 * time.Minute
	defaultRefreshTTL          = 30 * 24 * time.Hour
)

type Config struct {
	Analytics   Analytics
	LogLevel    string
	LogFormat   string
	DatabaseURL string

	HTTPAddress         string
	HTTPShutdownTimeout time.Duration

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

	shutdownTimeout, err := durationFromEnv("HTTP_SHUTDOWN_TIMEOUT", defaultHTTPShutdownTimeout)
	if err != nil {
		return Config{}, err
	}
	logLevel, err := choiceFromEnv("LOG_LEVEL", "info", []string{"debug", "info", "warn", "error"})
	if err != nil {
		return Config{}, err
	}
	logFormat, err := choiceFromEnv("LOG_FORMAT", "text", []string{"text", "json"})
	if err != nil {
		return Config{}, err
	}
	analyticsConfig, err := analyticsFromEnv()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Analytics:   analyticsConfig,
		LogLevel:    logLevel,
		LogFormat:   logFormat,
		DatabaseURL: databaseURL,

		HTTPAddress:         httpAddress,
		HTTPShutdownTimeout: shutdownTimeout,

		JWTSecret:       jwtSecret,
		JWTIssuer:       jwtIssuer,
		AccessTokenTTL:  accessTokenTTL,
		RefreshTokenTTL: refreshTokenTTL,
	}, nil
}

func choiceFromEnv(key, fallback string, allowed []string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	if !slices.Contains(allowed, value) {
		return "", errors.New(key + " has an unsupported value")
	}
	return value, nil
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
