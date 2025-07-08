package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProviderConfig(t *testing.T) {
	config := NewProviderConfig()

	assert.NotNil(t, config)
	assert.NotNil(t, config.configs)
	// Storage might be nil if PostgreSQL connection fails (which is expected in test environment)
}

func TestProviderConfig_SetTenantConfig(t *testing.T) {
	config := NewProviderConfig()

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

func TestProviderConfig_DeleteTenantConfig(t *testing.T) {
	config := NewProviderConfig()

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

func TestProviderConfig_GetStats(t *testing.T) {
	config := NewProviderConfig()

	// Set up some test config
	testConfig := map[string]string{
		"apiKey":    "test-key",
		"secretKey": "test-secret",
	}

	err := config.SetTenantConfig("APP1", "iyzico", testConfig)
	require.NoError(t, err)

	stats, err := config.GetStats()
	require.NoError(t, err)

	assert.Contains(t, stats, "memory_configs")
	assert.Equal(t, 1, stats["memory_configs"])

	// PostgreSQL stats will depend on whether storage is available
	assert.Contains(t, stats, "postgres")
}
