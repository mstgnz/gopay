package ozanpay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

const (
	// Official OzanPay test cards from documentation
	testCardSuccess1 = "5218487962459752" // 12/26 000
	testCardSuccess2 = "4446763125813623" // 12/26 000
	testCardSuccess3 = "5200190059838710" // 12/26 000

	// Test amounts and currencies
	testAmountSuccess = 100.50
	testCurrency      = "TRY" // OzanPay primarily supports TRY

	// OzanPay public test credentials
	testAPIKey      = "test-api-key-12345"
	testSecretKey   = "test-secret-key-67890"
	testProviderKey = "test-provider-key-abcde"
)

func getTestConfig() map[string]string {
	return map[string]string{
		"apiKey":       testAPIKey,
		"secretKey":    testSecretKey,
		"providerKey":  testProviderKey,
		"environment":  "sandbox", // Always use sandbox for integration tests
		"gopayBaseURL": "https://test.gopay.com",
	}
}

func TestOzanPayProvider_Integration_CreatePayment_Success(t *testing.T) {
	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	err := ozanpayProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	request := provider.PaymentRequest{
		Amount:   testAmountSuccess,
		Currency: testCurrency,
		Customer: provider.Customer{
			ID:      "customer_123",
			Name:    "John",
			Surname: "Doe",
			Email:   "john.doe@example.com",
			Address: &provider.Address{
				Country: "TR",
				City:    "Istanbul",
				Address: "Test Address 123",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     testCardSuccess1,
			CVV:            "000", // Test CVV
			ExpireMonth:    "12",
			ExpireYear:     "26", // 2026
		},
		Description: "Integration test payment",
	}

	ctx := context.Background()
	response, err := ozanpayProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	// Log response for debugging
	t.Logf("Payment response: %+v", response)

	if response.PaymentID == "" {
		t.Error("PaymentID should not be empty")
	}

	if response.Amount != testAmountSuccess {
		t.Errorf("Expected amount %.2f, got %.2f", testAmountSuccess, response.Amount)
	}

	if response.Currency != testCurrency {
		t.Errorf("Expected currency %s, got %s", testCurrency, response.Currency)
	}

	// Test may succeed or fail depending on card and amount, but should not error
	if response.Status == "" {
		t.Error("Status should not be empty")
	}
}

func TestOzanPayProvider_Integration_Create3DPayment(t *testing.T) {
	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	err := ozanpayProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	request := provider.PaymentRequest{
		Amount:   testAmountSuccess,
		Currency: testCurrency,
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john.doe@example.com",
			Address: &provider.Address{
				Country: "TR",
				City:    "Istanbul",
				Address: "Test Address 123",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     testCardSuccess2,
			CVV:            "000",
			ExpireMonth:    "12",
			ExpireYear:     "26",
		},
		CallbackURL: "https://example.com/callback",
		Use3D:       true,
		Description: "3D Secure test payment",
	}

	ctx := context.Background()
	response, err := ozanpayProvider.Create3DPayment(ctx, request)

	if err != nil {
		t.Fatalf("Create3DPayment failed: %v", err)
	}

	// Log response for debugging
	t.Logf("3D Payment response: %+v", response)

	if response.PaymentID == "" {
		t.Error("PaymentID should not be empty")
	}

	// For 3D payments, we expect either a redirect URL or a completed payment
	if response.Status == provider.StatusPending && response.RedirectURL == "" {
		t.Error("Expected redirect URL for 3D payment or completed payment")
	}
}

func TestOzanPayProvider_Integration_GetPaymentStatus(t *testing.T) {
	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	err := ozanpayProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// First create a payment
	request := provider.PaymentRequest{
		Amount:   testAmountSuccess,
		Currency: testCurrency,
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john.doe@example.com",
			Address: &provider.Address{
				Country: "TR",
				City:    "Istanbul",
				Address: "Test Address 123",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     testCardSuccess3,
			CVV:            "000",
			ExpireMonth:    "12",
			ExpireYear:     "26",
		},
		Description: "Status check test payment",
	}

	ctx := context.Background()
	createResponse, err := ozanpayProvider.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if createResponse.PaymentID == "" {
		t.Fatal("PaymentID should not be empty")
	}

	// Wait a moment for payment to process
	time.Sleep(2 * time.Second)

	// Now check the payment status
	statusResponse, err := ozanpayProvider.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: createResponse.PaymentID})
	if err != nil {
		t.Fatalf("GetPaymentStatus failed: %v", err)
	}

	// Log response for debugging
	t.Logf("Status response: %+v", statusResponse)

	if statusResponse.PaymentID != createResponse.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", createResponse.PaymentID, statusResponse.PaymentID)
	}

	if statusResponse.Status == "" {
		t.Error("Status should not be empty")
	}
}

