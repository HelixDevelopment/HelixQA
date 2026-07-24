package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear all env vars
	envVars := []string{
		"SERVER_PORT", "SERVER_HTTP3_PORT", "SERVER_HOST",
		"DATABASE_URL", "REDIS_URL", "NATS_URL",
		"JWT_PRIVATE_KEY_PATH", "JWT_PUBLIC_KEY_PATH",
		"JWT_ACCESS_EXPIRY", "JWT_REFRESH_EXPIRY",
		"LOG_LEVEL", "LOG_FORMAT", "ENCRYPTION_KEY",
		"STRIPE_API_KEY", "STRIPE_WEBHOOK_SECRET",
		"PAYPAL_CLIENT_ID", "PAYPAL_SECRET", "PAYPAL_WEBHOOK_ID",
		"SQUARE_ACCESS_TOKEN", "SQUARE_APPLICATION_ID", "SQUARE_WEBHOOK_SIGNATURE_KEY",
		"RATE_LIMIT_RPS", "BACKGROUND_WORKERS", "BACKGROUND_POLL_INTERVAL",
		"IDEMPOTENCY_TTL_HOURS", "RECONCILIATION_INTERVAL",
	}

	for _, v := range envVars {
		os.Unsetenv(v)
	}

	cfg := Load()

	if cfg.ServerPort != 8080 {
		t.Errorf("ServerPort = %d, want 8080", cfg.ServerPort)
	}
	if cfg.ServerHTTP3Port != 8443 {
		t.Errorf("ServerHTTP3Port = %d, want 8443", cfg.ServerHTTP3Port)
	}
	if cfg.ServerHost != "0.0.0.0" {
		t.Errorf("ServerHost = %q, want 0.0.0.0", cfg.ServerHost)
	}
	if cfg.DatabaseURL != "postgresql://helix:helix@localhost:5432/helix_seller" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "redis://localhost:6379" {
		t.Errorf("RedisURL = %q", cfg.RedisURL)
	}
	if cfg.NATSURL != "nats://localhost:4222" {
		t.Errorf("NATSURL = %q", cfg.NATSURL)
	}
	if cfg.JWTAccessExpiry != 15*time.Minute {
		t.Errorf("JWTAccessExpiry = %v, want 15m", cfg.JWTAccessExpiry)
	}
	if cfg.JWTRefreshExpiry != 168*time.Hour {
		t.Errorf("JWTRefreshExpiry = %v, want 168h", cfg.JWTRefreshExpiry)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want json", cfg.LogFormat)
	}
	if cfg.RateLimitRPS != 100 {
		t.Errorf("RateLimitRPS = %d, want 100", cfg.RateLimitRPS)
	}
	if cfg.BackgroundWorkers != 4 {
		t.Errorf("BackgroundWorkers = %d, want 4", cfg.BackgroundWorkers)
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want 5s", cfg.PollInterval)
	}
	if cfg.IdempotencyTTLHours != 24 {
		t.Errorf("IdempotencyTTLHours = %d, want 24", cfg.IdempotencyTTLHours)
	}
	if cfg.ReconciliationInterval != 1*time.Hour {
		t.Errorf("ReconciliationInterval = %v, want 1h", cfg.ReconciliationInterval)
	}
}

func TestLoad_WithEnvVars(t *testing.T) {
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("DATABASE_URL", "postgres://test:test@db:5432/test")
	os.Setenv("REDIS_URL", "redis://redis:6379")
	os.Setenv("NATS_URL", "nats://nats:4222")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("RATE_LIMIT_RPS", "50")
	os.Setenv("BACKGROUND_WORKERS", "8")
	defer func() {
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("NATS_URL")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("RATE_LIMIT_RPS")
		os.Unsetenv("BACKGROUND_WORKERS")
	}()

	cfg := Load()

	if cfg.ServerPort != 9090 {
		t.Errorf("ServerPort = %d, want 9090", cfg.ServerPort)
	}
	if cfg.DatabaseURL != "postgres://test:test@db:5432/test" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
	if cfg.RateLimitRPS != 50 {
		t.Errorf("RateLimitRPS = %d, want 50", cfg.RateLimitRPS)
	}
	if cfg.BackgroundWorkers != 8 {
		t.Errorf("BackgroundWorkers = %d, want 8", cfg.BackgroundWorkers)
	}
}

