package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type LogTarget string

const (
	TargetConsole LogTarget = "CONSOLE"
	TargetFile    LogTarget = "FILE"
)

type Config struct {
	AppEnv             string
	AppPort            string
	EnableDevEndpoints bool
	AuthJWKSURL        string
	AuthIssuer         string
	AuthAudience       string
	FakeWebhookSecret  string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string

	// RabbitMQ
	RabbitMQHost string
	RabbitMQPort string
	RabbitMQUser string
	RabbitMQPass string

	// S3 / Object Storage
	S3Endpoint  string
	S3Region    string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string

	// Webhooks
	ReceiptWebhookURL string

	// Logging
	LogTargets []LogTarget
}

func GetEnv(envKey string) (string, error) {
	val, exists := os.LookupEnv(envKey)
	if !exists || val == "" {
		return "", fmt.Errorf(
			"%s environment variable is required",
			envKey,
		)
	}

	return val, nil
}

// Load reads the .env file and environment variables
// into the Config struct.
func Load() (*Config, error) {
	// .env is optional; environment variables may be provided directly by the
	// operating system or deployment environment.
	_ = godotenv.Load()

	appEnv, err := GetEnv("APP_ENV")
	if err != nil {
		return nil, err
	}

	appPort, err := GetEnv("APP_PORT")
	if err != nil {
		return nil, err
	}

	enableDevEndpoints, err := parseEnableDevEndpoints(os.Getenv("ENABLE_DEV_ENDPOINTS"))
	if err != nil {
		return nil, err
	}
	if err := validateDevEndpoints(appEnv, enableDevEndpoints); err != nil {
		return nil, err
	}
	authJWKSURL, authIssuer, authAudience, err := loadAuthConfig(appEnv)
	if err != nil {
		return nil, err
	}
	fakeWebhookSecret := os.Getenv("FAKE_WEBHOOK_HMAC_SECRET")
	if enableDevEndpoints && len(fakeWebhookSecret) < 32 {
		return nil, fmt.Errorf("FAKE_WEBHOOK_HMAC_SECRET must contain at least 32 bytes when ENABLE_DEV_ENDPOINTS=true")
	}

	// Database
	dbHost, err := GetEnv("DB_HOST")
	if err != nil {
		return nil, err
	}

	dbPort, err := GetEnv("DB_PORT")
	if err != nil {
		return nil, err
	}

	dbUser, err := GetEnv("DB_USER")
	if err != nil {
		return nil, err
	}

	dbPassword, err := GetEnv("DB_PASSWORD")
	if err != nil {
		return nil, err
	}

	dbName, err := GetEnv("DB_NAME")
	if err != nil {
		return nil, err
	}

	// Redis
	redisHost, err := GetEnv("REDIS_HOST")
	if err != nil {
		return nil, err
	}

	redisPort, err := GetEnv("REDIS_PORT")
	if err != nil {
		return nil, err
	}

	redisPassword, err := GetEnv("REDIS_PASSWORD")
	if err != nil {
		return nil, err
	}

	// RabbitMQ
	rabbitMQHost, err := GetEnv("RABBITMQ_HOST")
	if err != nil {
		return nil, err
	}

	rabbitMQPort, err := GetEnv("RABBITMQ_PORT")
	if err != nil {
		return nil, err
	}

	rabbitMQUser, err := GetEnv("RABBITMQ_USER")
	if err != nil {
		return nil, err
	}

	rabbitMQPass, err := GetEnv("RABBITMQ_PASSWORD")
	if err != nil {
		return nil, err
	}

	// S3 / Object Storage
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	s3Region := os.Getenv("S3_REGION")
	s3Bucket := os.Getenv("S3_BUCKET")
	s3AccessKey := os.Getenv("S3_ACCESS_KEY")
	s3SecretKey := os.Getenv("S3_SECRET_KEY")

	// Webhooks
	receiptWebhookURL := os.Getenv("RECEIPT_WEBHOOK_URL")

	return &Config{
		AppEnv:             appEnv,
		AppPort:            appPort,
		EnableDevEndpoints: enableDevEndpoints,
		AuthJWKSURL:        authJWKSURL,
		AuthIssuer:         authIssuer,
		AuthAudience:       authAudience,
		FakeWebhookSecret:  fakeWebhookSecret,

		DBHost:     dbHost,
		DBPort:     dbPort,
		DBUser:     dbUser,
		DBPassword: dbPassword,
		DBName:     dbName,

		RedisHost:     redisHost,
		RedisPort:     redisPort,
		RedisPassword: redisPassword,

		RabbitMQHost: rabbitMQHost,
		RabbitMQPort: rabbitMQPort,
		RabbitMQUser: rabbitMQUser,
		RabbitMQPass: rabbitMQPass,

		S3Endpoint:  s3Endpoint,
		S3Region:    s3Region,
		S3Bucket:    s3Bucket,
		S3AccessKey: s3AccessKey,
		S3SecretKey: s3SecretKey,

		ReceiptWebhookURL: receiptWebhookURL,
	}, nil
}

func loadAuthConfig(appEnv string) (string, string, string, error) {
	jwksURL := strings.TrimSpace(os.Getenv("AUTH_JWKS_URL"))
	issuer := strings.TrimSpace(os.Getenv("AUTH_ISSUER"))
	audience := strings.TrimSpace(os.Getenv("AUTH_AUDIENCE"))
	if jwksURL == "" || issuer == "" || audience == "" {
		return "", "", "", fmt.Errorf("AUTH_JWKS_URL, AUTH_ISSUER, and AUTH_AUDIENCE are required")
	}
	parsed, err := url.ParseRequestURI(jwksURL)
	if err != nil || parsed.Host == "" {
		return "", "", "", fmt.Errorf("AUTH_JWKS_URL must use HTTPS (HTTP is allowed only for localhost or mock identity in development)")
	}
	allowLocalHTTP := isLoopbackHost(parsed.Hostname()) ||
		(strings.EqualFold(strings.TrimSpace(appEnv), "development") && parsed.Hostname() == "mock-identity")
	if parsed.Scheme != "https" && (parsed.Scheme != "http" || !allowLocalHTTP) {
		return "", "", "", fmt.Errorf("AUTH_JWKS_URL must use HTTPS (HTTP is allowed only for localhost or mock identity in development)")
	}
	return jwksURL, issuer, audience, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func parseEnableDevEndpoints(raw string) (bool, error) {
	if strings.TrimSpace(raw) == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("ENABLE_DEV_ENDPOINTS must be a boolean: %w", err)
	}
	return enabled, nil
}

func validateDevEndpoints(appEnv string, enabled bool) error {
	if enabled && strings.EqualFold(strings.TrimSpace(appEnv), "production") {
		return fmt.Errorf("ENABLE_DEV_ENDPOINTS cannot be enabled when APP_ENV=production")
	}
	return nil
}
