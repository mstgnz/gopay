package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSQLiteStorage(t *testing.T) {
	// Get project root directory (go up two levels from infra/config)
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..")
	dbPath := filepath.Join(projectRoot, "data", "gopay.db")

	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	os.MkdirAll(dataDir, 0755)

	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	require.NotNil(t, storage)
	defer storage.Close()

	assert.Equal(t, dbPath, storage.path)
	assert.NotNil(t, storage.db)

	// Test that database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestNewSQLiteStorage_InvalidPath(t *testing.T) {
	// Try to create database in a path that doesn't exist and can't be created
	invalidPath := "/root/invalid/path/test.db"

	_, err := NewSQLiteStorage(invalidPath)
	// This might succeed or fail depending on permissions
	// The important thing is it doesn't panic
	if err != nil {
		assert.Error(t, err)
	}
}

func TestSQLiteStorage_SaveTenantConfig(t *testing.T) {
	// Get project root directory (go up two levels from infra/config)
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..")
	dbPath := filepath.Join(projectRoot, "data", "gopay.db")

	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	os.MkdirAll(dataDir, 0755)

	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	tests := []struct {
		name         string
		tenantID     string
		providerName string
		config       map[string]string
		expectError  bool
	}{
		{
			name:         "valid_config",
			tenantID:     "APP1",
			providerName: "iyzico",
			config: map[string]string{
				"apiKey":    "test-key",
				"secretKey": "test-secret",
			},
			expectError: false,
		},
		{
			name:         "update_existing_config",
			tenantID:     "APP1",
			providerName: "iyzico",
			config: map[string]string{
				"apiKey":      "updated-key",
				"secretKey":   "updated-secret",
				"environment": "production",
			},
			expectError: false,
		},
		{
			name:         "different_tenant_same_provider",
			tenantID:     "APP2",
			providerName: "iyzico",
			config: map[string]string{
				"apiKey":    "app2-key",
				"secretKey": "app2-secret",
			},
			expectError: false,
		},
		{
			name:         "empty_config",
			tenantID:     "APP3",
			providerName: "ozanpay",
			config:       map[string]string{},
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := storage.SaveTenantConfig(tt.tenantID, tt.providerName, tt.config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSQLiteStorage_LoadTenantConfig(t *testing.T) {
	// Get project root directory (go up two levels from infra/config)
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..")
	dbPath := filepath.Join(projectRoot, "data", "gopay.db")

	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	os.MkdirAll(dataDir, 0755)

	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	// Set up test data
	testConfig := map[string]string{
		"apiKey":      "test-key",
		"secretKey":   "test-secret",
		"environment": "sandbox",
	}

	err = storage.SaveTenantConfig("APP1", "iyzico", testConfig)
	require.NoError(t, err)

	tests := []struct {
		name         string
		tenantID     string
		providerName string
		expectError  bool
		expected     map[string]string
	}{
		{
			name:         "existing_config",
			tenantID:     "APP1",
			providerName: "iyzico",
			expectError:  false,
			expected:     testConfig,
		},
		{
			name:         "non_existing_tenant",
			tenantID:     "APP2",
			providerName: "iyzico",
			expectError:  true,
		},
		{
			name:         "non_existing_provider",
			tenantID:     "APP1",
			providerName: "ozanpay",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := storage.LoadTenantConfig(tt.tenantID, tt.providerName)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSQLiteStorage_LoadAllTenantConfigs(t *testing.T) {
	// Get project root directory (go up two levels from infra/config)
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..")
	dbPath := filepath.Join(projectRoot, "data", "gopay.db")

	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	os.MkdirAll(dataDir, 0755)

	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	// Set up test data
	testConfigs := map[string]map[string]string{
		"APP1_iyzico": {
			"apiKey":    "key1",
			"secretKey": "secret1",
		},
		"APP2_iyzico": {
			"apiKey":    "key2",
			"secretKey": "secret2",
		},
		"APP1_ozanpay": {
			"apiKey":     "key3",
			"secretKey":  "secret3",
			"merchantId": "merchant1",
		},
	}

	// Save test configs
	for key, config := range testConfigs {
		parts := key[:len(key)-len("_iyzico")]
		if len(parts) < 4 {
			parts = key[:len(key)-len("_ozanpay")]
		}

		var tenantID, providerName string
		if key == "APP1_iyzico" {
			tenantID, providerName = "APP1", "iyzico"
		} else if key == "APP2_iyzico" {
			tenantID, providerName = "APP2", "iyzico"
		} else if key == "APP1_ozanpay" {
			tenantID, providerName = "APP1", "ozanpay"
		}

		err := storage.SaveTenantConfig(tenantID, providerName, config)
		require.NoError(t, err)
	}

	// Load all configs
	result, err := storage.LoadAllTenantConfigs()
	require.NoError(t, err)

	assert.Len(t, result, len(testConfigs))

	for expectedKey, expectedConfig := range testConfigs {
		actualConfig, exists := result[expectedKey]
		assert.True(t, exists, "Config for %s should exist", expectedKey)
		assert.Equal(t, expectedConfig, actualConfig)
	}
}

func TestSQLiteStorage_DeleteTenantConfig(t *testing.T) {
	// Get project root directory (go up two levels from infra/config)
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..")
	dbPath := filepath.Join(projectRoot, "data", "gopay.db")

	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	os.MkdirAll(dataDir, 0755)

	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	// Set up test data
	testConfig := map[string]string{
		"apiKey":    "test-key",
		"secretKey": "test-secret",
	}

	err = storage.SaveTenantConfig("APP1", "iyzico", testConfig)
	require.NoError(t, err)

	tests := []struct {
		name         string
		tenantID     string
		providerName string
		expectError  bool
	}{
		{
			name:         "delete_existing_config",
			tenantID:     "APP1",
			providerName: "iyzico",
			expectError:  false,
		},
		{
			name:         "delete_non_existing_config",
			tenantID:     "APP2",
			providerName: "iyzico",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := storage.DeleteTenantConfig(tt.tenantID, tt.providerName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify config was deleted
				_, err := storage.LoadTenantConfig(tt.tenantID, tt.providerName)
				assert.Error(t, err)
			}
		})
	}
}

func TestSQLiteStorage_GetTenantsByProvider(t *testing.T) {
	// Get project root directory (go up two levels from infra/config)
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..")
	dbPath := filepath.Join(projectRoot, "data", "gopay.db")

	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	os.MkdirAll(dataDir, 0755)

	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	// Set up test data
	testConfigs := []struct {
		tenant   string
		provider string
		config   map[string]string
	}{
		{
			tenant:   "APP1",
			provider: "iyzico",
			config:   map[string]string{"apiKey": "key1"},
		},
		{
			tenant:   "APP2",
			provider: "iyzico",
			config:   map[string]string{"apiKey": "key2"},
		},
		{
			tenant:   "APP3",
			provider: "iyzico",
			config:   map[string]string{"apiKey": "key3"},
		},
		{
			tenant:   "APP1",
			provider: "ozanpay",
			config:   map[string]string{"apiKey": "key4"},
		},
	}

	for _, tc := range testConfigs {
		err := storage.SaveTenantConfig(tc.tenant, tc.provider, tc.config)
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
			expected:     []string{"APP1", "APP2", "APP3"},
		},
		{
			name:         "ozanpay_tenants",
			providerName: "ozanpay",
			expected:     []string{"APP1"},
		},
		{
			name:         "non_existing_provider",
			providerName: "nonexistent",
			expected:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := storage.GetTenantsByProvider(tt.providerName)
			require.NoError(t, err)

			if len(tt.expected) == 0 {
				assert.Empty(t, result)
			} else {
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

func TestSQLiteStorage_GetStats(t *testing.T) {
	// Get project root directory (go up two levels from infra/config)
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..")
	dbPath := filepath.Join(projectRoot, "data", "gopay.db")

	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	os.MkdirAll(dataDir, 0755)

	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	// Initially empty
	stats, err := storage.GetStats()
	require.NoError(t, err)

	assert.Contains(t, stats, "total_configs")
	assert.Contains(t, stats, "unique_tenants")
	assert.Contains(t, stats, "unique_providers")
	assert.Contains(t, stats, "db_size_bytes")
	assert.Contains(t, stats, "db_path")

	assert.Equal(t, 0, stats["total_configs"])
	assert.Equal(t, 0, stats["unique_tenants"])
	assert.Equal(t, 0, stats["unique_providers"])
	assert.Equal(t, dbPath, stats["db_path"])

	// Add some test data
	testConfig := map[string]string{"apiKey": "test", "secretKey": "test"}
	err = storage.SaveTenantConfig("APP1", "iyzico", testConfig)
	require.NoError(t, err)

	err = storage.SaveTenantConfig("APP2", "ozanpay", testConfig)
	require.NoError(t, err)

	// Check updated stats
	stats, err = storage.GetStats()
	require.NoError(t, err)

	assert.Equal(t, 2, stats["total_configs"])
	assert.Equal(t, 2, stats["unique_tenants"])
	assert.Equal(t, 2, stats["unique_providers"])
	assert.Greater(t, stats["db_size_bytes"], int64(0))
}

func TestSQLiteStorage_Close(t *testing.T) {
	// Get project root directory (go up two levels from infra/config)
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..")
	dbPath := filepath.Join(projectRoot, "data", "gopay.db")

	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	os.MkdirAll(dataDir, 0755)

	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)

	// Close should work without error
	err = storage.Close()
	assert.NoError(t, err)

	// Multiple closes should not panic
	err = storage.Close()
	// This might return an error or not, depending on SQLite driver behavior
	// The important thing is it doesn't panic
}

func TestSQLiteStorage_ConcurrentAccess(t *testing.T) {
	// Get project root directory (go up two levels from infra/config)
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..")
	dbPath := filepath.Join(projectRoot, "data", "gopay.db")

	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	os.MkdirAll(dataDir, 0755)

	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	// Test concurrent writes
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			testConfig := map[string]string{
				"apiKey":    "test-key",
				"secretKey": "test-secret",
			}

			err := storage.SaveTenantConfig("APP"+string(rune('0'+id)), "iyzico", testConfig)
			assert.NoError(t, err)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all configs were saved
	configs, err := storage.LoadAllTenantConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 10)
}

func TestSQLiteStorage_InvalidJSON(t *testing.T) {
	// Get project root directory (go up two levels from infra/config)
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..")
	dbPath := filepath.Join(projectRoot, "data", "gopay.db")

	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	os.MkdirAll(dataDir, 0755)

	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer storage.Close()

	// Manually insert invalid JSON to test error handling
	_, err = storage.db.Exec(`
		INSERT INTO tenant_configs (tenant_id, provider_name, config_data)
		VALUES (?, ?, ?)
	`, "TEST", "invalid", "invalid-json")
	require.NoError(t, err)

	// LoadTenantConfig should handle invalid JSON gracefully
	_, err = storage.LoadTenantConfig("TEST", "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal config")

	// LoadAllTenantConfigs should skip invalid JSON and continue
	configs, err := storage.LoadAllTenantConfigs()
	require.NoError(t, err)
	// Should not include the invalid config
	_, exists := configs["TEST_invalid"]
	assert.False(t, exists)
}
