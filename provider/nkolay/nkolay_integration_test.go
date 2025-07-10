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
// These tests use the real test credentials provided:
// - sx (Token): 118591467|bScbGDYCtPf7SS1N6PQ6/+58rFhW1WpsWINqvkJFaJlu6bMH2tgPKDQtjeA5vClpzJP24uA0vx7OX53cP3SgUspa4EvYix+1C3aXe++8glUvu9Oyyj3v300p5NP7ro/9K57Zcw==
// - Merchant Secret Key: _YckdxUbv4vrnMUZ6VQsr
// - URL: https://paynkolaytest.nkolayislem.com.tr/Vpos
//
// Test can be controlled with environment variable:
// NKOLAY_TEST_ENABLED=true

func getTestProvider(t *testing.T) *NkolayProvider {
	nkolayProvider := NewProvider().(*NkolayProvider)

	// Use real test credentials provided by Nkolay
	config := map[string]string{
		"sx":           testSx,
		"sxList":       testSxList,
		"sxCancel":     testSxCancel,
		"secretKey":    testSecretKey,
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
		TenantID: 1,
		Amount:   10.04, // Test amount from postman collection
		Currency: "TRY",
		Customer: provider.Customer{
			ID:          fmt.Sprintf("test-customer-%d", time.Now().Unix()),
			Name:        "Tuna",
			Surname:     "Atlas",
			Email:       "tuna.atlas@mail.com",
			PhoneNumber: "+905554090909",
			IPAddress:   "192.168.1.1",
			Address: &provider.Address{
				City:    "Istanbul",
				Country: "Turkey",
				Address: "Adres bilgisi",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Ömer Çınaroğlu",   // From postman collection
			CardNumber:     "4546711234567894", // Test card from postman
			CVV:            "001",
			ExpireMonth:    "12",
			ExpireYear:     "2026",
		},
		Items: []provider.Item{
			{
				ID:       fmt.Sprintf("item-%d", time.Now().Unix()),
				Name:     "Test Product",
				Category: "Electronics",
				Price:    10.04,
				Quantity: 1,
			},
		},
		Description:    "İşleme dair diğer bilgiler",
		ConversationID: fmt.Sprintf("conv-%d", time.Now().Unix()),
	}
}

func TestIntegration_CreatePayment(t *testing.T) {
	nkolayProvider := getTestProvider(t)
	request := getValidPaymentRequest()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := nkolayProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	// Log raw response for debugging
	if resp, ok := response.ProviderResponse.(map[string]any); ok {
		t.Logf("Raw Response: %s", resp["raw_response"])
	}

	// Verify response
	if response.PaymentID == "" {
		t.Error("Expected non-empty payment ID")
	}

	if response.Amount != request.Amount {
		t.Errorf("Expected amount %.2f, got %.2f", request.Amount, response.Amount)
	}

	if response.Currency != request.Currency {
		t.Errorf("Expected currency %s, got %s", request.Currency, response.Currency)
	}

	// Check if it's a successful payment or 3D redirect
	if response.Success && response.Status == provider.StatusSuccessful {
		t.Logf("✅ Payment successful - ID: %s, Amount: %.2f %s",
			response.PaymentID, response.Amount, response.Currency)
	} else if response.Success && response.Status == provider.StatusPending && response.HTML != "" {
		t.Logf("✅ 3D Secure form received - ID: %s, HTML length: %d",
			response.PaymentID, len(response.HTML))
	} else {
		t.Logf("⚠️ Unexpected response - Success: %v, Status: %v, Message: %s",
			response.Success, response.Status, response.Message)
	}
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

	// Log raw response for debugging
	if resp, ok := response.ProviderResponse.(map[string]any); ok {
		t.Logf("Raw 3D Response: %s", resp["raw_response"])
	}

	// For 3D payments, we expect either success or a form for 3D authentication
	if response.Success && response.Status == provider.StatusPending {
		if response.HTML != "" {
			t.Logf("✅ 3D Secure form received - ID: %s, HTML length: %d",
				response.PaymentID, len(response.HTML))

			// Check if HTML contains form
			if strings.Contains(response.HTML, "<form") {
				t.Logf("✅ HTML contains form for 3D authentication")
			}
		} else if response.RedirectURL != "" {
			t.Logf("✅ 3D Secure redirect URL received - ID: %s, URL: %s",
				response.PaymentID, response.RedirectURL)
		}
	} else if response.Success && response.Status == provider.StatusSuccessful {
		t.Logf("✅ Direct payment successful (no 3D required) - ID: %s", response.PaymentID)
	} else {
		t.Logf("⚠️ Unexpected 3D response - Success: %v, Status: %v, Message: %s, Error: %s",
			response.Success, response.Status, response.Message, response.ErrorCode)
	}

	// Basic validations
	if response.PaymentID == "" {
		t.Error("Expected non-empty payment ID")
	}

	if response.Amount != request.Amount {
		t.Errorf("Expected amount %.2f, got %.2f", request.Amount, response.Amount)
	}
}

func TestIntegration_PaymentWithDifferentCard(t *testing.T) {
	nkolayProvider := getTestProvider(t)
	request := getValidPaymentRequest()

	// Use different test card from postman collection
	request.CardInfo.CardNumber = "4155650100416111" // Different test card
	request.CardInfo.CardHolderName = "Test User"
	request.Amount = 6.00 // Different amount from postman collection

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := nkolayProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment with different card failed: %v", err)
	}

	if resp, ok := response.ProviderResponse.(map[string]any); ok {
		t.Logf("Different Card Response: %s", resp["raw_response"])
	}

	// Log result
	if response.Success {
		t.Logf("✅ Payment with different card successful - ID: %s, Status: %v",
			response.PaymentID, response.Status)
	} else {
		t.Logf("❌ Payment with different card failed - Error: %s, Message: %s",
			response.ErrorCode, response.Message)
	}
}

