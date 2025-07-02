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

// CreateProvider creates a new instance of a payment provider
func (r *ProviderRegistry) CreateProvider(name string) (PaymentProvider, error) {
	factory, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	return factory(), nil
}

// GetProviderNames returns a list of all registered provider names
func (r *ProviderRegistry) GetProviderNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
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

// CreateProvider creates a provider instance from the default registry
func CreateProvider(name string) (PaymentProvider, error) {
	return DefaultRegistry.CreateProvider(name)
}
