package stripe

import (
	"strings"
	"testing"
)

func TestStripeProvider_GetRequiredConfig(t *testing.T) {
	provider := NewProvider().(*StripeProvider)

	tests := []struct {
		name        string
		environment string
		expected    int
	}{
		{"sandbox environment", "sandbox", 3},
		{"production environment", "production", 3},
		{"test environment", "test", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetRequiredConfig(tt.environment)
			if len(result) != tt.expected {
				t.Errorf("GetRequiredConfig() returned %d fields, want %d", len(result), tt.expected)
			}

			// Check required fields
			expectedFields := []string{"secretKey", "publicKey", "environment"}
			for i, field := range result {
				if field.Key != expectedFields[i] {
					t.Errorf("Expected field %s, got %s", expectedFields[i], field.Key)
				}
				// Only secretKey and environment should be required, publicKey is optional
				if field.Key == "publicKey" {
					if field.Required {
						t.Errorf("Field %s should be optional", field.Key)
					}
				} else {
					if !field.Required {
						t.Errorf("Field %s should be required", field.Key)
					}
				}
				if field.Type != "string" {
					t.Errorf("Field %s should be string type", field.Key)
				}
			}
		})
	}
}

func TestStripeProvider_ValidateConfig(t *testing.T) {
	provider := NewProvider().(*StripeProvider)

	tests := []struct {
		name        string
		config      map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid sandbox config",
			config: map[string]string{
				"secretKey":   "sk_test_4eC39HqLyjWDarjtT1zdp7dc123456789012345678901234567890123456789012345678901234567890123456789012345",
				"publicKey":   "pk_test_TYooMQauvdEDq54NiTphI7jx123456789012345678901234567890123456789012345678901234567890123456789012345",
				"environment": "sandbox",
			},
			expectError: false,
		},
		{
			name: "valid production config",
			config: map[string]string{
				"secretKey":   "sk_live_4eC39HqLyjWDarjtT1zdp7dc123456789012345678901234567890123456789012345678901234567890123456789012345",
				"publicKey":   "pk_live_TYooMQauvdEDq54NiTphI7jx123456789012345678901234567890123456789012345678901234567890123456789012345",
				"environment": "production",
			},
			expectError: false,
		},
		{
			name: "valid config without optional publicKey",
			config: map[string]string{
				"secretKey":   "sk_test_4eC39HqLyjWDarjtT1zdp7dc123456789012345678901234567890123456789012345678901234567890123456789012345",
				"environment": "sandbox",
			},
			expectError: false,
		},
		{
			name: "missing secretKey",
			config: map[string]string{
				"publicKey":   "pk_test_TYooMQauvdEDq54NiTphI7jx123456789012345678901234567890123456789012345678901234567890123456789012345",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'secretKey' is missing",
		},
		{
			name: "missing environment",
			config: map[string]string{
				"secretKey": "sk_test_4eC39HqLyjWDarjtT1zdp7dc123456789012345678901234567890123456789012345678901234567890123456789012345",
				"publicKey": "pk_test_TYooMQauvdEDq54NiTphI7jx123456789012345678901234567890123456789012345678901234567890123456789012345",
			},
			expectError: true,
			errorMsg:    "required field 'environment' is missing",
		},
		{
			name: "empty secretKey",
			config: map[string]string{
				"secretKey":   "",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'secretKey' cannot be empty",
		},
		{
			name: "invalid environment",
			config: map[string]string{
				"secretKey":   "sk_test_4eC39HqLyjWDarjtT1zdp7dc123456789012345678901234567890123456789012345678901234567890123456789012345",
				"environment": "invalid_env",
			},
			expectError: true,
			errorMsg:    "environment must be one of",
		},
		{
			name: "secretKey too short",
			config: map[string]string{
				"secretKey":   "sk_test_short",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 107 characters",
		},
		{
			name: "secretKey invalid prefix",
			config: map[string]string{
				"secretKey":   "invalid_4eC39HqLyjWDarjtT1zdp7dc123456789012345678901234567890123456789012345678901234567890123456789012345",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "does not match required pattern",
		},
		{
			name: "publicKey invalid prefix",
			config: map[string]string{
				"secretKey":   "sk_test_4eC39HqLyjWDarjtT1zdp7dc123456789012345678901234567890123456789012345678901234567890123456789012345",
				"publicKey":   "invalid_TYooMQauvdEDq54NiTphI7jx123456789012345678901234567890123456789012345678901234567890123456789012345",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "public key must start with 'pk_test_' or 'pk_live_'",
		},
		{
			name: "secretKey with wrong environment prefix (live key with sandbox env)",
			config: map[string]string{
				"secretKey":   "sk_live_4eC39HqLyjWDarjtT1zdp7dc123456789012345678901234567890123456789012345678901234567890123456789012345",
				"environment": "sandbox",
			},
			expectError: false, // This should be valid, environment is separate from key type
		},
		{
			name: "publicKey too short",
			config: map[string]string{
				"secretKey":   "sk_test_4eC39HqLyjWDarjtT1zdp7dc123456789012345678901234567890123456789012345678901234567890123456789012345",
				"publicKey":   "pk_test_short",
				"environment": "sandbox",
			},
			expectError: false, // publicKey is optional, so short keys might be allowed
		},
		{
			name: "secretKey pattern mismatch",
			config: map[string]string{
				"secretKey":   "sk_test_4eC39HqLyjWDarjtT1zdp7dc!@#$%^&*()123456789012345678901234567890123456789012345678901234567890123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "does not match required pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %s", err.Error())
				}
			}
		})
	}
}