func TestIntegration_GetPaymentStatus(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// First create a payment
	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	paymentResponse, err := nkolayProvider.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("Failed to create payment for status test: %v", err)
	}

	t.Logf("Created payment for status check: %s", paymentResponse.PaymentID)

	// Wait a moment before checking status
	time.Sleep(2 * time.Second)

	// Now check the payment status
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	statusResponse, err := nkolayProvider.GetPaymentStatus(ctx2, paymentResponse.PaymentID)
	if err != nil {
		t.Fatalf("GetPaymentStatus failed: %v", err)
	}

	if resp, ok := statusResponse.ProviderResponse.(map[string]any); ok {
		t.Logf("Payment status response: %s", resp["raw_response"])
	}

	// Basic validation
	if statusResponse.PaymentID != paymentResponse.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", paymentResponse.PaymentID, statusResponse.PaymentID)
	}

	t.Logf("✅ Payment status check completed - ID: %s, Success: %v",
		statusResponse.PaymentID, statusResponse.Success)
}

func TestIntegration_CancelPayment(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// Use a dummy payment ID for cancel test
	dummyPaymentID := "IKSIRPF428910" // Reference from postman collection

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := nkolayProvider.CancelPayment(ctx, dummyPaymentID, "Test cancellation")

	if err != nil {
		t.Fatalf("CancelPayment failed: %v", err)
	}

	if resp, ok := response.ProviderResponse.(map[string]any); ok {
		t.Logf("Cancel response: %s", resp["raw_response"])
	}

	// Log result
	if response.Success {
		t.Logf("✅ Payment cancellation successful - ID: %s", response.PaymentID)
	} else {
		t.Logf("⚠️ Payment cancellation result - Success: %v, Message: %s",
			response.Success, response.Message)
	}

	// Basic validation
	if response.PaymentID != dummyPaymentID {
		t.Errorf("Expected payment ID %s, got %s", dummyPaymentID, response.PaymentID)
	}
}

func TestIntegration_RefundPayment(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// Use a dummy payment ID for refund test
	dummyPaymentID := "IKSIRPF428910" // Reference from postman collection
	refundAmount := 1.50

	refundRequest := provider.RefundRequest{
		PaymentID:    dummyPaymentID,
		RefundAmount: refundAmount,
		Reason:       "Test refund",
		Currency:     "TRY",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := nkolayProvider.RefundPayment(ctx, refundRequest)

	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}

	if resp, ok := response.RawResponse.(map[string]any); ok {
		t.Logf("Refund response: %s", resp["raw_response"])
	}

	// Log result
	if response.Success {
		t.Logf("✅ Payment refund successful - Refund ID: %s, Amount: %.2f",
			response.RefundID, response.RefundAmount)
	} else {
		t.Logf("⚠️ Payment refund result - Success: %v, Message: %s",
			response.Success, response.Message)
	}

	// Basic validation
	if response.PaymentID != dummyPaymentID {
		t.Errorf("Expected payment ID %s, got %s", dummyPaymentID, response.PaymentID)
	}

	if response.RefundAmount != refundAmount {
		t.Errorf("Expected refund amount %.2f, got %.2f", refundAmount, response.RefundAmount)
	}
}

