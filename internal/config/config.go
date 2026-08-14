package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type LogTarget string

const (
	TargetConsole LogTarget = "CONSOLE"
	TargetFile    LogTarget = "FILE"
)

type Config struct {
	AppEnv  string
	AppPort string

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
	if err := godotenv.Load(); err != nil {
		// .env is optional.
		// Environment variables may be provided directly
		// by the operating system or deployment environment.
	}

	appEnv, err := GetEnv("APP_ENV")
	if err != nil {
		return nil, err
	}

	appPort, err := GetEnv("APP_PORT")
	if err != nil {
		return nil, err
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

	return &Config{
		AppEnv:        appEnv,
		AppPort:       appPort,

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
	}, nil
}