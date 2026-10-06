package config

import (
	"errors"
	"io/fs"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/mstgnz/gopay/infra/conn"
)

type CKey string

type Config struct {
	DB        *conn.DB
	Validator *validator.Validate
	SecretKey string
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
			// The fallback is public in this repo; cmd/main.go refuses to start with it.
			SecretKey: GetEnv("JWT_SECRET", fallbackJWTSecret),
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

const (
	// fallbackJWTSecret is public in this repository: tokens signed with it can be minted by anyone.
	fallbackJWTSecret = "default-secret-key"
	// minJWTSecretLen is the HS256 key size in bytes.
	minJWTSecretLen = 32
)

// CheckJWTSecret refuses an unset or public JWT secret and reports a short one.
func CheckJWTSecret(secret string) (weak bool, err error) {
	if secret == "" || secret == fallbackJWTSecret {
		return false, errors.New("JWT_SECRET is unset or the public fallback, so anyone could mint tokens")
	}
	return len(secret) < minJWTSecretLen, nil
}

// LoadDotEnv loads a .env file when one is present. A missing file is not an error: the
// container gets its variables from compose env_file and the image no longer ships a .env.
// Variables already set in the process environment win over the file.
func LoadDotEnv(path string) (loaded bool, err error) {
	if err := godotenv.Load(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
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
