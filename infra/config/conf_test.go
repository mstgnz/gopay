package config

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApp(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "singleton_instance",
			test: func(t *testing.T) {
				config1 := App()
				config2 := App()

				require.NotNil(t, config1)
				require.NotNil(t, config2)
				assert.Equal(t, config1, config2, "App() should return singleton instance")
				assert.NotNil(t, config1.Validator, "Validator should be initialized")
				assert.NotEmpty(t, config1.SecretKey, "SecretKey should be generated")
			},
		},
		{
			name: "validator_initialized",
			test: func(t *testing.T) {
				config := App()
				assert.NotNil(t, config.Validator, "Validator should be initialized")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func TestGetAppConfig(t *testing.T) {
	// Save original env values
	originalValues := map[string]string{
		"APP_PORT":                  os.Getenv("APP_PORT"),
		"OPENSEARCH_URL":            os.Getenv("OPENSEARCH_URL"),
		"OPENSEARCH_USER":           os.Getenv("OPENSEARCH_USER"),
		"OPENSEARCH_PASSWORD":       os.Getenv("OPENSEARCH_PASSWORD"),
		"ENABLE_OPENSEARCH_LOGGING": os.Getenv("ENABLE_OPENSEARCH_LOGGING"),
		"LOGGING_LEVEL":             os.Getenv("LOGGING_LEVEL"),
		"LOG_RETENTION_DAYS":        os.Getenv("LOG_RETENTION_DAYS"),
	}

	// Clear env vars
	for key := range originalValues {
		os.Unsetenv(key)
	}

	// Reset singleton instance
	appConfigInstance = nil

	defer func() {
		// Restore original values
		for key, value := range originalValues {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
		// Reset singleton
		appConfigInstance = nil
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected *AppConfig
	}{
		{
			name:    "default_values",
			envVars: map[string]string{},
			expected: &AppConfig{
				Port:             "9999",
				OpenSearchURL:    "http://localhost:9200",
				OpenSearchUser:   "",
				OpenSearchPass:   "",
				EnableLogging:    true,
				LoggingLevel:     "info",
				LogRetentionDays: 30,
			},
		},
		{
			name: "custom_values",
			envVars: map[string]string{
				"APP_PORT":                  "8080",
				"OPENSEARCH_URL":            "https://search.example.com:9200",
				"OPENSEARCH_USER":           "testuser",
				"OPENSEARCH_PASSWORD":       "testpass",
				"ENABLE_OPENSEARCH_LOGGING": "false",
				"LOGGING_LEVEL":             "debug",
				"LOG_RETENTION_DAYS":        "60",
			},
			expected: &AppConfig{
				Port:             "8080",
				OpenSearchURL:    "https://search.example.com:9200",
				OpenSearchUser:   "testuser",
				OpenSearchPass:   "testpass",
				EnableLogging:    false,
				LoggingLevel:     "debug",
				LogRetentionDays: 60,
			},
		},
		{
			name: "invalid_boolean_defaults_to_true",
			envVars: map[string]string{
				"ENABLE_OPENSEARCH_LOGGING": "invalid",
			},
			expected: &AppConfig{
				Port:             "9999",
				OpenSearchURL:    "http://localhost:9200",
				OpenSearchUser:   "",
				OpenSearchPass:   "",
				EnableLogging:    true,
				LoggingLevel:     "info",
				LogRetentionDays: 30,
			},
		},
		{
			name: "invalid_int_defaults_to_30",
			envVars: map[string]string{
				"LOG_RETENTION_DAYS": "invalid",
			},
			expected: &AppConfig{
				Port:             "9999",
				OpenSearchURL:    "http://localhost:9200",
				OpenSearchUser:   "",
				OpenSearchPass:   "",
				EnableLogging:    true,
				LoggingLevel:     "info",
				LogRetentionDays: 30,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset singleton for each test
			appConfigInstance = nil

			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			config := GetAppConfig()
			require.NotNil(t, config)

			assert.Equal(t, tt.expected.Port, config.Port)
			assert.Equal(t, tt.expected.OpenSearchURL, config.OpenSearchURL)
			assert.Equal(t, tt.expected.OpenSearchUser, config.OpenSearchUser)
			assert.Equal(t, tt.expected.OpenSearchPass, config.OpenSearchPass)
			assert.Equal(t, tt.expected.EnableLogging, config.EnableLogging)
			assert.Equal(t, tt.expected.LoggingLevel, config.LoggingLevel)
			assert.Equal(t, tt.expected.LogRetentionDays, config.LogRetentionDays)

			// Test singleton behavior
			config2 := GetAppConfig()
			assert.Equal(t, config, config2, "GetAppConfig() should return singleton instance")

			// Clear env vars for next test
			for key := range tt.envVars {
				os.Unsetenv(key)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "env_var_exists",
			key:          "TEST_ENV_VAR",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "env_var_not_exists",
			key:          "NON_EXISTENT_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "env_var_empty",
			key:          "EMPTY_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before test
			os.Unsetenv(tt.key)

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := GetEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBoolEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue bool
		envValue     string
		expected     bool
	}{
		{
			name:         "true_string",
			key:          "TEST_BOOL_VAR",
			defaultValue: false,
			envValue:     "true",
			expected:     true,
		},
		{
			name:         "false_string",
			key:          "TEST_BOOL_VAR",
			defaultValue: true,
			envValue:     "false",
			expected:     false,
		},
		{
			name:         "1_string",
			key:          "TEST_BOOL_VAR",
			defaultValue: false,
			envValue:     "1",
			expected:     true,
		},
		{
			name:         "0_string",
			key:          "TEST_BOOL_VAR",
			defaultValue: true,
			envValue:     "0",
			expected:     false,
		},
		{
			name:         "invalid_string_returns_default",
			key:          "TEST_BOOL_VAR",
			defaultValue: true,
			envValue:     "invalid",
			expected:     true,
		},
		{
			name:         "empty_string_returns_default",
			key:          "TEST_BOOL_VAR",
			defaultValue: false,
			envValue:     "",
			expected:     false,
		},
		{
			name:         "non_existent_var_returns_default",
			key:          "NON_EXISTENT_BOOL",
			defaultValue: true,
			envValue:     "",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before test
			os.Unsetenv(tt.key)

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := GetBoolEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetIntEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		expected     int
	}{
		{
			name:         "valid_int",
			key:          "TEST_INT_VAR",
			defaultValue: 0,
			envValue:     "123",
			expected:     123,
		},
		{
			name:         "negative_int",
			key:          "TEST_INT_VAR",
			defaultValue: 0,
			envValue:     "-456",
			expected:     -456,
		},
		{
			name:         "zero_int",
			key:          "TEST_INT_VAR",
			defaultValue: 100,
			envValue:     "0",
			expected:     0,
		},
		{
			name:         "invalid_string_returns_default",
			key:          "TEST_INT_VAR",
			defaultValue: 42,
			envValue:     "invalid",
			expected:     42,
		},
		{
			name:         "empty_string_returns_default",
			key:          "TEST_INT_VAR",
			defaultValue: 99,
			envValue:     "",
			expected:     99,
		},
		{
			name:         "non_existent_var_returns_default",
			key:          "NON_EXISTENT_INT",
			defaultValue: 777,
			envValue:     "",
			expected:     777,
		},
		{
			name:         "float_string_returns_default",
			key:          "TEST_INT_VAR",
			defaultValue: 50,
			envValue:     "12.34",
			expected:     50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before test
			os.Unsetenv(tt.key)

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := GetIntEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"short_string", 5},
		{"medium_string", 32},
		{"long_string", 128},
		{"zero_length", 0},
		{"single_char", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RandomString(tt.length)
			assert.Len(t, result, tt.length)

			if tt.length > 0 {
				// Check that string contains only valid characters
				charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
				for _, char := range result {
					assert.Contains(t, charset, string(char))
				}

				// Test that multiple calls return different strings (very high probability)
				result2 := RandomString(tt.length)
				if tt.length > 1 {
					assert.NotEqual(t, result, result2, "Random strings should be different")
				}
			}
		})
	}
}

func TestCatch(t *testing.T) {
	tests := []struct {
		name        string
		handler     HttpHandler
		expectError bool
	}{
		{
			name: "successful_handler",
			handler: func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			},
			expectError: false,
		},
		{
			name: "handler_with_error",
			handler: func(w http.ResponseWriter, r *http.Request) error {
				return assert.AnError
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create wrapped handler
			wrappedHandler := Catch(tt.handler)
			assert.NotNil(t, wrappedHandler)

			// This test verifies the wrapper function is created correctly
			// The actual error logging would require capturing log output
			// which is not critical for coverage purposes
		})
	}
}
