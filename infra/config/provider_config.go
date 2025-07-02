package config

import (
	"fmt"
	"strings"
	"sync"
)

// ProviderConfig manages payment provider configurations
type ProviderConfig struct {
	configs map[string]map[string]string
	baseURL string
	storage *SQLiteStorage // SQLite storage for persistence
	mu      sync.RWMutex   // Thread-safe access için mutex ekliyorum
}

// NewProviderConfig creates a new provider configuration
func NewProviderConfig() *ProviderConfig {
	// Get database path from environment variable or use default
	dbPath := GetEnv("SQLITE_DB_PATH", "./data/gopay.db")

	// Initialize SQLite storage
	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		// Fallback to memory-only mode if SQLite fails
		fmt.Printf("Warning: Failed to initialize SQLite storage (%v), falling back to memory-only mode\n", err)
	}

	config := &ProviderConfig{
		configs: make(map[string]map[string]string),
		baseURL: GetEnv("APP_URL", "http://localhost:9999"),
		storage: storage,
	}

	// Load existing configurations from SQLite if available
	if storage != nil {
		if err := config.loadFromSQLite(); err != nil {
			fmt.Printf("Warning: Failed to load configurations from SQLite: %v\n", err)
		}
	}

	return config
}

// LoadFromEnv loads provider configurations from environment variables
// using the pattern PROVIDER_NAME_KEY=value
func (c *ProviderConfig) LoadFromEnv() {
	// Load base URL configuration for callback URLs
	c.baseURL = GetEnv("APP_URL", "http://localhost:9999")

	// Load Iyzico configuration
	c.loadProviderFromEnv("iyzico", []string{
		"apiKey",
		"secretKey",
		"environment",
	})

	// Load OzanPay configuration
	c.loadProviderFromEnv("ozanpay", []string{
		"apiKey",
		"secretKey",
		"merchantId",
		"environment",
	})

	// Load Paycell configuration
	c.loadProviderFromEnv("paycell", []string{
		"username",
		"password",
		"merchantId",
		"terminalId",
		"environment",
	})
}

