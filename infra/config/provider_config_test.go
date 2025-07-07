package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProviderConfig(t *testing.T) {
	config := NewProviderConfig()
	defer config.Close()

	assert.NotNil(t, config)
	assert.NotNil(t, config.configs)
	assert.Equal(t, "http://localhost:9999", config.baseURL)
	// Storage might be nil if PostgreSQL connection fails (which is expected in test environment)
}

func TestProviderConfig_LoadFromEnv(t *testing.T) {
	// Save original env values
	originalEnvs := map[string]string{
		"APP_URL":             os.Getenv("APP_URL"),
		"IYZICO_APIKEY":       os.Getenv("IYZICO_APIKEY"),
		"IYZICO_SECRETKEY":    os.Getenv("IYZICO_SECRETKEY"),
		"IYZICO_ENVIRONMENT":  os.Getenv("IYZICO_ENVIRONMENT"),
		"OZANPAY_APIKEY":      os.Getenv("OZANPAY_APIKEY"),
		"OZANPAY_SECRETKEY":   os.Getenv("OZANPAY_SECRETKEY"),
		"OZANPAY_MERCHANTID":  os.Getenv("OZANPAY_MERCHANTID"),
		"OZANPAY_ENVIRONMENT": os.Getenv("OZANPAY_ENVIRONMENT"),
		"PAYCELL_USERNAME":    os.Getenv("PAYCELL_USERNAME"),
		"PAYCELL_PASSWORD":    os.Getenv("PAYCELL_PASSWORD"),
		"PAYCELL_MERCHANTID":  os.Getenv("PAYCELL_MERCHANTID"),
		"PAYCELL_TERMINALID":  os.Getenv("PAYCELL_TERMINALID"),
		"PAYCELL_ENVIRONMENT": os.Getenv("PAYCELL_ENVIRONMENT"),
		"NKOLAY_APIKEY":       os.Getenv("NKOLAY_APIKEY"),
		"NKOLAY_SECRETKEY":    os.Getenv("NKOLAY_SECRETKEY"),
		"NKOLAY_MERCHANTID":   os.Getenv("NKOLAY_MERCHANTID"),
		"NKOLAY_ENVIRONMENT":  os.Getenv("NKOLAY_ENVIRONMENT"),
		"PAPARA_APIKEY":       os.Getenv("PAPARA_APIKEY"),
		"PAPARA_ENVIRONMENT":  os.Getenv("PAPARA_ENVIRONMENT"),
	}

	// Clear all env vars
	for key := range originalEnvs {
		os.Unsetenv(key)
	}

	defer func() {
		// Restore original values
		for key, value := range originalEnvs {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	tests := []struct {
		name    string
		envVars map[string]string
	}{
		{
			name: "iyzico_config",
			envVars: map[string]string{
				"IYZICO_APIKEY":      "test-api-key",
				"IYZICO_SECRETKEY":   "test-secret-key",
				"IYZICO_ENVIRONMENT": "sandbox",
			},
		},
		{
			name: "ozanpay_config",
			envVars: map[string]string{
				"OZANPAY_APIKEY":      "test-api-key",
				"OZANPAY_SECRETKEY":   "test-secret-key",
				"OZANPAY_MERCHANTID":  "test-merchant",
				"OZANPAY_ENVIRONMENT": "sandbox",
			},
		},
		{
			name: "paycell_config",
			envVars: map[string]string{
				"PAYCELL_USERNAME":    "test-user",
				"PAYCELL_PASSWORD":    "test-pass",
				"PAYCELL_MERCHANTID":  "test-merchant",
				"PAYCELL_TERMINALID":  "test-terminal",
				"PAYCELL_ENVIRONMENT": "sandbox",
			},
		},
		{
			name: "multiple_providers",
			envVars: map[string]string{
				"IYZICO_APIKEY":    "iyzico-key",
				"IYZICO_SECRETKEY": "iyzico-secret",
				"PAPARA_APIKEY":    "papara-key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			config := NewProviderConfig()
			defer config.Close()

			config.LoadFromEnv()

			// Clear env vars for next test
			for key := range tt.envVars {
				os.Unsetenv(key)
			}
		})
	}
}

func TestProviderConfig_SetTenantConfig(t *testing.T) {
	config := NewProviderConfig()
	defer config.Close()

	tests := []struct {
		name         string
		tenantID     string
		providerName string
		configData   map[string]string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "valid_iyzico_config",
			tenantID:     "APP1",
			providerName: "iyzico",
			configData: map[string]string{
				"apiKey":      "test-key",
				"secretKey":   "test-secret",
				"environment": "sandbox",
			},
			expectError: false,
		},
		{
			name:         "valid_ozanpay_config",
			tenantID:     "APP2",
			providerName: "ozanpay",
			configData: map[string]string{
				"apiKey":      "test-key",
				"secretKey":   "test-secret",
				"merchantId":  "test-merchant",
				"environment": "production",
			},
			expectError: false,
		},
		{
			name:         "empty_tenant_id",
			tenantID:     "",
			providerName: "iyzico",
			configData: map[string]string{
				"apiKey":    "test-key",
				"secretKey": "test-secret",
			},
			expectError: true,
			errorMsg:    "tenant ID cannot be empty",
		},
		{
			name:         "empty_provider_name",
			tenantID:     "APP1",
			providerName: "",
			configData: map[string]string{
				"apiKey":    "test-key",
				"secretKey": "test-secret",
			},
			expectError: true,
			errorMsg:    "provider name cannot be empty",
		},
		{
			name:         "empty_config",
			tenantID:     "APP1",
			providerName: "iyzico",
			configData:   map[string]string{},
			expectError:  true,
			errorMsg:     "config cannot be empty",
		},
		{
			name:         "missing_required_keys",
			tenantID:     "APP1",
			providerName: "iyzico",
			configData: map[string]string{
				"apiKey": "test-key",
				// Missing secretKey
			},
			expectError: true,
			errorMsg:    "required key 'secretKey' is missing or empty",
		},
		{
			name:         "unsupported_provider",
			tenantID:     "APP1",
			providerName: "unknown",
			configData: map[string]string{
				"someKey": "someValue",
			},
			expectError: true,
			errorMsg:    "unsupported provider: unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.SetTenantConfig(tt.tenantID, tt.providerName, tt.configData)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)

				// Verify config was saved
				savedConfig, err := config.GetTenantConfig(tt.tenantID, tt.providerName)
				require.NoError(t, err)
				assert.Equal(t, tt.configData, savedConfig)
			}
		})
	}
}

