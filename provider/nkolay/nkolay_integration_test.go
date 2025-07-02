package nkolay

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// Integration tests for Nkolay real API
// These tests require valid Nkolay sandbox credentials
// Set environment variables:
// NKOLAY_TEST_API_KEY=your_sandbox_api_key
// NKOLAY_TEST_SECRET_KEY=your_sandbox_secret_key
// NKOLAY_TEST_MERCHANT_ID=your_sandbox_merchant_id
// NKOLAY_TEST_ENABLED=true

func getTestProvider(t *testing.T) *NkolayProvider {
	apiKey := os.Getenv("NKOLAY_TEST_API_KEY")
	secretKey := os.Getenv("NKOLAY_TEST_SECRET_KEY")
	merchantID := os.Getenv("NKOLAY_TEST_MERCHANT_ID")
	testEnabled := os.Getenv("NKOLAY_TEST_ENABLED")

	if testEnabled != "true" || apiKey == "" || secretKey == "" || merchantID == "" {
		t.Skip("Nkolay integration tests disabled or credentials not provided. Set NKOLAY_TEST_ENABLED=true, NKOLAY_TEST_API_KEY, NKOLAY_TEST_SECRET_KEY, and NKOLAY_TEST_MERCHANT_ID")
	}

	nkolayProvider := NewProvider().(*NkolayProvider)
	config := map[string]string{
		"apiKey":       apiKey,
		"secretKey":    secretKey,
		"merchantId":   merchantID,
		"environment":  "sandbox",
		"gopayBaseURL": "https://test.example.com", // Test callback URL
	}

	err := nkolayProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	return nkolayProvider
}

func getValidPaymentRequest() provider.PaymentRequest {
	return provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{
			ID:          fmt.Sprintf("test-customer-%d", time.Now().Unix()),
			Name:        "Test",
			Surname:     "User",
			Email:       "test@example.com",
			PhoneNumber: "+905551234567",
			IPAddress:   "192.168.1.1",
			Address: provider.Address{
				City:    "Istanbul",
				Country: "Turkey",
				Address: "Test Address 123",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test User",
			CardNumber:     "5528790000000008", // Nkolay successful test card
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
		Description:    "Integration test payment",
		ConversationID: fmt.Sprintf("conv-%d", time.Now().Unix()),
	}
}

func TestIntegration_CreatePayment_Success(t *testing.T) {
	nkolayProvider := getTestProvider(t)
	request := getValidPaymentRequest()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := nkolayProvider.CreatePayment(ctx, request)

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
	nkolayProvider := getTestProvider(t)
	request := getValidPaymentRequest()
	request.CardInfo.CardNumber = "5528790000000016" // Nkolay insufficient funds test card

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := nkolayProvider.CreatePayment(ctx, request)

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

	if response.ErrorCode != errorCodeInsufficientFunds {
		t.Errorf("Expected error code %s, got %s", errorCodeInsufficientFunds, response.ErrorCode)
	}

	t.Logf("✅ Insufficient funds error handled correctly - Code: %s, Message: %s",
		response.ErrorCode, response.Message)
}

func TestIntegration_CreatePayment_InvalidCard(t *testing.T) {
	nkolayProvider := getTestProvider(t)
	request := getValidPaymentRequest()
	request.CardInfo.CardNumber = "4508034508034517" // Nkolay invalid card test card

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := nkolayProvider.CreatePayment(ctx, request)

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
	nkolayProvider := getTestProvider(t)
	request := getValidPaymentRequest()
	request.Use3D = true
	request.CallbackURL = "https://test.example.com/callback?successUrl=https://test.example.com/success&errorUrl=https://test.example.com/error"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := nkolayProvider.Create3DPayment(ctx, request)

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

	if response.RedirectURL == "" && response.HTML == "" {
		t.Error("Expected either redirect URL or HTML for 3D authentication")
	}

	t.Logf("✅ 3D Payment initiated - ID: %s, Status: %v",
		response.PaymentID, response.Status)

	if response.RedirectURL != "" {
		t.Logf("   Redirect URL: %s", response.RedirectURL)
	}
	if response.HTML != "" {
		t.Logf("   HTML length: %d characters", len(response.HTML))
	}
}

func TestIntegration_GetPaymentStatus(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// First create a payment
	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createResponse, err := nkolayProvider.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("Failed to create payment for status test: %v", err)
	}

	if !createResponse.Success {
		t.Skipf("Skipping status test due to payment creation failure: %s", createResponse.Message)
	}

	// Wait a moment for payment to be processed
	time.Sleep(2 * time.Second)

	// Now check payment status
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	statusResponse, err := nkolayProvider.GetPaymentStatus(ctx2, createResponse.PaymentID)

	if err != nil {
		t.Fatalf("GetPaymentStatus failed: %v", err)
	}

	if !statusResponse.Success {
		t.Errorf("Expected successful status response, got: %s", statusResponse.Message)
	}

	if statusResponse.PaymentID != createResponse.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", createResponse.PaymentID, statusResponse.PaymentID)
	}

	if statusResponse.Amount != createResponse.Amount {
		t.Errorf("Expected amount %.2f, got %.2f", createResponse.Amount, statusResponse.Amount)
	}

	t.Logf("✅ Payment status retrieved - ID: %s, Status: %v, Amount: %.2f",
		statusResponse.PaymentID, statusResponse.Status, statusResponse.Amount)
}

