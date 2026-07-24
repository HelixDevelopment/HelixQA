package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerPort     int
	ServerHTTP3Port int
	ServerHost     string
	DatabaseURL    string
	RedisURL       string
	NATSURL        string
	JWTPrivateKeyPath string
	JWTPublicKeyPath  string
	JWTAccessExpiry   time.Duration
	JWTRefreshExpiry  time.Duration
	LogLevel       string
	LogFormat      string
	EncryptionKey  string

	StripeAPIKey       string
	StripeWebhookSecret string
	PayPalClientID     string
	PayPalSecret       string
	PayPalWebhookID    string
	SquareAccessToken  string
	SquareApplicationID string
	SquareWebhookSigKey string

	RateLimitRPS int

	BackgroundWorkers    int
	PollInterval         time.Duration
	IdempotencyTTLHours int

	ReconciliationInterval time.Duration
}

func Load() *Config {
	return &Config{
		ServerPort:      getEnvAsInt("SERVER_PORT", 8080),
		ServerHTTP3Port: getEnvAsInt("SERVER_HTTP3_PORT", 8443),
		ServerHost:      getEnv("SERVER_HOST", "0.0.0.0"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgresql://helix:helix@localhost:5432/helix_seller"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		NATSURL:         getEnv("NATS_URL", "nats://localhost:4222"),
		JWTPrivateKeyPath: getEnv("JWT_PRIVATE_KEY_PATH", "keys/jwt_private.pem"),
		JWTPublicKeyPath:  getEnv("JWT_PUBLIC_KEY_PATH", "keys/jwt_public.pem"),
		JWTAccessExpiry:   getEnvAsDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
		JWTRefreshExpiry:  getEnvAsDuration("JWT_REFRESH_EXPIRY", 168*time.Hour),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		LogFormat:       getEnv("LOG_FORMAT", "json"),
		EncryptionKey:   getEnv("ENCRYPTION_KEY", ""),

		StripeAPIKey:       getEnv("STRIPE_API_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		PayPalClientID:     getEnv("PAYPAL_CLIENT_ID", ""),
		PayPalSecret:       getEnv("PAYPAL_SECRET", ""),
		PayPalWebhookID:    getEnv("PAYPAL_WEBHOOK_ID", ""),
		SquareAccessToken:  getEnv("SQUARE_ACCESS_TOKEN", ""),
		SquareApplicationID: getEnv("SQUARE_APPLICATION_ID", ""),
		SquareWebhookSigKey: getEnv("SQUARE_WEBHOOK_SIGNATURE_KEY", ""),

		RateLimitRPS: getEnvAsInt("RATE_LIMIT_RPS", 100),

		BackgroundWorkers:    getEnvAsInt("BACKGROUND_WORKERS", 4),
		PollInterval:         getEnvAsDuration("BACKGROUND_POLL_INTERVAL", 5*time.Second),
		IdempotencyTTLHours: getEnvAsInt("IDEMPOTENCY_TTL_HOURS", 24),

		ReconciliationInterval: getEnvAsDuration("RECONCILIATION_INTERVAL", 1*time.Hour),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
