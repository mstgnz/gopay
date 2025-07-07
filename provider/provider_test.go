package provider

import (
	"context"
	"testing"
)

// CustomPaymentProcessor is a custom payment processor that has similar methods
// to the PaymentProvider interface but isn't explicitly implementing it
type CustomPaymentProcessor struct {
	name   string
	apiKey string
}

// Initialize sets up the payment processor
func (p *CustomPaymentProcessor) Initialize(config map[string]string) error {
	p.apiKey = config["apiKey"]
	return nil
}

// CreatePayment processes a payment
func (p *CustomPaymentProcessor) CreatePayment(ctx context.Context, request PaymentRequest) (*PaymentResponse, error) {
	// This would normally call the actual payment API
	return &PaymentResponse{
		Success:   true,
		Status:    StatusSuccessful,
		PaymentID: "payment_123",
		Amount:    request.Amount,
		Currency:  request.Currency,
	}, nil
}

// Create3DPayment initiates a 3D secure payment
func (p *CustomPaymentProcessor) Create3DPayment(ctx context.Context, request PaymentRequest) (*PaymentResponse, error) {
	return &PaymentResponse{
		Success:     true,
		Status:      StatusPending,
		PaymentID:   "payment_3d_123",
		Amount:      request.Amount,
		Currency:    request.Currency,
		RedirectURL: "https://example.com/3d-auth",
	}, nil
}

// Complete3DPayment completes a 3D payment
func (p *CustomPaymentProcessor) Complete3DPayment(ctx context.Context, paymentID, conversationID string, data map[string]string) (*PaymentResponse, error) {
	return &PaymentResponse{
		Success:   true,
		Status:    StatusSuccessful,
		PaymentID: paymentID,
		Amount:    100.0, // In a real implementation, would retrieve from DB
		Currency:  "USD",
	}, nil
}

// GetPaymentStatus checks payment status
func (p *CustomPaymentProcessor) GetPaymentStatus(ctx context.Context, paymentID string) (*PaymentResponse, error) {
	return &PaymentResponse{
		Success:   true,
		Status:    StatusSuccessful,
		PaymentID: paymentID,
	}, nil
}

// CancelPayment cancels a payment
func (p *CustomPaymentProcessor) CancelPayment(ctx context.Context, paymentID, reason string) (*PaymentResponse, error) {
	return &PaymentResponse{
		Success:   true,
		Status:    StatusCancelled,
		PaymentID: paymentID,
		Message:   reason,
	}, nil
}

// RefundPayment processes a refund
func (p *CustomPaymentProcessor) RefundPayment(ctx context.Context, request RefundRequest) (*RefundResponse, error) {
	return &RefundResponse{
		Success:      true,
		RefundID:     "refund_123",
		PaymentID:    request.PaymentID,
		Status:       "completed",
		RefundAmount: request.RefundAmount,
	}, nil
}

// ValidateWebhook validates webhook data
func (p *CustomPaymentProcessor) ValidateWebhook(ctx context.Context, data, headers map[string]string) (bool, map[string]string, error) {
	// Simplified validation
	if _, ok := headers["X-Signature"]; ok {
		return true, map[string]string{"status": "validated"}, nil
	}
	return false, nil, nil
}

func TestGenericProvider(t *testing.T) {
	// Create a custom payment processor
	customProcessor := &CustomPaymentProcessor{
		name: "CustomProcessor",
	}

	// Wrap it with the generic provider
	genericProvider := NewGenericProvider(customProcessor)

	// Initialize the provider
	err := genericProvider.Initialize(map[string]string{
		"apiKey": "test_api_key",
	})
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Test creating a payment
	ctx := context.Background()
	response, err := genericProvider.CreatePayment(ctx, PaymentRequest{
		Amount:   100.0,
		Currency: "USD",
		Customer: Customer{
			ID:    "cust_123",
			Name:  "John",
			Email: "john@example.com",
		},
		CardInfo: CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     "4111111111111111",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CVV:            "123",
		},
		Use3D: false,
	})

	if err != nil {
		t.Fatalf("Failed to create payment: %v", err)
	}

	if !response.Success || response.Status != StatusSuccessful {
		t.Errorf("Expected successful payment, got: %v", response.Status)
	}

	// Test other methods as needed...
}

// Example of using the generic provider in a service
func ExampleNewGenericProvider() {
	// Create a service
	service := NewPaymentService(&MockLogger{})

	// Create a custom payment processor
	customProcessor := &CustomPaymentProcessor{
		name: "CustomProcessor",
	}

	// Wrap it with the generic provider
	genericProvider := NewGenericProvider(customProcessor)

	// Add it to the service (normally this would be done via AddProvider method)
	service.providers = map[string]PaymentProvider{
		"custom": genericProvider,
	}
	service.defaultProvider = "custom"

	// Now the service can use the custom processor via the generic wrapper
}
