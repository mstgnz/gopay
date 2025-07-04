package iyzico

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// Integration tests for İyzico real API
//
// This file contains integration tests that make actual HTTP requests to İyzico's sandbox API.
// To run these tests successfully:
//
// 1. Register at https://sandbox-merchant.iyzipay.com/ to get your sandbox credentials
// 2. Replace the placeholder API key and secret key in getTestProvider() function
// 3. Run tests with: go test -v ./provider/iyzico/ -run TestIntegration
//
// Test Cards (from İyzico documentation):
// - Success: 5528790000000008 (12/2030, CVV: 123)
// - Insufficient funds: 5528790000000016
// - Invalid card: 5528790000000032
//
// Note: Tests are designed to work with real API responses and will skip if placeholder credentials are used.

func getTestProvider(t *testing.T) *IyzicoProvider {
	// İyzico public test credentials - register at sandbox-merchant.iyzipay.com for your own
	// These are placeholder values - replace with real sandbox credentials for actual testing
	apiKey := "sandbox-iYV8BNMt9a4tx0Aq"                    // Your sandbox API key
	secretKey := "sandbox-7LZdTLtRfaDrpQN2klBNJ7vVsRIpHFH8" // Your sandbox secret key

	// Skip tests if using placeholder values
	if apiKey == "sandbox-iYV8BNMt9a4tx0Aq" {
		t.Skip("Using placeholder test credentials. Please set real İyzico sandbox credentials")
	}

	// Create provider instance
	iyzicoProvider := NewProvider().(*IyzicoProvider)

	// Configure with test environment
	config := map[string]string{
		"apiKey":       apiKey,
		"secretKey":    secretKey,
		"environment":  "sandbox",                        // Always use sandbox for tests
		"gopayBaseURL": "https://test-gopay.example.com", // Test callback URL
	}

	err := iyzicoProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize İyzico provider: %v", err)
	}

	// Verify we're using sandbox environment
	if iyzicoProvider.isProduction {
		t.Fatal("Test provider must use sandbox environment, not production")
	}

	t.Logf("✅ İyzico provider initialized with sandbox environment (API URL: %s)", iyzicoProvider.baseURL)
	return iyzicoProvider
}

func getValidPaymentRequest() provider.PaymentRequest {
	timestamp := time.Now().Unix()
	return provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{
			ID:          fmt.Sprintf("test-customer-%d", timestamp),
			Name:        "Test",
			Surname:     "User",
			Email:       "test@example.com",
			PhoneNumber: "+905555555555",
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
				ID:       fmt.Sprintf("item-%d", timestamp),
				Name:     "Test Product",
				Category: "Electronics",
				Price:    100.50,
				Quantity: 1,
			},
		},
		Description:    "Integration test payment",
		ConversationID: fmt.Sprintf("conv-%d", timestamp),
	}
}

