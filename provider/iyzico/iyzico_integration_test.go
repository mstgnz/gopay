package iyzico

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// Integration tests for İyzico real API
// These tests require valid İyzico sandbox credentials
// Set environment variables:
// IYZICO_TEST_API_KEY=your_sandbox_api_key
// IYZICO_TEST_SECRET_KEY=your_sandbox_secret_key
// IYZICO_TEST_ENABLED=true

func getTestProvider(t *testing.T) *IyzicoProvider {
	apiKey := "sandbox-iyzico-api-key"
	secretKey := "sandbox-iyzico-secret-key"

	if apiKey == "" || secretKey == "" {
		t.Skip("İyzico test credentials not provided. Set IYZICO_TEST_API_KEY and IYZICO_TEST_SECRET_KEY")
	}

	iyzicoProvider := NewProvider().(*IyzicoProvider)
	config := map[string]string{
		"apiKey":       apiKey,
		"secretKey":    secretKey,
		"environment":  "sandbox",
		"gopayBaseURL": "https://test.example.com", // Test callback URL
	}

	err := iyzicoProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	return iyzicoProvider
}

func getValidPaymentRequest() provider.PaymentRequest {
	return provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{
			ID:      fmt.Sprintf("test-customer-%d", time.Now().Unix()),
			Name:    "Test",
			Surname: "User",
			Email:   "test@example.com",
			Address: provider.Address{
				City:    "Istanbul",
				Country: "Turkey",
				Address: "Test Address 123",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test User",
			CardNumber:     "5528790000000008", // İyzico successful test card
			CVV:            "123",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
		},
		Items: []provider.Item{
			{
				ID:       fmt.Sprintf("item-%d", time.Now().Unix()),
				Name:     "Test Product",
				Category: "Electronics",
				Price:    100.50,
				Quantity: 1,
			},
		},
		Description: "Integration test payment",
	}
}

func TestIntegration_CreatePayment_Success(t *testing.T) {
	iyzicoProvider := getTestProvider(t)
	request := getValidPaymentRequest()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := iyzicoProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	// Verify response
	if !response.Success {
		t.Errorf("Expected successful payment, got: %s (Code: %s)", response.Message, response.ErrorCode)
	}

	if response.Status != provider.StatusSuccessful {
		t.Errorf("Expected status successful, got: %v", response.Status)
	}

	if response.PaymentID == "" {
		t.Error("Expected non-empty payment ID")
	}

	if response.Amount != request.Amount {
		t.Errorf("Expected amount %.2f, got %.2f", request.Amount, response.Amount)
	}

	if response.Currency != request.Currency {
		t.Errorf("Expected currency %s, got %s", request.Currency, response.Currency)
	}

	t.Logf("✅ Payment successful - ID: %s, Amount: %.2f %s",
		response.PaymentID, response.Amount, response.Currency)
}

func TestIntegration_CreatePayment_InsufficientFunds(t *testing.T) {
	iyzicoProvider := getTestProvider(t)
	request := getValidPaymentRequest()
	request.CardInfo.CardNumber = "5528790000000016" // İyzico insufficient funds test card

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := iyzicoProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	// Verify error response
	if response.Success {
		t.Error("Expected failed payment for insufficient funds card")
	}

	if response.Status != provider.StatusFailed {
		t.Errorf("Expected status failed, got: %v", response.Status)
	}

	if response.ErrorCode != errorCodeNotEnoughMoney {
		t.Errorf("Expected error code %s, got %s", errorCodeNotEnoughMoney, response.ErrorCode)
	}

	t.Logf("✅ Insufficient funds error handled correctly - Code: %s, Message: %s",
		response.ErrorCode, response.Message)
}

func TestIntegration_CreatePayment_InvalidCard(t *testing.T) {
	iyzicoProvider := getTestProvider(t)
	request := getValidPaymentRequest()
	request.CardInfo.CardNumber = "5528790000000032" // İyzico invalid card test card

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := iyzicoProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	// Verify error response
	if response.Success {
		t.Error("Expected failed payment for invalid card")
	}

	if response.Status != provider.StatusFailed {
		t.Errorf("Expected status failed, got: %v", response.Status)
	}

	if response.ErrorCode != errorCodeInvalidCard {
		t.Errorf("Expected error code %s, got %s", errorCodeInvalidCard, response.ErrorCode)
	}

	t.Logf("✅ Invalid card error handled correctly - Code: %s, Message: %s",
		response.ErrorCode, response.Message)
}

