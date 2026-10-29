// Package config loads and validates application settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application settings loaded from environment variables.
type Config struct {
	APPEnv              string
	HTTPAddr            string
	HTTPReadTimeout     time.Duration
	HTTPWriteTimeout    time.Duration
	HTTPIdleTimeout     time.Duration
	ShutdownTimeout     time.Duration
	LogLevel            string
	LogFormat           string
	DatabaseURL         string
	DBMinConns          int32
	DBMaxConns          int32
	DBMaxConnLifetime   time.Duration
	DBMaxConnIdleTime   time.Duration
	DBHealthCheckPeriod time.Duration
	DBConnTimeout       time.Duration
	JWTSecret           string
	JWTIssuer           string
	JWTAudience         string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	Argon2Memory        uint32
	Argon2Iterations    uint32
	Argon2Parallelism   uint8
	Argon2KeyLen        uint32
	PasswordPepper      string
	CORSAllowedOrigins  []string
	RateLimitRPS        int
	RateLimitBurst      int
	TrustedProxies      []string
	MetricsAddr         string
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		APPEnv:              getenv("APP_ENV", "development"),
		HTTPAddr:            getenv("HTTP_ADDR", ":8080"),
		HTTPReadTimeout:     mustDuration("HTTP_READ_TIMEOUT", 5*time.Second),
		HTTPWriteTimeout:    mustDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
		HTTPIdleTimeout:     mustDuration("HTTP_IDLE_TIMEOUT", 30*time.Second),
		ShutdownTimeout:     mustDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		LogLevel:            strings.ToUpper(getenv("LOG_LEVEL", "INFO")),
		LogFormat:           strings.ToLower(getenv("LOG_FORMAT", "json")),
		DatabaseURL:         getenv("DATABASE_URL", ""),
		DBMinConns:          mustInt32("DB_MIN_CONNS", 2),
		DBMaxConns:          mustInt32("DB_MAX_CONNS", 20),
		DBMaxConnLifetime:   mustDuration("DB_MAX_CONN_LIFETIME", time.Hour),
		DBMaxConnIdleTime:   mustDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
		DBHealthCheckPeriod: mustDuration("DB_HEALTH_CHECK_PERIOD", 30*time.Second),
		DBConnTimeout:       mustDuration("DB_CONN_TIMEOUT", 5*time.Second),
		JWTSecret:           getenv("JWT_SECRET", ""),
		JWTIssuer:           getenv("JWT_ISSUER", "prod-go"),
		JWTAudience:         getenv("JWT_AUDIENCE", "prod-go"),
		AccessTokenTTL:      mustDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:     mustDuration("REFRESH_TOKEN_TTL", 720*time.Hour),
		Argon2Memory:        mustUint32("ARGON2_MEMORY", 64),
		Argon2Iterations:    mustUint32("ARGON2_ITERATIONS", 3),
		Argon2Parallelism:   mustUint8("ARGON2_PARALLELISM", 2),
		Argon2KeyLen:        mustUint32("ARGON2_KEYLEN", 32),
		PasswordPepper:      getenv("PASSWORD_PEPPER", ""),
		CORSAllowedOrigins:  strings.FieldsFunc(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"), func(r rune) bool { return r == ',' }),
		RateLimitRPS:        mustInt("RATE_LIMIT_RPS", 10),
		RateLimitBurst:      mustInt("RATE_LIMIT_BURST", 20),
		TrustedProxies:      strings.FieldsFunc(getenv("TRUSTED_PROXIES", "127.0.0.1,::1"), func(r rune) bool { return r == ',' || r == ' ' }),
		MetricsAddr:         getenv("METRICS_ADDR", ":9090"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 bytes")
	}
	if c.LogFormat != "json" && c.LogFormat != "text" {
		return fmt.Errorf("LOG_FORMAT must be json or text")
	}
	if c.HTTPAddr == "" {
		return fmt.Errorf("HTTP_ADDR is required")
	}
	if c.DBMinConns <= 0 || c.DBMaxConns <= 0 || c.DBMinConns > c.DBMaxConns {
		return fmt.Errorf("DB_MIN_CONNS and DB_MAX_CONNS must be valid and DB_MIN_CONNS <= DB_MAX_CONNS")
	}
	if c.DBMaxConnLifetime <= 0 || c.DBMaxConnIdleTime <= 0 || c.DBHealthCheckPeriod <= 0 || c.DBConnTimeout <= 0 {
		return fmt.Errorf("database pool durations must be positive")
	}
	if c.Argon2Memory < 32 || c.Argon2Iterations < 1 || c.Argon2Parallelism < 1 || c.Argon2KeyLen < 16 {
		return fmt.Errorf("invalid argon2 configuration")
	}
	if c.AccessTokenTTL <= 0 || c.RefreshTokenTTL <= 0 {
		return fmt.Errorf("token TTLs must be positive")
	}
	return nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func mustInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		value, err := strconv.Atoi(v)
		if err == nil {
			return value
		}
	}
	return fallback
}

func mustInt32(key string, fallback int32) int32 {
	if v, ok := os.LookupEnv(key); ok {
		parsed, err := strconv.ParseInt(v, 10, 32)
		if err == nil {
			return int32(parsed)
		}
	}
	return fallback
}

func mustUint32(key string, fallback uint32) uint32 {
	if v, ok := os.LookupEnv(key); ok {
		parsed, err := strconv.ParseUint(v, 10, 32)
		if err == nil {
			return uint32(parsed)
		}
	}
	return fallback
}

func mustUint8(key string, fallback uint8) uint8 {
	if v, ok := os.LookupEnv(key); ok {
		parsed, err := strconv.ParseUint(v, 10, 8)
		if err == nil {
			return uint8(parsed)
		}
	}
	return fallback
}

func mustDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return fallback
}