func TestIntegration_CreatePayment_Success(t *testing.T) {
	iyzicoProvider := getTestProvider(t)
	request := getValidPaymentRequest()

	t.Logf("🧪 Testing successful payment with card: %s", maskCardNumber(request.CardInfo.CardNumber))

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

	t.Logf("🧪 Testing insufficient funds with card: %s", maskCardNumber(request.CardInfo.CardNumber))

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

	t.Logf("🧪 Testing invalid card with card: %s", maskCardNumber(request.CardInfo.CardNumber))

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
	request.CallbackURL = "https://test-gopay.example.com/callback?successUrl=https://test-gopay.example.com/success&errorUrl=https://test-gopay.example.com/error"

	t.Logf("🧪 Testing 3D Secure payment initiation")

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

	t.Log("🧪 Testing payment status retrieval")

	// First create a payment
	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createResponse, err := iyzicoProvider.CreatePayment(ctx, request)
	if err != nil || !createResponse.Success {
		t.Fatalf("Failed to create payment for status test: %v", err)
	}

	t.Logf("💳 Payment created with ID: %s", createResponse.PaymentID)

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

	t.Log("🧪 Testing partial refund functionality")

	// First create a payment
	request := getValidPaymentRequest()
	request.Amount = 200.00 // Higher amount for partial refund test

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createResponse, err := iyzicoProvider.CreatePayment(ctx, request)
	if err != nil || !createResponse.Success {
		t.Fatalf("Failed to create payment for refund test: %v", err)
	}

	t.Logf("💳 Payment created with ID: %s, Amount: %.2f TRY", createResponse.PaymentID, createResponse.Amount)

	// Wait a bit for payment to be processed
	t.Log("⏳ Waiting for payment settlement...")
	time.Sleep(2 * time.Second)

	// Create refund request
	refundRequest := provider.RefundRequest{
		PaymentID:    createResponse.PaymentID,
		RefundAmount: 50.00, // Partial refund
		Reason:       "Integration test refund",
		Description:  "Testing partial refund functionality",
		Currency:     "TRY",
	}

	t.Logf("🔄 Requesting partial refund of %.2f TRY", refundRequest.RefundAmount)

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

	t.Logf("✅ Refund successful - Payment ID: %s, Refund ID: %s, Amount: %.2f TRY",
		refundResponse.PaymentID, refundResponse.RefundID, refundResponse.RefundAmount)
}

func TestIntegration_CancelPayment(t *testing.T) {
	iyzicoProvider := getTestProvider(t)

	t.Log("🧪 Testing payment cancellation")

	// First create a payment
	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createResponse, err := iyzicoProvider.CreatePayment(ctx, request)
	if err != nil || !createResponse.Success {
		t.Fatalf("Failed to create payment for cancel test: %v", err)
	}

	t.Logf("💳 Payment created with ID: %s", createResponse.PaymentID)

	// Wait a bit for payment to be processed
	t.Log("⏳ Waiting for payment settlement...")
	time.Sleep(2 * time.Second)

	// Cancel the payment
	t.Log("🚫 Attempting to cancel payment...")
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
	t.Log("🧪 Testing authentication failure with invalid credentials")

	// Test with invalid credentials
	iyzicoProvider := NewProvider().(*IyzicoProvider)
	config := map[string]string{
		"apiKey":       "invalid-api-key",
		"secretKey":    "invalid-secret-key",
		"environment":  "sandbox",
		"gopayBaseURL": "https://test-gopay.example.com",
	}

	err := iyzicoProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("🔑 Attempting payment with invalid credentials...")
	response, err := iyzicoProvider.CreatePayment(ctx, request)

	// Should get an error or failed response due to invalid credentials
	if err == nil && response.Success {
		t.Error("Expected authentication failure with invalid credentials")
	}

	if response != nil && !response.Success {
		t.Logf("🚫 Expected authentication error received: %s (Code: %s)", response.Message, response.ErrorCode)
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

// Helper function to mask credit card numbers for logging
func maskCardNumber(cardNumber string) string {
	if len(cardNumber) < 8 {
		return "****"
	}

	// Show first 4 and last 4 digits, mask the middle
	first4 := cardNumber[:4]
	last4 := cardNumber[len(cardNumber)-4:]
	middle := strings.Repeat("*", len(cardNumber)-8)

	return first4 + middle + last4
}

// To run these integration tests:
//
// 1. Get your sandbox credentials from https://sandbox-merchant.iyzipay.com/
// 2. Replace the placeholder credentials in getTestProvider() function
// 3. Run individual tests:
//    go test -v ./provider/iyzico/ -run TestIntegration_CreatePayment_Success
//    go test -v ./provider/iyzico/ -run TestIntegration_Create3DPayment
//    go test -v ./provider/iyzico/ -run TestIntegration_FullWorkflow
// 4. Run all integration tests:
//    go test -v ./provider/iyzico/ -run TestIntegration
//
// Important Notes:
// - These tests make real API calls to İyzico's sandbox environment
// - Tests will be skipped if placeholder credentials are used
// - Each test is independent and can be run separately
// - Use test cards provided in the file header comments
// - Tests include comprehensive logging for debugging purposes