func TestIntegration_Create3DPayment(t *testing.T) {
	iyzicoProvider := getTestProvider(t)
	request := getValidPaymentRequest()
	request.Use3D = true
	request.CallbackURL = "https://test.example.com/callback?successUrl=https://test.example.com/success&errorUrl=https://test.example.com/error"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := iyzicoProvider.Create3DPayment(ctx, request)

	if err != nil {
		t.Fatalf("Create3DPayment failed: %v", err)
	}

	// Verify 3D response
	if !response.Success {
		t.Errorf("Expected successful 3D initiation, got: %s (Code: %s)", response.Message, response.ErrorCode)
	}

	if response.Status != provider.StatusPending {
		t.Errorf("Expected status pending for 3D payment, got: %v", response.Status)
	}

	if response.PaymentID == "" {
		t.Error("Expected non-empty payment ID for 3D payment")
	}

	// Should have either HTML content or redirect URL
	if response.HTML == "" && response.RedirectURL == "" {
		t.Error("Expected either HTML content or redirect URL for 3D payment")
	}

	t.Logf("✅ 3D Payment initiated - ID: %s, HTML: %t, RedirectURL: %t",
		response.PaymentID, response.HTML != "", response.RedirectURL != "")
}

func TestIntegration_GetPaymentStatus(t *testing.T) {
	iyzicoProvider := getTestProvider(t)

	// First create a payment
	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createResponse, err := iyzicoProvider.CreatePayment(ctx, request)
	if err != nil || !createResponse.Success {
		t.Fatalf("Failed to create payment for status test: %v", err)
	}

	// Now check its status
	statusResponse, err := iyzicoProvider.GetPaymentStatus(ctx, createResponse.PaymentID)
	if err != nil {
		t.Fatalf("GetPaymentStatus failed: %v", err)
	}

	// Verify status response
	if !statusResponse.Success {
		t.Errorf("Expected successful status check, got: %s", statusResponse.Message)
	}

	if statusResponse.PaymentID != createResponse.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", createResponse.PaymentID, statusResponse.PaymentID)
	}

	if statusResponse.Status != provider.StatusSuccessful {
		t.Errorf("Expected status successful, got: %v", statusResponse.Status)
	}

	t.Logf("✅ Payment status retrieved - ID: %s, Status: %s",
		statusResponse.PaymentID, statusResponse.Status)
}

func TestIntegration_RefundPayment(t *testing.T) {
	iyzicoProvider := getTestProvider(t)

	// First create a payment
	request := getValidPaymentRequest()
	request.Amount = 200.00 // Higher amount for partial refund test

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createResponse, err := iyzicoProvider.CreatePayment(ctx, request)
	if err != nil || !createResponse.Success {
		t.Fatalf("Failed to create payment for refund test: %v", err)
	}

	// Wait a bit for payment to be processed
	time.Sleep(2 * time.Second)

	// Create refund request
	refundRequest := provider.RefundRequest{
		PaymentID:    createResponse.PaymentID,
		RefundAmount: 50.00, // Partial refund
		Reason:       "Integration test refund",
		Description:  "Testing partial refund functionality",
		Currency:     "TRY",
	}

	refundResponse, err := iyzicoProvider.RefundPayment(ctx, refundRequest)
	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}

	// Verify refund response
	if !refundResponse.Success {
		t.Errorf("Expected successful refund, got: %s (Code: %s)", refundResponse.Message, refundResponse.ErrorCode)
	}

	if refundResponse.PaymentID != createResponse.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", createResponse.PaymentID, refundResponse.PaymentID)
	}

	if refundResponse.RefundAmount != 50.00 {
		t.Errorf("Expected refund amount 50.00, got %.2f", refundResponse.RefundAmount)
	}

	t.Logf("✅ Refund successful - Payment ID: %s, Refund ID: %s, Amount: %.2f",
		refundResponse.PaymentID, refundResponse.RefundID, refundResponse.RefundAmount)
}

