package config

import (
	"fmt"
	"strings"
)

// ProviderConfig manages payment provider configurations
type ProviderConfig struct {
	configs map[string]map[string]string
	baseURL string
}

// NewProviderConfig creates a new provider configuration
func NewProviderConfig() *ProviderConfig {
	return &ProviderConfig{
		configs: make(map[string]map[string]string),
		baseURL: GetEnv("APP_URL", "http://localhost:9999"),
	}
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

// GetConfig returns configuration for a specific provider
func (c *ProviderConfig) GetConfig(providerName string) (map[string]string, error) {
	config, exists := c.configs[providerName]
	if !exists {
		return nil, fmt.Errorf("no configuration found for provider: %s", providerName)
	}
	return config, nil
}

// GetAvailableProviders returns all providers that have configurations
func (c *ProviderConfig) GetAvailableProviders() []string {
	providers := make([]string, 0, len(c.configs))
	for provider := range c.configs {
		providers = append(providers, provider)
	}
	return providers
}

// GetBaseURL returns the configured base URL for callback URLs
func (c *ProviderConfig) GetBaseURL() string {
	return c.baseURL
}