func TestIntegration_RefundPayment(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// First create a successful payment
	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	paymentResponse, err := nkolayProvider.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("Failed to create payment for refund test: %v", err)
	}

	if !paymentResponse.Success {
		t.Skipf("Skipping refund test due to payment creation failure: %s", paymentResponse.Message)
	}

	// Wait for payment to be settled
	time.Sleep(5 * time.Second)

	// Now attempt refund
	refundRequest := provider.RefundRequest{
		PaymentID:      paymentResponse.PaymentID,
		RefundAmount:   50.25, // Partial refund
		Reason:         "Integration test refund",
		Currency:       paymentResponse.Currency,
		ConversationID: fmt.Sprintf("refund-%d", time.Now().Unix()),
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	refundResponse, err := nkolayProvider.RefundPayment(ctx2, refundRequest)

	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}

	if !refundResponse.Success {
		t.Errorf("Expected successful refund, got: %s (Code: %s)", refundResponse.Message, refundResponse.ErrorCode)
	}

	if refundResponse.RefundAmount != refundRequest.RefundAmount {
		t.Errorf("Expected refund amount %.2f, got %.2f", refundRequest.RefundAmount, refundResponse.RefundAmount)
	}

	t.Logf("✅ Refund successful - Payment ID: %s, Refund Amount: %.2f",
		paymentResponse.PaymentID, refundResponse.RefundAmount)
}

func TestIntegration_CancelPayment(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// First create a payment
	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	paymentResponse, err := nkolayProvider.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("Failed to create payment for cancel test: %v", err)
	}

	if !paymentResponse.Success {
		t.Skipf("Skipping cancel test due to payment creation failure: %s", paymentResponse.Message)
	}

	// Attempt to cancel immediately (before settlement)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	cancelResponse, err := nkolayProvider.CancelPayment(ctx2, paymentResponse.PaymentID, "Integration test cancellation")

	if err != nil {
		t.Fatalf("CancelPayment failed: %v", err)
	}

	if !cancelResponse.Success {
		t.Errorf("Expected successful cancellation, got: %s (Code: %s)", cancelResponse.Message, cancelResponse.ErrorCode)
	}

	if cancelResponse.Status != provider.StatusCancelled {
		t.Errorf("Expected status cancelled, got: %v", cancelResponse.Status)
	}

	t.Logf("✅ Payment cancelled - ID: %s, Status: %v",
		paymentResponse.PaymentID, cancelResponse.Status)
}

