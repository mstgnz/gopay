package config

import (
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type CKey string

type Config struct {
	Validator *validator.Validate
	SecretKey string
}

// AppConfig represents the application configuration
type AppConfig struct {
	Port             string
	OpenSearchURL    string
	OpenSearchUser   string
	OpenSearchPass   string
	EnableLogging    bool
	LoggingLevel     string
	LogRetentionDays int
}

var (
	instance          *Config
	appConfigInstance *AppConfig
)

func App() *Config {
	if instance == nil {
		instance = &Config{
			Validator: validator.New(),
			// the secret key will change every time the application is restarted.
			SecretKey: uuid.New().String(),
		}
	}
	return instance
}

// GetAppConfig returns the application configuration
func GetAppConfig() *AppConfig {
	if appConfigInstance == nil {
		appConfigInstance = &AppConfig{
			Port:             getEnv("APP_PORT", "9999"),
			OpenSearchURL:    getEnv("OPENSEARCH_URL", "http://localhost:9200"),
			OpenSearchUser:   getEnv("OPENSEARCH_USER", ""),
			OpenSearchPass:   getEnv("OPENSEARCH_PASSWORD", ""),
			EnableLogging:    getBoolEnv("ENABLE_OPENSEARCH_LOGGING", true),
			LoggingLevel:     getEnv("LOGGING_LEVEL", "info"),
			LogRetentionDays: getIntEnv("LOG_RETENTION_DAYS", 30),
		}
	}
	return appConfigInstance
}

// getEnv returns the value of an environment variable or a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getBoolEnv returns the boolean value of an environment variable or a default value
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getIntEnv returns the integer value of an environment variable or a default value
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func RandomString(length int) string {
	var charset = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