func TestOzanPayProvider_Integration_RefundPayment(t *testing.T) {
	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	err := ozanpayProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// First create a successful payment
	request := provider.PaymentRequest{
		Amount:   testAmountSuccess,
		Currency: testCurrency,
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john.doe@example.com",
			Address: &provider.Address{
				Country: "TR",
				City:    "Istanbul",
				Address: "Test Address 123",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     testCardSuccess1,
			CVV:            "000",
			ExpireMonth:    "12",
			ExpireYear:     "26",
		},
		Description: "Refund test payment",
	}

	ctx := context.Background()
	createResponse, err := ozanpayProvider.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	// Only attempt refund if payment was successful
	if createResponse.Status != provider.StatusSuccessful {
		t.Skipf("Skipping refund test as payment was not successful: %v", createResponse.Status)
	}

	// Wait for payment to settle
	time.Sleep(5 * time.Second)

	// Attempt partial refund
	refundRequest := provider.RefundRequest{
		PaymentID:      createResponse.PaymentID,
		RefundAmount:   50.0, // Partial refund
		Currency:       testCurrency,
		Reason:         "Integration test refund",
		Description:    "Test refund",
		ConversationID: "test_refund_123",
	}

	refundResponse, err := ozanpayProvider.RefundPayment(ctx, refundRequest)
	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}

	// Log response for debugging
	t.Logf("Refund response: %+v", refundResponse)

	if refundResponse.PaymentID != refundRequest.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", refundRequest.PaymentID, refundResponse.PaymentID)
	}

	if refundResponse.RefundAmount != 50.0 {
		t.Errorf("Expected refund amount 50.0, got %.2f", refundResponse.RefundAmount)
	}
}

func TestOzanPayProvider_Integration_ValidateWebhook(t *testing.T) {
	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	err := ozanpayProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Sample webhook data based on OzanPay documentation
	webhookData := map[string]string{
		"transactionId": "9-1438782271-1",
		"referenceNo":   "1-1386413490-0089-14",
		"amount":        "10050", // Amount in cents
		"currency":      "TRY",
		"status":        "APPROVED",
		"message":       "Auth3D is APPROVED",
		"code":          "00",
	}

	// Calculate correct checksum using the secret key
	// checksum = SHA256(referenceNo + amount + currency + status + message + code + secretKey)
	checksumString := webhookData["referenceNo"] + webhookData["amount"] + webhookData["currency"] +
		webhookData["status"] + webhookData["message"] + webhookData["code"] + config["secretKey"]

	hash := sha256.Sum256([]byte(checksumString))
	correctChecksum := hex.EncodeToString(hash[:])
	webhookData["checksum"] = correctChecksum

	headers := map[string]string{} // OzanPay doesn't use headers for webhook validation

	ctx := context.Background()
	valid, result, err := ozanpayProvider.ValidateWebhook(ctx, webhookData, headers)

	if err != nil {
		t.Fatalf("ValidateWebhook failed: %v", err)
	}

	if !valid {
		t.Error("Expected valid webhook")
	}

	if result["paymentId"] != "9-1438782271-1" {
		t.Errorf("Expected payment ID 9-1438782271-1, got %v", result["paymentId"])
	}

	if result["status"] != "APPROVED" {
		t.Errorf("Expected status APPROVED, got %v", result["status"])
	}
}

func TestOzanPayProvider_Integration_ErrorScenarios(t *testing.T) {
	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	err := ozanpayProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	ctx := context.Background()

	// Test invalid payment request
	invalidRequest := provider.PaymentRequest{
		Amount: 0, // Invalid amount
	}

	_, err = ozanpayProvider.CreatePayment(ctx, invalidRequest)
	if err == nil {
		t.Error("Expected error for invalid payment request")
	}

	// Test missing payment ID for status check
	_, err = ozanpayProvider.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: ""})
	if err == nil {
		t.Error("Expected error for empty payment ID")
	}

	// Test missing payment ID for refund
	invalidRefund := provider.RefundRequest{
		PaymentID: "", // Empty payment ID
	}

	_, err = ozanpayProvider.RefundPayment(ctx, invalidRefund)
	if err == nil {
		t.Error("Expected error for empty payment ID in refund")
	}
}

func TestOzanPayProvider_Integration_NetworkTimeout(t *testing.T) {
	ozanpayProvider := NewProvider().(*OzanPayProvider)
	config := getTestConfig()
	err := ozanpayProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Set very short timeout by reinitializing with short timeout
	config["timeout"] = "1ms"
	ozanpayProvider.httpClient = provider.NewProviderHTTPClient(&provider.HTTPClientConfig{
		BaseURL: ozanpayProvider.baseURL,
		Timeout: 1 * time.Millisecond,
	})

	request := provider.PaymentRequest{
		Amount:   testAmountSuccess,
		Currency: testCurrency,
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     testCardSuccess1,
			CVV:            "000",
			ExpireMonth:    "12",
			ExpireYear:     "26",
		},
	}

	ctx := context.Background()
	_, err = ozanpayProvider.CreatePayment(ctx, request)

	if err == nil {
		t.Error("Expected timeout error")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout-related error, got: %v", err)
	}
}
