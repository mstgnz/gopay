package provider

import (
	"fmt"
	"sync"
)

// ProviderRegistry manages all payment provider implementations
type ProviderRegistry struct {
	providers map[string]ProviderFactory
	mu        sync.RWMutex
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]ProviderFactory),
	}
}

// Register adds a payment provider factory to the registry
func (r *ProviderRegistry) Register(name string, factory ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = factory
}

// Get retrieves a payment provider factory by name
func (r *ProviderRegistry) Get(name string) (ProviderFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("payment provider '%s' is not registered", name)
	}

	return factory, nil
}

// GetAvailableProviders returns all registered provider names
func (r *ProviderRegistry) GetAvailableProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]string, 0, len(r.providers))
	for name := range r.providers {
		providers = append(providers, name)
	}

	return providers
}

// DefaultRegistry is the global default provider registry
var DefaultRegistry = NewProviderRegistry()

// Register registers a provider with the default registry
func Register(name string, factory ProviderFactory) {
	DefaultRegistry.Register(name, factory)
}

// Get retrieves a provider factory from the default registry
func Get(name string) (ProviderFactory, error) {
	return DefaultRegistry.Get(name)
}

// GetAvailableProviders returns all registered provider names from the default registry
func GetAvailableProviders() []string {
	return DefaultRegistry.GetAvailableProviders()
}
