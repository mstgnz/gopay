package config

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// ProviderConfig manages payment provider configurations
type ProviderConfig struct {
	configs map[string]map[string]string
	storage *PostgresStorage // PostgreSQL storage for persistence
	mu      sync.RWMutex     // Thread-safe access
}

// NewProviderConfig creates a new provider configuration
func NewProviderConfig() *ProviderConfig {
	config := &ProviderConfig{
		configs: make(map[string]map[string]string),
	}

	// Initialize PostgreSQL storage using shared database connection
	db := App().DB
	if db != nil && db.DB != nil {
		storage, err := NewPostgresStorage(db)
		if err != nil {
			// Fallback to memory-only mode if PostgreSQL fails
			log.Printf("Warning: Failed to initialize PostgreSQL storage (%v), falling back to memory-only mode", err)
		} else {
			config.storage = storage

			// Load existing configurations from PostgreSQL if available
			if err := config.loadFromPostgreSQL(); err != nil {
				log.Printf("Warning: Failed to load configurations from PostgreSQL: %v", err)
			}
		}
	} else {
		log.Printf("Warning: Database connection not available, using memory-only mode")
	}

	return config
}

// loadFromPostgreSQL loads all tenant configurations from PostgreSQL storage
func (c *ProviderConfig) loadFromPostgreSQL() error {
	if c.storage == nil {
		return fmt.Errorf("PostgreSQL storage not initialized")
	}

	configs, err := c.storage.LoadAllTenantConfigs()
	if err != nil {
		return fmt.Errorf("failed to load configs from PostgreSQL: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Merge PostgreSQL configs with in-memory configs
	for k, v := range configs {
		c.configs[k] = v
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

	c.mu.Lock()
	defer c.mu.Unlock()

	// Save to PostgreSQL if available
	if c.storage != nil {
		if err := c.storage.SaveTenantConfig(tenantID, providerName, config); err != nil {
			return fmt.Errorf("failed to save config to PostgreSQL: %w", err)
		}
	}

	// Create tenant-specific provider key
	tenantProviderKey := fmt.Sprintf("%s_%s", strings.ToUpper(tenantID), strings.ToLower(providerName))

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

	// If not found in memory, try loading from PostgreSQL
	if !exists && c.storage != nil {
		postgresConfig, err := c.storage.LoadTenantConfig(tenantID, providerName)
		if err == nil {
			// Cache in memory for future use
			c.mu.Lock()
			c.configs[tenantProviderKey] = postgresConfig
			c.mu.Unlock()
			config = postgresConfig
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

// GetStats returns configuration and storage statistics
func (c *ProviderConfig) GetStats() (map[string]any, error) {
	stats := make(map[string]any)

	c.mu.RLock()
	memoryConfigs := len(c.configs)
	c.mu.RUnlock()

	stats["memory_configs"] = memoryConfigs

	// Get PostgreSQL statistics if available
	if c.storage != nil {
		postgresStats, err := c.storage.GetStats()
		if err != nil {
			stats["postgres_error"] = err.Error()
		} else {
			stats["postgres"] = postgresStats
		}
	} else {
		stats["postgres"] = "not_available"
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

	// Delete from PostgreSQL if available
	if c.storage != nil {
		if err := c.storage.DeleteTenantConfig(tenantID, providerName); err != nil {
			return fmt.Errorf("failed to delete config from PostgreSQL: %w", err)
		}
	}

	// Delete from memory cache
	delete(c.configs, tenantProviderKey)
	return nil
}

// GetProviderIDByName returns the provider ID for a given provider name, or error if not found
func (c *ProviderConfig) GetProviderIDByName(providerName string) (int, error) {
	if c.storage == nil {
		return 0, fmt.Errorf("storage not initialized")
	}
	return c.storage.getProviderID(providerName)
}
