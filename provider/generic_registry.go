package provider

// RegisterGeneric registers a generic provider factory with the default registry
// T is the type of the custom provider that will be wrapped with GenericProvider
func RegisterGeneric[T any](name string, factory func() T) {
	providerFactory := func() PaymentProvider {
		customProvider := factory()
		return NewGenericProvider(customProvider)
	}

	Register(name, providerFactory)
}

// CreateGenericProvider creates a new GenericProvider wrapping any type
// and registers it with the given name
func CreateGenericProvider[T any](name string, provider T) error {
	genericProvider := NewGenericProvider(provider)

	providerFactory := func() PaymentProvider {
		return genericProvider
	}

	Register(name, providerFactory)
	return nil
}