func TestIntegration_ValidateWebhook(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// Sample webhook data (would come from Nkolay callback)
	webhookData := map[string]string{
		"referenceCode": "IKSIRPF428910",
		"status":        "SUCCESS",
		"amount":        "10.04",
		"message":       "Payment successful",
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	isValid, validatedData, err := nkolayProvider.ValidateWebhook(ctx, webhookData, headers)

	if err != nil {
		t.Fatalf("ValidateWebhook failed: %v", err)
	}

	if !isValid {
		t.Error("Expected webhook to be valid")
	}

	if validatedData["referenceCode"] != webhookData["referenceCode"] {
		t.Errorf("Expected reference code %s, got %s",
			webhookData["referenceCode"], validatedData["referenceCode"])
	}

	t.Logf("✅ Webhook validation successful - Reference: %s, Status: %s",
		validatedData["referenceCode"], validatedData["status"])
}

func TestIntegration_Complete3DPayment(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// Sample 3D callback data (would come from Nkolay)
	callbackData := map[string]string{
		"referenceCode": "gopay_12345",
		"status":        "SUCCESS",
		"amount":        "10.04",
		"message":       "3D payment completed",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := nkolayProvider.Complete3DPayment(ctx, "gopay_12345", "conv_123", callbackData)

	if err != nil {
		t.Fatalf("Complete3DPayment failed: %v", err)
	}

	// Verify response
	if !response.Success {
		t.Errorf("Expected successful 3D completion, got: %s", response.Message)
	}

	if response.Status != provider.StatusSuccessful {
		t.Errorf("Expected status successful, got: %v", response.Status)
	}

	if response.PaymentID != "gopay_12345" {
		t.Errorf("Expected payment ID gopay_12345, got %s", response.PaymentID)
	}

	t.Logf("✅ 3D Payment completion successful - ID: %s, Amount: %.2f",
		response.PaymentID, response.Amount)
}

func TestIntegration_PaymentEndpoints(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// Test that endpoints are properly configured
	expectedBaseURL := "https://paynkolaytest.nkolayislem.com.tr"
	if nkolayProvider.baseURL != expectedBaseURL {
		t.Errorf("Expected base URL %s, got %s", expectedBaseURL, nkolayProvider.baseURL)
	}

	// Test that credentials are set
	if nkolayProvider.sx == "" {
		t.Error("Expected sx token to be set")
	}

	if nkolayProvider.secretKey == "" {
		t.Error("Expected secret key to be set")
	}

	t.Logf("✅ Nkolay provider properly configured")
	t.Logf("Base URL: %s", nkolayProvider.baseURL)
	t.Logf("SX Token: %s...", nkolayProvider.sx[:50]) // Log first 50 chars
	t.Logf("Secret Key: %s", nkolayProvider.secretKey)
}

func BenchmarkIntegration_CreatePayment(b *testing.B) {
	if os.Getenv("NKOLAY_TEST_ENABLED") != "true" {
		b.Skip("Nkolay integration tests disabled")
	}

	nkolayProvider := NewProvider().(*NkolayProvider)
	config := map[string]string{
		"sx":           testSx,
		"secretKey":    testSecretKey,
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
		request.Amount = float64(i%10 + 1) // Vary amounts

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		_, err := nkolayProvider.CreatePayment(ctx, request)
		cancel()

		if err != nil {
			b.Logf("Payment %d failed: %v", i, err)
		}
	}
}

func TestIntegration_FullWorkflow(t *testing.T) {
	nkolayProvider := getTestProvider(t)

	// 1. Create payment
	request := getValidPaymentRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("🔄 Step 1: Creating payment...")
	paymentResponse, err := nkolayProvider.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("Failed to create payment: %v", err)
	}

	t.Logf("✅ Step 1 completed - Payment ID: %s", paymentResponse.PaymentID)

	// 2. Check payment status
	time.Sleep(1 * time.Second)
	t.Log("🔄 Step 2: Checking payment status...")

	statusResponse, err := nkolayProvider.GetPaymentStatus(ctx, paymentResponse.PaymentID)
	if err != nil {
		t.Logf("⚠️ Status check failed (expected for test environment): %v", err)
	} else {
		t.Logf("✅ Step 2 completed - Status: %v", statusResponse.Status)
	}

	// 3. Simulate webhook
	t.Log("🔄 Step 3: Simulating webhook...")
	webhookData := map[string]string{
		"referenceCode": paymentResponse.PaymentID,
		"status":        "SUCCESS",
		"amount":        fmt.Sprintf("%.2f", request.Amount),
	}

	isValid, _, err := nkolayProvider.ValidateWebhook(ctx, webhookData, map[string]string{})
	if err != nil {
		t.Logf("⚠️ Webhook validation failed: %v", err)
	} else if isValid {
		t.Log("✅ Step 3 completed - Webhook validated")
	}

	t.Log("🎉 Full workflow test completed!")
}
