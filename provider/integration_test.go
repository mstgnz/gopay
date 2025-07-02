package provider

import (
	"context"
	"testing"
)

// MockProvider is a mock implementation of PaymentProvider for testing
type MockProvider struct {
	shouldFail    bool
	failMessage   string
	paymentID     string
	transactionID string
}

func NewMockProvider() PaymentProvider {
	return &MockProvider{
		paymentID:     "mock-payment-123",
		transactionID: "mock-transaction-456",
	}
}

func (m *MockProvider) Initialize(config map[string]string) error {
	if config["shouldFail"] == "true" {
		m.shouldFail = true
		m.failMessage = config["failMessage"]
	}
	return nil
}

func (m *MockProvider) CreatePayment(ctx context.Context, request PaymentRequest) (*PaymentResponse, error) {
	if m.shouldFail {
		return &PaymentResponse{
			Success:   false,
			Status:    StatusFailed,
			Message:   m.failMessage,
			ErrorCode: "MOCK_ERROR",
		}, nil
	}

	return &PaymentResponse{
		Success:       true,
		Status:        StatusSuccessful,
		PaymentID:     m.paymentID,
		TransactionID: m.transactionID,
		Amount:        request.Amount,
		Currency:      request.Currency,
		Message:       "Payment successful",
	}, nil
}

func (m *MockProvider) Create3DPayment(ctx context.Context, request PaymentRequest) (*PaymentResponse, error) {
	if m.shouldFail {
		return &PaymentResponse{
			Success:   false,
			Status:    StatusFailed,
			Message:   m.failMessage,
			ErrorCode: "MOCK_ERROR",
		}, nil
	}

	return &PaymentResponse{
		Success:     true,
		Status:      StatusPending,
		PaymentID:   m.paymentID,
		Amount:      request.Amount,
		Currency:    request.Currency,
		HTML:        "<html>Mock 3D form</html>",
		RedirectURL: "https://mock-3d.example.com",
		Message:     "3D Secure authentication required",
	}, nil
}

func (m *MockProvider) Complete3DPayment(ctx context.Context, paymentID string, conversationID string, data map[string]string) (*PaymentResponse, error) {
	if m.shouldFail {
		return &PaymentResponse{
			Success:   false,
			Status:    StatusFailed,
			Message:   m.failMessage,
			ErrorCode: "MOCK_ERROR",
		}, nil
	}

	return &PaymentResponse{
		Success:       true,
		Status:        StatusSuccessful,
		PaymentID:     paymentID,
		TransactionID: m.transactionID,
		Message:       "3D payment completed successfully",
	}, nil
}

func (m *MockProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*PaymentResponse, error) {
	if m.shouldFail {
		return &PaymentResponse{
			Success:   false,
			Status:    StatusFailed,
			Message:   m.failMessage,
			ErrorCode: "MOCK_ERROR",
		}, nil
	}

	return &PaymentResponse{
		Success:   true,
		Status:    StatusSuccessful,
		PaymentID: paymentID,
		Message:   "Payment found",
	}, nil
}

func (m *MockProvider) CancelPayment(ctx context.Context, paymentID string, reason string) (*PaymentResponse, error) {
	if m.shouldFail {
		return &PaymentResponse{
			Success:   false,
			Status:    StatusFailed,
			Message:   m.failMessage,
			ErrorCode: "MOCK_ERROR",
		}, nil
	}

	return &PaymentResponse{
		Success:   true,
		Status:    StatusCancelled,
		PaymentID: paymentID,
		Message:   "Payment cancelled successfully",
	}, nil
}

func (m *MockProvider) RefundPayment(ctx context.Context, request RefundRequest) (*RefundResponse, error) {
	if m.shouldFail {
		return &RefundResponse{
			Success:   false,
			Message:   m.failMessage,
			ErrorCode: "MOCK_ERROR",
		}, nil
	}

	return &RefundResponse{
		Success:      true,
		RefundID:     "mock-refund-789",
		PaymentID:    request.PaymentID,
		RefundAmount: request.RefundAmount,
		Status:       "success",
		Message:      "Refund successful",
	}, nil
}

func (m *MockProvider) ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	if m.shouldFail {
		return false, nil, nil
	}

	return true, map[string]string{
		"status":    "valid",
		"paymentId": data["paymentId"],
	}, nil
}