func TestIntegration_CancelPayment(t *testing.T) {
	iyzicoProvider := getTestProvider(t)

	// First create a payment
	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createResponse, err := iyzicoProvider.CreatePayment(ctx, request)
	if err != nil || !createResponse.Success {
		t.Fatalf("Failed to create payment for cancel test: %v", err)
	}

	// Wait a bit for payment to be processed
	time.Sleep(2 * time.Second)

	// Cancel the payment
	cancelResponse, err := iyzicoProvider.CancelPayment(ctx, createResponse.PaymentID, "Integration test cancellation")
	if err != nil {
		t.Fatalf("CancelPayment failed: %v", err)
	}

	// Verify cancel response
	if !cancelResponse.Success {
		t.Errorf("Expected successful cancellation, got: %s (Code: %s)", cancelResponse.Message, cancelResponse.ErrorCode)
	}

	if cancelResponse.PaymentID != createResponse.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", createResponse.PaymentID, cancelResponse.PaymentID)
	}

	t.Logf("✅ Payment cancelled - ID: %s, Status: %s",
		cancelResponse.PaymentID, cancelResponse.Status)
}

func TestIntegration_AuthenticationFailure(t *testing.T) {
	// Test with invalid credentials
	iyzicoProvider := NewProvider().(*IyzicoProvider)
	config := map[string]string{
		"apiKey":       "invalid-api-key",
		"secretKey":    "invalid-secret-key",
		"environment":  "sandbox",
		"gopayBaseURL": "https://test.example.com",
	}

	err := iyzicoProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := iyzicoProvider.CreatePayment(ctx, request)

	// Should get an error or failed response due to invalid credentials
	if err == nil && response.Success {
		t.Error("Expected authentication failure with invalid credentials")
	}

	t.Logf("✅ Authentication failure handled correctly")
}

func TestIntegration_RequestTimeout(t *testing.T) {
	iyzicoProvider := getTestProvider(t)
	request := getValidPaymentRequest()

	// Create a very short timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := iyzicoProvider.CreatePayment(ctx, request)

	// Should get a timeout error
	if err == nil {
		t.Error("Expected timeout error with very short context timeout")
	}

	t.Logf("✅ Request timeout handled correctly: %v", err)
}

// Benchmark tests for performance
func BenchmarkIntegration_CreatePayment(b *testing.B) {
	iyzicoProvider := getTestProvider(&testing.T{})
	request := getValidPaymentRequest()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use different card numbers or customer IDs to avoid conflicts
		request.Customer.ID = fmt.Sprintf("bench-customer-%d", i)
		request.Items[0].ID = fmt.Sprintf("bench-item-%d", i)

		_, err := iyzicoProvider.CreatePayment(ctx, request)
		if err != nil {
			b.Fatalf("CreatePayment failed: %v", err)
		}
	}
}

// Helper function to run all integration tests in sequence
func TestIntegration_FullWorkflow(t *testing.T) {
	iyzicoProvider := getTestProvider(t)
	request := getValidPaymentRequest()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Log("🧪 Starting full İyzico integration workflow test...")

	// Step 1: Create payment
	t.Log("Step 1: Creating payment...")
	createResponse, err := iyzicoProvider.CreatePayment(ctx, request)
	if err != nil || !createResponse.Success {
		t.Fatalf("Step 1 failed - CreatePayment: %v", err)
	}
	t.Logf("✅ Step 1 complete - Payment ID: %s", createResponse.PaymentID)

	// Step 2: Check payment status
	t.Log("Step 2: Checking payment status...")
	statusResponse, err := iyzicoProvider.GetPaymentStatus(ctx, createResponse.PaymentID)
	if err != nil || !statusResponse.Success {
		t.Fatalf("Step 2 failed - GetPaymentStatus: %v", err)
	}
	t.Logf("✅ Step 2 complete - Status: %s", statusResponse.Status)

	// Step 3: Wait and perform partial refund
	t.Log("Step 3: Processing partial refund...")
	time.Sleep(3 * time.Second) // Wait for settlement

	refundRequest := provider.RefundRequest{
		PaymentID:    createResponse.PaymentID,
		RefundAmount: 30.00,
		Reason:       "Workflow test partial refund",
		Currency:     "TRY",
	}

	refundResponse, err := iyzicoProvider.RefundPayment(ctx, refundRequest)
	if err != nil || !refundResponse.Success {
		t.Fatalf("Step 3 failed - RefundPayment: %v", err)
	}
	t.Logf("✅ Step 3 complete - Refund ID: %s, Amount: %.2f",
		refundResponse.RefundID, refundResponse.RefundAmount)

	t.Log("🎉 Full workflow completed successfully!")
}