func TestLoad_InvalidInt(t *testing.T) {
	os.Setenv("SERVER_PORT", "not-a-number")
	defer os.Unsetenv("SERVER_PORT")

	cfg := Load()

	if cfg.ServerPort != 8080 {
		t.Errorf("ServerPort = %d, want 8080 (default on invalid)", cfg.ServerPort)
	}
}

func TestLoad_InvalidDuration(t *testing.T) {
	os.Setenv("JWT_ACCESS_EXPIRY", "not-a-duration")
	defer os.Unsetenv("JWT_ACCESS_EXPIRY")

	cfg := Load()

	if cfg.JWTAccessExpiry != 15*time.Minute {
		t.Errorf("JWTAccessExpiry = %v, want 15m (default on invalid)", cfg.JWTAccessExpiry)
	}
}

func TestLoad_EmptyStringValue(t *testing.T) {
	os.Setenv("ENCRYPTION_KEY", "")
	defer os.Unsetenv("ENCRYPTION_KEY")

	cfg := Load()

	if cfg.EncryptionKey != "" {
		t.Errorf("EncryptionKey = %q, want empty", cfg.EncryptionKey)
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_GET_ENV", "hello")
	defer os.Unsetenv("TEST_GET_ENV")

	if got := getEnv("TEST_GET_ENV", "default"); got != "hello" {
		t.Errorf("getEnv = %q, want hello", got)
	}
}

func TestGetEnv_Default(t *testing.T) {
	os.Unsetenv("TEST_GET_ENV_MISSING")

	if got := getEnv("TEST_GET_ENV_MISSING", "default"); got != "default" {
		t.Errorf("getEnv = %q, want default", got)
	}
}

func TestGetEnvAsInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	if got := getEnvAsInt("TEST_INT", 0); got != 42 {
		t.Errorf("getEnvAsInt = %d, want 42", got)
	}
}

func TestGetEnvAsInt_Default(t *testing.T) {
	os.Unsetenv("TEST_INT_MISSING")

	if got := getEnvAsInt("TEST_INT_MISSING", 100); got != 100 {
		t.Errorf("getEnvAsInt = %d, want 100", got)
	}
}

func TestGetEnvAsInt_Invalid(t *testing.T) {
	os.Setenv("TEST_INT_INVALID", "abc")
	defer os.Unsetenv("TEST_INT_INVALID")

	if got := getEnvAsInt("TEST_INT_INVALID", 50); got != 50 {
		t.Errorf("getEnvAsInt = %d, want 50 (default on invalid)", got)
	}
}

func TestGetEnvAsDuration(t *testing.T) {
	os.Setenv("TEST_DURATION", "5m")
	defer os.Unsetenv("TEST_DURATION")

	if got := getEnvAsDuration("TEST_DURATION", 0); got != 5*time.Minute {
		t.Errorf("getEnvAsDuration = %v, want 5m", got)
	}
}

func TestGetEnvAsDuration_Default(t *testing.T) {
	os.Unsetenv("TEST_DURATION_MISSING")

	if got := getEnvAsDuration("TEST_DURATION_MISSING", 10*time.Second); got != 10*time.Second {
		t.Errorf("getEnvAsDuration = %v, want 10s", got)
	}
}

func TestGetEnvAsDuration_Invalid(t *testing.T) {
	os.Setenv("TEST_DURATION_INVALID", "notaduration")
	defer os.Unsetenv("TEST_DURATION_INVALID")

	if got := getEnvAsDuration("TEST_DURATION_INVALID", 30*time.Second); got != 30*time.Second {
		t.Errorf("getEnvAsDuration = %v, want 30s (default on invalid)", got)
	}
}

