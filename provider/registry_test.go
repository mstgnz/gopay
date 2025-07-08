package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderRegistry_Register(t *testing.T) {
	registry := NewProviderRegistry()

	// Mock provider factory
	mockFactory := func() PaymentProvider { return nil }

	registry.Register("test-provider", mockFactory)

	// Verify provider is registered
	factory, err := registry.Get("test-provider")
	assert.NoError(t, err)
	assert.NotNil(t, factory)
}

func TestProviderRegistry_GetAvailableProviders(t *testing.T) {
	registry := NewProviderRegistry()

	// Initially should be empty
	providers := registry.GetAvailableProviders()
	assert.Empty(t, providers)

	// Register some providers
	mockFactory := func() PaymentProvider { return nil }
	registry.Register("provider1", mockFactory)
	registry.Register("provider2", mockFactory)

	// Should return both providers
	providers = registry.GetAvailableProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, "provider1")
	assert.Contains(t, providers, "provider2")
}

func TestProviderRegistry_Get_NotFound(t *testing.T) {
	registry := NewProviderRegistry()

	factory, err := registry.Get("non-existent")
	assert.Error(t, err)
	assert.Nil(t, factory)
	assert.Contains(t, err.Error(), "is not registered")
}

func TestDefaultRegistry(t *testing.T) {
	// Test default registry functions
	mockFactory := func() PaymentProvider { return nil }

	Register("default-test", mockFactory)

	factory, err := Get("default-test")
	assert.NoError(t, err)
	assert.NotNil(t, factory)

	providers := GetAvailableProviders()
	assert.Contains(t, providers, "default-test")
}
