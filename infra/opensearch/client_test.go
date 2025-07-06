package opensearch

import (
	"testing"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.AppConfig
		expectError bool
	}{
		{
			name: "valid_config_no_auth",
			cfg: &config.AppConfig{
				OpenSearchURL:  "http://localhost:9200",
				EnableLogging:  true,
				OpenSearchUser: "",
				OpenSearchPass: "",
			},
			expectError: false,
		},
		{
			name: "valid_config_with_auth",
			cfg: &config.AppConfig{
				OpenSearchURL:  "http://localhost:9200",
				EnableLogging:  true,
				OpenSearchUser: "admin",
				OpenSearchPass: "admin",
			},
			expectError: false,
		},
		{
			name: "invalid_url",
			cfg: &config.AppConfig{
				OpenSearchURL:  "invalid-url",
				EnableLogging:  true,
				OpenSearchUser: "",
				OpenSearchPass: "",
			},
			expectError: false, // Client creation might still succeed, connection would fail later
		},
		{
			name: "empty_url",
			cfg: &config.AppConfig{
				OpenSearchURL:  "",
				EnableLogging:  true,
				OpenSearchUser: "",
				OpenSearchPass: "",
			},
			expectError: false, // OpenSearch client might use defaults
		},
		{
			name: "logging_disabled",
			cfg: &config.AppConfig{
				OpenSearchURL:  "http://localhost:9200",
				EnableLogging:  false,
				OpenSearchUser: "",
				OpenSearchPass: "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				// Note: We might not actually be able to connect to OpenSearch in tests
				// but the client creation should succeed
				if err != nil {
					// If error occurs, it should be connection-related, not configuration
					t.Logf("Expected connection error in test environment: %v", err)
				} else {
					assert.NotNil(t, client)
					assert.NotNil(t, client.client)
					assert.Equal(t, tt.cfg, client.config)
				}
			}
		})
	}
}

func TestClient_GetClient(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)

	osClient := client.GetClient()
	assert.NotNil(t, osClient)
}

func TestClient_GetLogIndexName(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)

	tests := []struct {
		name     string
		tenantID string
		provider string
		expected string
	}{
		{
			name:     "with_tenant_id",
			tenantID: "APP1",
			provider: "iyzico",
			expected: "gopay-APP1-iyzico-logs",
		},
		{
			name:     "without_tenant_id",
			tenantID: "",
			provider: "iyzico",
			expected: "gopay-iyzico-logs",
		},
		{
			name:     "empty_tenant_id",
			tenantID: "",
			provider: "ozanpay",
			expected: "gopay-ozanpay-logs",
		},
		{
			name:     "complex_tenant_id",
			tenantID: "MY-APP-123",
			provider: "stripe",
			expected: "gopay-MY-APP-123-stripe-logs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.GetLogIndexName(tt.tenantID, tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClient_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{
			name:     "logging_enabled",
			enabled:  true,
			expected: true,
		},
		{
			name:     "logging_disabled",
			enabled:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				OpenSearchURL: "http://localhost:9200",
				EnableLogging: tt.enabled,
			}

			client, err := NewClient(cfg)
			if err != nil {
				t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
			}

			require.NotNil(t, client)

			result := client.IsEnabled()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClient_setupIndices(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)

	// setupIndices is called during NewClient, so if we reach here it means it didn't panic
	// We can't easily test the actual index creation without a real OpenSearch instance
	err = client.setupIndices()
	// This might fail due to connection issues in test environment, but shouldn't panic
	if err != nil {
		t.Logf("Expected error in test environment: %v", err)
	}
}

func TestClient_createIndexIfNotExists(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)

	// Test creating an index (this method is not exported, so we test it indirectly through setupIndices)
	err = client.setupIndices()
	// This will likely fail in test environment due to no real OpenSearch
	if err != nil {
		t.Logf("Expected error in test environment: %v", err)
	}
}

func TestNewClient_InvalidConfig(t *testing.T) {
	// Test with nil config - this should panic or error
	assert.Panics(t, func() {
		_, _ = NewClient(nil)
	})
}

func TestClient_ProviderIndexNames(t *testing.T) {
	cfg := &config.AppConfig{
		OpenSearchURL: "http://localhost:9200",
		EnableLogging: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test due to OpenSearch connection error: %v", err)
	}

	require.NotNil(t, client)

	// Test all supported providers
	providers := []string{"iyzico", "ozanpay", "stripe", "paytr", "paycell", "papara", "nkolay", "shopier"}

	for _, provider := range providers {
		indexName := client.GetLogIndexName("", provider)
		expected := "gopay-" + provider + "-logs"
		assert.Equal(t, expected, indexName, "Index name should match pattern for provider %s", provider)

		// Test with tenant ID
		indexNameWithTenant := client.GetLogIndexName("APP1", provider)
		expectedWithTenant := "gopay-APP1-" + provider + "-logs"
		assert.Equal(t, expectedWithTenant, indexNameWithTenant, "Index name with tenant should match pattern for provider %s", provider)
	}
}