func TestLoad_StripeKeys(t *testing.T) {
	os.Setenv("STRIPE_API_KEY", "sk_test_123")
	os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_456")
	defer func() {
		os.Unsetenv("STRIPE_API_KEY")
		os.Unsetenv("STRIPE_WEBHOOK_SECRET")
	}()

	cfg := Load()

	if cfg.StripeAPIKey != "sk_test_123" {
		t.Errorf("StripeAPIKey = %q", cfg.StripeAPIKey)
	}
	if cfg.StripeWebhookSecret != "whsec_456" {
		t.Errorf("StripeWebhookSecret = %q", cfg.StripeWebhookSecret)
	}
}

func TestLoad_PayPalKeys(t *testing.T) {
	os.Setenv("PAYPAL_CLIENT_ID", "client123")
	os.Setenv("PAYPAL_SECRET", "secret456")
	os.Setenv("PAYPAL_WEBHOOK_ID", "wh789")
	defer func() {
		os.Unsetenv("PAYPAL_CLIENT_ID")
		os.Unsetenv("PAYPAL_SECRET")
		os.Unsetenv("PAYPAL_WEBHOOK_ID")
	}()

	cfg := Load()

	if cfg.PayPalClientID != "client123" {
		t.Errorf("PayPalClientID = %q", cfg.PayPalClientID)
	}
	if cfg.PayPalSecret != "secret456" {
		t.Errorf("PayPalSecret = %q", cfg.PayPalSecret)
	}
	if cfg.PayPalWebhookID != "wh789" {
		t.Errorf("PayPalWebhookID = %q", cfg.PayPalWebhookID)
	}
}

func TestLoad_SquareKeys(t *testing.T) {
	os.Setenv("SQUARE_ACCESS_TOKEN", "sq0atp_123")
	os.Setenv("SQUARE_APPLICATION_ID", "sq0cid_456")
	os.Setenv("SQUARE_WEBHOOK_SIGNATURE_KEY", "sig789")
	defer func() {
		os.Unsetenv("SQUARE_ACCESS_TOKEN")
		os.Unsetenv("SQUARE_APPLICATION_ID")
		os.Unsetenv("SQUARE_WEBHOOK_SIGNATURE_KEY")
	}()

	cfg := Load()

	if cfg.SquareAccessToken != "sq0atp_123" {
		t.Errorf("SquareAccessToken = %q", cfg.SquareAccessToken)
	}
	if cfg.SquareApplicationID != "sq0cid_456" {
		t.Errorf("SquareApplicationID = %q", cfg.SquareApplicationID)
	}
	if cfg.SquareWebhookSigKey != "sig789" {
		t.Errorf("SquareWebhookSigKey = %q", cfg.SquareWebhookSigKey)
	}
}

func TestLoad_JWTPaths(t *testing.T) {
	os.Setenv("JWT_PRIVATE_KEY_PATH", "/custom/private.pem")
	os.Setenv("JWT_PUBLIC_KEY_PATH", "/custom/public.pem")
	defer func() {
		os.Unsetenv("JWT_PRIVATE_KEY_PATH")
		os.Unsetenv("JWT_PUBLIC_KEY_PATH")
	}()

	cfg := Load()

	if cfg.JWTPrivateKeyPath != "/custom/private.pem" {
		t.Errorf("JWTPrivateKeyPath = %q", cfg.JWTPrivateKeyPath)
	}
	if cfg.JWTPublicKeyPath != "/custom/public.pem" {
		t.Errorf("JWTPublicKeyPath = %q", cfg.JWTPublicKeyPath)
	}
}

func TestLoad_HostOverride(t *testing.T) {
	os.Setenv("SERVER_HOST", "127.0.0.1")
	defer os.Unsetenv("SERVER_HOST")

	cfg := Load()

	if cfg.ServerHost != "127.0.0.1" {
		t.Errorf("ServerHost = %q, want 127.0.0.1", cfg.ServerHost)
	}
}