func TestProviderConfig_GetTenantConfig(t *testing.T) {
	config := NewProviderConfig()
	defer config.Close()

	// Set up test data
	testConfig := map[string]string{
		"apiKey":      "test-key",
		"secretKey":   "test-secret",
		"environment": "sandbox",
	}

	err := config.SetTenantConfig("APP1", "iyzico", testConfig)
	require.NoError(t, err)

	tests := []struct {
		name         string
		tenantID     string
		providerName string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "existing_config",
			tenantID:     "APP1",
			providerName: "iyzico",
			expectError:  false,
		},
		{
			name:         "non_existing_config",
			tenantID:     "APP2",
			providerName: "iyzico",
			expectError:  true,
			errorMsg:     "no configuration found for tenant: APP2, provider: iyzico",
		},
		{
			name:         "empty_tenant_id",
			tenantID:     "",
			providerName: "iyzico",
			expectError:  true,
			errorMsg:     "tenant ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := config.GetTenantConfig(tt.tenantID, tt.providerName)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testConfig, result)

				// Verify config is a copy (not the original)
				result["newKey"] = "newValue"
				originalConfig, _ := config.GetTenantConfig(tt.tenantID, tt.providerName)
				_, exists := originalConfig["newKey"]
				assert.False(t, exists, "Config should be a copy, not reference")
			}
		})
	}
}