// loadFromSQLite loads all tenant configurations from SQLite storage
func (c *ProviderConfig) loadFromSQLite() error {
	if c.storage == nil {
		return fmt.Errorf("SQLite storage not initialized")
	}

	configs, err := c.storage.LoadAllTenantConfigs()
	if err != nil {
		return fmt.Errorf("failed to load configs from SQLite: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Merge SQLite configs with in-memory configs
	for key, config := range configs {
		c.configs[key] = config
	}

	return nil
}

// SetTenantConfig dynamically sets configuration for a specific tenant and provider
func (c *ProviderConfig) SetTenantConfig(tenantID, providerName string, config map[string]string) error {
	if tenantID == "" {
		return fmt.Errorf("tenant ID cannot be empty")
	}
	if providerName == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if len(config) == 0 {
		return fmt.Errorf("config cannot be empty")
	}

	// Validate required keys based on provider
	if err := c.validateProviderConfig(providerName, config); err != nil {
		return fmt.Errorf("invalid config for provider %s: %w", providerName, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Create tenant-specific provider key
	tenantProviderKey := fmt.Sprintf("%s_%s", strings.ToUpper(tenantID), strings.ToLower(providerName))

	// Save to SQLite if available
	if c.storage != nil {
		if err := c.storage.SaveTenantConfig(tenantID, providerName, config); err != nil {
			return fmt.Errorf("failed to save config to SQLite: %w", err)
		}
	}

	// Update in-memory cache
	c.configs[tenantProviderKey] = config
	return nil
}

// GetTenantConfig returns configuration for a specific tenant and provider
func (c *ProviderConfig) GetTenantConfig(tenantID, providerName string) (map[string]string, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant ID cannot be empty")
	}

	c.mu.RLock()
	// Create tenant-specific provider key
	tenantProviderKey := fmt.Sprintf("%s_%s", strings.ToUpper(tenantID), strings.ToLower(providerName))

	config, exists := c.configs[tenantProviderKey]
	c.mu.RUnlock()

	// If not found in memory, try loading from SQLite
	if !exists && c.storage != nil {
		sqliteConfig, err := c.storage.LoadTenantConfig(tenantID, providerName)
		if err == nil {
			// Cache in memory for future use
			c.mu.Lock()
			c.configs[tenantProviderKey] = sqliteConfig
			c.mu.Unlock()
			config = sqliteConfig
			exists = true
		}
	}

	if !exists {
		return nil, fmt.Errorf("no configuration found for tenant: %s, provider: %s", tenantID, providerName)
	}

	// Return a copy to prevent external modification
	configCopy := make(map[string]string)
	for k, v := range config {
		configCopy[k] = v
	}

	return configCopy, nil
}

// GetAvailableTenantsForProvider returns all tenants that have configuration for a specific provider
func (c *ProviderConfig) GetAvailableTenantsForProvider(providerName string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var tenants []string
	providerSuffix := "_" + strings.ToLower(providerName)

	for key := range c.configs {
		if strings.HasSuffix(key, providerSuffix) {
			tenant := strings.TrimSuffix(key, providerSuffix)
			tenants = append(tenants, strings.ToLower(tenant))
		}
	}

	return tenants
}

// validateProviderConfig validates configuration based on provider requirements
func (c *ProviderConfig) validateProviderConfig(providerName string, config map[string]string) error {
	requiredKeys := make(map[string][]string)

	// Define required keys for each provider
	requiredKeys["iyzico"] = []string{"apiKey", "secretKey"}
	requiredKeys["ozanpay"] = []string{"apiKey", "secretKey", "merchantId"}
	requiredKeys["paycell"] = []string{"username", "password", "merchantId", "terminalId"}

	required, exists := requiredKeys[strings.ToLower(providerName)]
	if !exists {
		return fmt.Errorf("unsupported provider: %s", providerName)
	}

	// Check if all required keys are present and not empty
	for _, key := range required {
		value, exists := config[key]
		if !exists || strings.TrimSpace(value) == "" {
			return fmt.Errorf("required key '%s' is missing or empty", key)
		}
	}

	return nil
}

// loadProviderFromEnv loads a single provider's configuration from environment variables
func (c *ProviderConfig) loadProviderFromEnv(providerName string, keys []string) {
	config := make(map[string]string)
	providerPrefix := strings.ToUpper(providerName) + "_"

	for _, key := range keys {
		envKey := providerPrefix + strings.ToUpper(key)
		value := GetEnv(envKey, "")
		if value != "" {
			config[key] = value
		}
	}

	// Only add if there are any values configured
	if len(config) > 0 {
		c.configs[providerName] = config
	}
}

// GetConfig returns configuration for a specific provider (legacy method, for backward compatibility)
func (c *ProviderConfig) GetConfig(providerName string) (map[string]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config, exists := c.configs[providerName]
	if !exists {
		return nil, fmt.Errorf("no configuration found for provider: %s", providerName)
	}
	return config, nil
}

// GetAvailableProviders returns all providers that have configurations (legacy method)
func (c *ProviderConfig) GetAvailableProviders() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	providers := make([]string, 0, len(c.configs))
	for provider := range c.configs {
		// Skip tenant-specific configs (they contain underscore)
		if !strings.Contains(provider, "_") {
			providers = append(providers, provider)
		}
	}
	return providers
}

// GetBaseURL returns the configured base URL for callback URLs
func (c *ProviderConfig) GetBaseURL() string {
	return c.baseURL
}

// Close closes the SQLite storage connection
func (c *ProviderConfig) Close() error {
	if c.storage != nil {
		return c.storage.Close()
	}
	return nil
}

// GetStats returns configuration and storage statistics
func (c *ProviderConfig) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	c.mu.RLock()
	memoryConfigs := len(c.configs)
	c.mu.RUnlock()

	stats["memory_configs"] = memoryConfigs
	stats["base_url"] = c.baseURL

	// Get SQLite statistics if available
	if c.storage != nil {
		sqliteStats, err := c.storage.GetStats()
		if err != nil {
			stats["sqlite_error"] = err.Error()
		} else {
			stats["sqlite"] = sqliteStats
		}
	} else {
		stats["sqlite"] = "not_available"
	}

	return stats, nil
}

// DeleteTenantConfig deletes a tenant configuration
func (c *ProviderConfig) DeleteTenantConfig(tenantID, providerName string) error {
	if tenantID == "" {
		return fmt.Errorf("tenant ID cannot be empty")
	}
	if providerName == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Create tenant-specific provider key
	tenantProviderKey := fmt.Sprintf("%s_%s", strings.ToUpper(tenantID), strings.ToLower(providerName))

	// Delete from SQLite if available
	if c.storage != nil {
		if err := c.storage.DeleteTenantConfig(tenantID, providerName); err != nil {
			return fmt.Errorf("failed to delete config from SQLite: %w", err)
		}
	}

	// Delete from memory cache
	delete(c.configs, tenantProviderKey)
	return nil
}
