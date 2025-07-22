package config

import (
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/conn"
)

type CKey string

type Config struct {
	DB         *conn.DB
	Validator  *validator.Validate
	SecretKey  string
	EncryptKey string
}

// AppConfig represents the application configuration
type AppConfig struct {
	Port             string
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
			DB:        &conn.DB{},
			Validator: validator.New(),
			// the secret key will change every time the application is restarted.
			SecretKey: GetEnv("JWT_SECRET", "default-secret-key"),
			//SecretKey: uuid.New().String(), // every time the application is restarted, the secret key will change.
			EncryptKey: GetEnv("ENCRYPT_SECRET", "default-encrypt-key"),
		}
		instance.DB.ConnectDatabase()
	}
	return instance
}

// GetAppConfig returns the application configuration
func GetAppConfig() *AppConfig {
	if appConfigInstance == nil {
		appConfigInstance = &AppConfig{
			Port:             GetEnv("APP_PORT", "9999"),
			EnableLogging:    GetBoolEnv("ENABLE_LOGGING", true),
			LoggingLevel:     GetEnv("LOGGING_LEVEL", "info"),
			LogRetentionDays: GetIntEnv("LOG_RETENTION_DAYS", 30),
		}
	}
	return appConfigInstance
}

// getEnv returns the value of an environment variable or a default value
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getBoolEnv returns the boolean value of an environment variable or a default value
func GetBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getIntEnv returns the integer value of an environment variable or a default value
func GetIntEnv(key string, defaultValue int) int {
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
