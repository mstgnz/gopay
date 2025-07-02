package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// PaymentService manages payment operations through various providers
type PaymentService struct {
	providers       map[string]PaymentProvider
	defaultProvider string
	mu              sync.RWMutex
}

// NewPaymentService creates a new payment service
func NewPaymentService() *PaymentService {
	return &PaymentService{
		providers: make(map[string]PaymentProvider),
	}
}

// AddProvider adds a configured payment provider to the service
func (s *PaymentService) AddProvider(name string, config map[string]string) error {
	provider, err := CreateProvider(name)
	if err != nil {
		return fmt.Errorf("failed to create provider '%s': %w", name, err)
	}

	if err := provider.Initialize(config); err != nil {
		return fmt.Errorf("failed to initialize provider '%s': %w", name, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.providers[name] = provider

	// Set as default if it's the first provider
	if len(s.providers) == 1 {
		s.defaultProvider = name
	}

	return nil
}

// SetDefaultProvider sets the default payment provider
func (s *PaymentService) SetDefaultProvider(name string) error {
	s.mu.RLock()
	_, exists := s.providers[name]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("provider '%s' is not registered", name)
	}

	s.mu.Lock()
	s.defaultProvider = name
	s.mu.Unlock()

	return nil
}

// GetProvider returns a registered provider by name
func (s *PaymentService) GetProvider(name string) (PaymentProvider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provider, exists := s.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider '%s' is not registered", name)
	}

	return provider, nil
}

// GetDefaultProvider returns the default payment provider
func (s *PaymentService) GetDefaultProvider() (PaymentProvider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.defaultProvider == "" {
		return nil, errors.New("no default provider set")
	}

	provider, exists := s.providers[s.defaultProvider]
	if !exists {
		return nil, errors.New("default provider not found")
	}

	return provider, nil
}

// CreatePayment processes a payment using the specified provider
func (s *PaymentService) CreatePayment(ctx context.Context, providerName string, request PaymentRequest) (*PaymentResponse, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return nil, err
	}

	if request.Use3D {
		return provider.Create3DPayment(ctx, request)
	}

	return provider.CreatePayment(ctx, request)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (s *PaymentService) Complete3DPayment(ctx context.Context, providerName, paymentID, conversationID string, data map[string]string) (*PaymentResponse, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return nil, err
	}

	return provider.Complete3DPayment(ctx, paymentID, conversationID, data)
}

// GetPaymentStatus retrieves the current status of a payment
func (s *PaymentService) GetPaymentStatus(ctx context.Context, providerName, paymentID string) (*PaymentResponse, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return nil, err
	}

	return provider.GetPaymentStatus(ctx, paymentID)
}

// CancelPayment cancels a payment
func (s *PaymentService) CancelPayment(ctx context.Context, providerName, paymentID, reason string) (*PaymentResponse, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return nil, err
	}

	return provider.CancelPayment(ctx, paymentID, reason)
}

// RefundPayment issues a refund for a payment
func (s *PaymentService) RefundPayment(ctx context.Context, providerName string, request RefundRequest) (*RefundResponse, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return nil, err
	}

	return provider.RefundPayment(ctx, request)
}

// ValidateWebhook validates an incoming webhook notification
func (s *PaymentService) ValidateWebhook(ctx context.Context, providerName string, data, headers map[string]string) (bool, map[string]string, error) {
	provider, err := s.getProviderForOperation(providerName)
	if err != nil {
		return false, nil, err
	}

	return provider.ValidateWebhook(ctx, data, headers)
}

// Helper method to get the right provider for an operation
func (s *PaymentService) getProviderForOperation(providerName string) (PaymentProvider, error) {
	if providerName != "" {
		return s.GetProvider(providerName)
	}

	return s.GetDefaultProvider()
}