func TestProviderConfig_GetAvailableTenantsForProvider(t *testing.T) {
	config := NewProviderConfig()
	defer config.Close()

	// Set up test data
	testConfigs := []struct {
		tenant   string
		provider string
		config   map[string]string
	}{
		{
			tenant:   "APP1",
			provider: "iyzico",
			config:   map[string]string{"apiKey": "key1", "secretKey": "secret1"},
		},
		{
			tenant:   "APP2",
			provider: "iyzico",
			config:   map[string]string{"apiKey": "key2", "secretKey": "secret2"},
		},
		{
			tenant:   "APP1",
			provider: "ozanpay",
			config:   map[string]string{"apiKey": "key3", "secretKey": "secret3", "merchantId": "merchant1"},
		},
	}

	for _, tc := range testConfigs {
		err := config.SetTenantConfig(tc.tenant, tc.provider, tc.config)
		require.NoError(t, err)
	}

	tests := []struct {
		name         string
		providerName string
		expected     []string
	}{
		{
			name:         "iyzico_tenants",
			providerName: "iyzico",
			expected:     []string{"app1", "app2"},
		},
		{
			name:         "ozanpay_tenants",
			providerName: "ozanpay",
			expected:     []string{"app1"},
		},
		{
			name:         "non_existing_provider",
			providerName: "nonexistent",
			expected:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetAvailableTenantsForProvider(tt.providerName)

			if len(tt.expected) == 0 {
				assert.Empty(t, result)
			} else {
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

func TestProviderConfig_DeleteTenantConfig(t *testing.T) {
	config := NewProviderConfig()
	defer config.Close()

	// Set up test data
	testConfig := map[string]string{
		"apiKey":    "test-key",
		"secretKey": "test-secret",
	}

	err := config.SetTenantConfig("APP1", "iyzico", testConfig)
	require.NoError(t, err)

	tests := []struct {
		name         string
		tenantID     string
		providerName string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "delete_existing_config",
			tenantID:     "APP1",
			providerName: "iyzico",
			expectError:  false,
		},
		{
			name:         "empty_tenant_id",
			tenantID:     "",
			providerName: "iyzico",
			expectError:  true,
			errorMsg:     "tenant ID cannot be empty",
		},
		{
			name:         "empty_provider_name",
			tenantID:     "APP1",
			providerName: "",
			expectError:  true,
			errorMsg:     "provider name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.DeleteTenantConfig(tt.tenantID, tt.providerName)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)

				// Verify config was deleted
				_, err := config.GetTenantConfig(tt.tenantID, tt.providerName)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no configuration found")
			}
		})
	}
}

func TestProviderConfig_validateProviderConfig(t *testing.T) {
	config := &ProviderConfig{}

	tests := []struct {
		name         string
		providerName string
		configData   map[string]string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "valid_iyzico",
			providerName: "iyzico",
			configData: map[string]string{
				"apiKey":    "test-key",
				"secretKey": "test-secret",
			},
			expectError: false,
		},
		{
			name:         "valid_ozanpay",
			providerName: "ozanpay",
			configData: map[string]string{
				"apiKey":     "test-key",
				"secretKey":  "test-secret",
				"merchantId": "test-merchant",
			},
			expectError: false,
		},
		{
			name:         "valid_paycell",
			providerName: "paycell",
			configData: map[string]string{
				"username":   "test-user",
				"password":   "test-pass",
				"merchantId": "test-merchant",
				"terminalId": "test-terminal",
			},
			expectError: false,
		},
		{
			name:         "valid_nkolay",
			providerName: "nkolay",
			configData: map[string]string{
				"apiKey":     "test-key",
				"secretKey":  "test-secret",
				"merchantId": "test-merchant",
			},
			expectError: false,
		},
		{
			name:         "valid_papara",
			providerName: "papara",
			configData: map[string]string{
				"apiKey": "test-key",
			},
			expectError: false,
		},
		{
			name:         "unsupported_provider",
			providerName: "unknown",
			configData: map[string]string{
				"someKey": "someValue",
			},
			expectError: true,
			errorMsg:    "unsupported provider: unknown",
		},
		{
			name:         "missing_required_key",
			providerName: "iyzico",
			configData: map[string]string{
				"apiKey": "test-key",
				// Missing secretKey
			},
			expectError: true,
			errorMsg:    "required key 'secretKey' is missing or empty",
		},
		{
			name:         "empty_required_value",
			providerName: "iyzico",
			configData: map[string]string{
				"apiKey":    "test-key",
				"secretKey": "   ", // Empty/whitespace value
			},
			expectError: true,
			errorMsg:    "required key 'secretKey' is missing or empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.validateProviderConfig(tt.providerName, tt.configData)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProviderConfig_GetBaseURL(t *testing.T) {
	// Test default base URL
	config := NewProviderConfig()
	defer config.Close()

	assert.Equal(t, "http://localhost:9999", config.GetBaseURL())

	// Test custom base URL
	os.Setenv("APP_URL", "https://api.example.com")
	defer os.Unsetenv("APP_URL")

	config2 := NewProviderConfig()
	defer config2.Close()

	assert.Equal(t, "https://api.example.com", config2.GetBaseURL())
}

func TestProviderConfig_GetStats(t *testing.T) {
	config := NewProviderConfig()
	defer config.Close()

	// Add some test data
	testConfig := map[string]string{"apiKey": "test", "secretKey": "test"}
	err := config.SetTenantConfig("APP1", "iyzico", testConfig)
	require.NoError(t, err)

	stats, err := config.GetStats()
	require.NoError(t, err)

	assert.Contains(t, stats, "memory_configs")
	assert.Contains(t, stats, "base_url")
	// PostgreSQL storage is expected to fail in test environment, so we check for postgres key
	assert.Contains(t, stats, "postgres")

	assert.Equal(t, 1, stats["memory_configs"])
	assert.Equal(t, "http://localhost:9999", stats["base_url"])
}

func TestProviderConfig_LegacyMethods(t *testing.T) {
	config := NewProviderConfig()
	defer config.Close()

	// Test GetConfig (legacy method)
	_, err := config.GetConfig("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no configuration found for provider: nonexistent")

	// Test GetAvailableProviders (legacy method)
	providers := config.GetAvailableProviders()
	assert.IsType(t, []string{}, providers)
}