func TestIntegration_AuthenticationFailure(t *testing.T) {
	// Create provider with invalid credentials
	nkolayProvider := NewProvider().(*NkolayProvider)
	config := map[string]string{
		"apiKey":       "invalid-api-key",
		"secretKey":    "invalid-secret-key",
		"merchantId":   "invalid-merchant-id",
		"environment":  "sandbox",
		"gopayBaseURL": "https://test.example.com",
	}

	err := nkolayProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := nkolayProvider.CreatePayment(ctx, request)

	// We expect either an error or a failed response
	if err == nil && response.Success {
		t.Error("Expected authentication failure with invalid credentials")
	}

	if err == nil {
		t.Logf("✅ Authentication failure handled correctly - Code: %s, Message: %s",
			response.ErrorCode, response.Message)
	} else {
		t.Logf("✅ Authentication failure handled correctly with error: %v", err)
	}
}

func TestIntegration_RequestTimeout(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// Set a very short timeout
	nkolayProvider.client.Timeout = 1 * time.Millisecond

	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := nkolayProvider.CreatePayment(ctx, request)

	if err == nil {
		t.Error("Expected timeout error with very short timeout")
	}

	// Check if it's a timeout error
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	t.Logf("✅ Timeout handled correctly: %v", err)
}

// Benchmark test for performance measurement
func BenchmarkIntegration_CreatePayment(b *testing.B) {
	apiKey := os.Getenv("NKOLAY_TEST_API_KEY")
	secretKey := os.Getenv("NKOLAY_TEST_SECRET_KEY")
	merchantID := os.Getenv("NKOLAY_TEST_MERCHANT_ID")
	testEnabled := os.Getenv("NKOLAY_TEST_ENABLED")

	if testEnabled != "true" || apiKey == "" || secretKey == "" || merchantID == "" {
		b.Skip("Nkolay benchmark tests disabled or credentials not provided")
	}

	nkolayProvider := NewProvider().(*NkolayProvider)
	config := map[string]string{
		"apiKey":       apiKey,
		"secretKey":    secretKey,
		"merchantId":   merchantID,
		"environment":  "sandbox",
		"gopayBaseURL": "https://test.example.com",
	}

	err := nkolayProvider.Initialize(config)
	if err != nil {
		b.Fatalf("Failed to initialize provider: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		request := getValidPaymentRequest()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		_, err := nkolayProvider.CreatePayment(ctx, request)
		cancel()

		if err != nil {
			b.Errorf("Payment failed: %v", err)
		}
	}
}

func TestIntegration_FullWorkflow(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// Step 1: Create a 3D payment
	request := getValidPaymentRequest()
	request.Use3D = true
	request.CallbackURL = "https://test.example.com/callback"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	paymentResponse, err := nkolayProvider.Create3DPayment(ctx, request)
	if err != nil {
		t.Fatalf("Failed to create 3D payment: %v", err)
	}

	if !paymentResponse.Success {
		t.Skipf("Skipping full workflow test due to 3D payment creation failure: %s", paymentResponse.Message)
	}

	t.Logf("Step 1: 3D Payment created - ID: %s", paymentResponse.PaymentID)

	// Step 2: Check payment status
	time.Sleep(2 * time.Second)

	statusResponse, err := nkolayProvider.GetPaymentStatus(ctx, paymentResponse.PaymentID)
	if err != nil {
		t.Fatalf("Failed to get payment status: %v", err)
	}

	t.Logf("Step 2: Payment status - Status: %v", statusResponse.Status)

	// Step 3: If payment is in a refundable state, attempt refund
	if statusResponse.Status == provider.StatusSuccessful {
		refundRequest := provider.RefundRequest{
			PaymentID:      paymentResponse.PaymentID,
			RefundAmount:   request.Amount / 2, // Partial refund
			Reason:         "Full workflow test refund",
			Currency:       request.Currency,
			ConversationID: fmt.Sprintf("workflow-refund-%d", time.Now().Unix()),
		}

		refundResponse, err := nkolayProvider.RefundPayment(ctx, refundRequest)
		if err != nil {
			t.Logf("Refund failed (may be expected): %v", err)
		} else if refundResponse.Success {
			t.Logf("Step 3: Refund successful - Amount: %.2f", refundResponse.RefundAmount)
		} else {
			t.Logf("Step 3: Refund failed - %s", refundResponse.Message)
		}
	} else {
		t.Logf("Step 3: Skipping refund due to payment status: %v", statusResponse.Status)
	}

	t.Logf("✅ Full workflow test completed")
}
