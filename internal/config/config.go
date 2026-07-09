package config

import (
	"fmt"
	"os"
	"strings"
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

	// logging
	LogTargets []LogTarget
}

func GetEnv(envKey string) (string, error) {
	val, exists := os.LookupEnv(envKey)
	if !exists || val == "" {
		return "", fmt.Errorf("%s environment variable is required", envKey)
	}
	return val, nil
}

// Load reads environment variables and parses them into the Config struct.
func Load() (*Config, error) {
	appEnv, err := GetEnv("APP_ENV")
	if err != nil {
		return nil, err
	}

	appPort, err := GetEnv("APP_PORT")
	if err != nil {
		return nil, err
	}

	// Database Validations
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

	// Redis Validations
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

	// RabbitMQ Validations
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
		DBHost:        dbHost,
		DBPort:        dbPort,
		DBUser:        dbUser,
		DBPassword:    dbPassword,
		DBName:        dbName,
		RedisHost:     redisHost,
		RedisPort:     redisPort,
		RedisPassword: redisPassword,
		RabbitMQHost:  rabbitMQHost,
		RabbitMQPort:  rabbitMQPort,
		RabbitMQUser:  rabbitMQUser,
		RabbitMQPass:  rabbitMQPass,
	}, nil
}

// log targets are gonna be defined in env with like LOG_TARGETS=CONSOLE,FILE
func parseLogTargets(val string) []LogTarget {
	if val == "" {
		return []LogTarget{TargetConsole} // Safe baseline default
	}

	rawTargets := strings.Split(val, ",")
	var targets []LogTarget
	for _, raw := range rawTargets {
		trimmed := LogTarget(strings.ToUpper(strings.TrimSpace(raw)))
		if trimmed != "" {
			targets = append(targets, trimmed)
		}
	}
	return targets
}