func TestPaymentServiceIntegration(t *testing.T) {
	// Register mock provider
	Register("mock", NewMockProvider)

	service := NewPaymentService()

	// Test adding a provider
	err := service.AddProvider("mock", map[string]string{})
	if err != nil {
		t.Errorf("Failed to add provider: %v", err)
	}

	// Test setting default provider
	err = service.SetDefaultProvider("mock")
	if err != nil {
		t.Errorf("Failed to set default provider: %v", err)
	}

	// Test payment request
	paymentRequest := PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: CardInfo{
			CardNumber:  "5528790000000008",
			CVV:         "123",
			ExpireMonth: "12",
			ExpireYear:  "2030",
		},
	}

	ctx := context.Background()

	// Test CreatePayment
	response, err := service.CreatePayment(ctx, "", paymentRequest)
	if err != nil {
		t.Errorf("CreatePayment failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful payment")
	}

	if response.PaymentID != "mock-payment-123" {
		t.Errorf("Expected payment ID 'mock-payment-123', got %s", response.PaymentID)
	}

	// Test Create3DPayment
	paymentRequest.Use3D = true
	paymentRequest.CallbackURL = "https://example.com/callback"

	response3D, err := service.CreatePayment(ctx, "", paymentRequest)
	if err != nil {
		t.Errorf("Create3DPayment failed: %v", err)
	}

	if response3D.Status != StatusPending {
		t.Errorf("Expected status pending for 3D payment, got %v", response3D.Status)
	}

	// Test Complete3DPayment
	completeResponse, err := service.Complete3DPayment(ctx, "", response3D.PaymentID, "conv123", map[string]string{"status": "success"})
	if err != nil {
		t.Errorf("Complete3DPayment failed: %v", err)
	}

	if !completeResponse.Success {
		t.Error("Expected successful 3D payment completion")
	}

	// Test GetPaymentStatus
	statusResponse, err := service.GetPaymentStatus(ctx, "", response.PaymentID)
	if err != nil {
		t.Errorf("GetPaymentStatus failed: %v", err)
	}

	if !statusResponse.Success {
		t.Error("Expected successful status check")
	}

	// Test CancelPayment
	cancelResponse, err := service.CancelPayment(ctx, "", response.PaymentID, "Customer request")
	if err != nil {
		t.Errorf("CancelPayment failed: %v", err)
	}

	if cancelResponse.Status != StatusCancelled {
		t.Errorf("Expected status cancelled, got %v", cancelResponse.Status)
	}

	// Test RefundPayment
	refundRequest := RefundRequest{
		PaymentID:    response.PaymentID,
		RefundAmount: 50.0,
		Reason:       "Customer request",
	}

	refundResponse, err := service.RefundPayment(ctx, "", refundRequest)
	if err != nil {
		t.Errorf("RefundPayment failed: %v", err)
	}

	if !refundResponse.Success {
		t.Error("Expected successful refund")
	}

	// Test ValidateWebhook
	webhookData := map[string]string{
		"paymentId": response.PaymentID,
		"status":    "success",
	}

	isValid, result, err := service.ValidateWebhook(ctx, "", webhookData, map[string]string{})
	if err != nil {
		t.Errorf("ValidateWebhook failed: %v", err)
	}

	if !isValid {
		t.Error("Expected valid webhook")
	}

	if result["status"] != "valid" {
		t.Errorf("Expected webhook status 'valid', got %s", result["status"])
	}
}

func TestPaymentServiceProviderNotFound(t *testing.T) {
	service := NewPaymentService()

	// Test with non-existent provider
	paymentRequest := PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
	}

	ctx := context.Background()

	_, err := service.CreatePayment(ctx, "nonexistent", paymentRequest)
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}

func TestPaymentServiceFailingProvider(t *testing.T) {
	// Register failing mock provider
	Register("failing-mock", NewMockProvider)

	service := NewPaymentService()

	// Add provider with failure configuration
	err := service.AddProvider("failing-mock", map[string]string{
		"shouldFail":  "true",
		"failMessage": "Mock failure",
	})
	if err != nil {
		t.Errorf("Failed to add failing provider: %v", err)
	}

	paymentRequest := PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: CardInfo{
			CardNumber:  "5528790000000008",
			CVV:         "123",
			ExpireMonth: "12",
			ExpireYear:  "2030",
		},
	}

	ctx := context.Background()

	// Test failing payment
	response, err := service.CreatePayment(ctx, "failing-mock", paymentRequest)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if response.Success {
		t.Error("Expected failed payment")
	}

	if response.Message != "Mock failure" {
		t.Errorf("Expected message 'Mock failure', got %s", response.Message)
	}

	if response.ErrorCode != "MOCK_ERROR" {
		t.Errorf("Expected error code 'MOCK_ERROR', got %s", response.ErrorCode)
	}
}

func TestMultipleProviders(t *testing.T) {
	service := NewPaymentService()

	// Register multiple providers
	Register("provider1", NewMockProvider)
	Register("provider2", NewMockProvider)

	err := service.AddProvider("provider1", map[string]string{})
	if err != nil {
		t.Errorf("Failed to add provider1: %v", err)
	}

	err = service.AddProvider("provider2", map[string]string{})
	if err != nil {
		t.Errorf("Failed to add provider2: %v", err)
	}

	// First provider should be default
	defaultProvider, err := service.GetDefaultProvider()
	if err != nil {
		t.Errorf("Failed to get default provider: %v", err)
	}

	if defaultProvider == nil {
		t.Error("Default provider should not be nil")
	}

	// Test switching default provider
	err = service.SetDefaultProvider("provider2")
	if err != nil {
		t.Errorf("Failed to set provider2 as default: %v", err)
	}

	// Test getting specific provider
	provider1, err := service.GetProvider("provider1")
	if err != nil {
		t.Errorf("Failed to get provider1: %v", err)
	}

	if provider1 == nil {
		t.Error("Provider1 should not be nil")
	}

	provider2, err := service.GetProvider("provider2")
	if err != nil {
		t.Errorf("Failed to get provider2: %v", err)
	}

	if provider2 == nil {
		t.Error("Provider2 should not be nil")
	}
}
