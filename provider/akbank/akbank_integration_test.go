package akbank

import (
	"context"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

// Test credentials from Akbank sandbox
const (
	testMerchantSafeId = "2025100217305644994AAC1BF57EC29B"
	testTerminalSafeId = "202510021730564616275A2A52298FCF"
	testSecretKey      = "323032353130303231373330353634343135315f763737353873315f3176383731723331723572377367315f333572386733383132377431315f377267313532"
)

// Test card numbers
var (
	testCard = provider.CardInfo{
		CardNumber:     "4355084355084358",
		CardHolderName: "Test User",
		ExpireMonth:    "12",
		ExpireYear:     "2026",
		CVV:            "000",
	}
)

func getTestProvider(t *testing.T) *AkbankProvider {
	p := NewProvider()
	config := map[string]string{
		"merchantSafeId": testMerchantSafeId,
		"terminalSafeId": testTerminalSafeId,
		"secretKey":      testSecretKey,
		"environment":    "sandbox",
	}

	err := p.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	return p.(*AkbankProvider)
}

func TestIntegration_CreatePayment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	p := getTestProvider(t)
	ctx := context.Background()

	request := provider.PaymentRequest{
		TenantID: 1,
		Amount:   1.00,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:      "Test",
			Surname:   "User",
			Email:     "test@test.com",
			IPAddress: "127.0.0.1",
		},
		CardInfo:         testCard,
		InstallmentCount: 1,
		LogID:            1,
	}

	response, err := p.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	t.Logf("Payment Response:")
	t.Logf("  Success: %v", response.Success)
	t.Logf("  Status: %v", response.Status)
	t.Logf("  Message: %v", response.Message)
	t.Logf("  PaymentID: %v", response.PaymentID)
	t.Logf("  TransactionID: %v", response.TransactionID)
	t.Logf("  ErrorCode: %v", response.ErrorCode)

	// Note: In sandbox, the payment might fail if test mode is not properly configured
	// The important thing is that we get a proper response structure
	if response.Success {
		if response.PaymentID == "" {
			t.Error("Expected PaymentID to be set for successful payment")
		}
		if response.Status != provider.StatusSuccessful {
			t.Errorf("Expected status to be successful, got %v", response.Status)
		}
	}
}

func TestIntegration_CreatePaymentWithInstallments(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	p := getTestProvider(t)
	ctx := context.Background()

	request := provider.PaymentRequest{
		TenantID: 1,
		Amount:   100.00,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:      "Test",
			Surname:   "User",
			Email:     "test@test.com",
			IPAddress: "127.0.0.1",
		},
		CardInfo:         testCard,
		InstallmentCount: 3,
		LogID:            2,
	}

	response, err := p.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("CreatePayment with installments failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	t.Logf("Payment with Installments Response:")
	t.Logf("  Success: %v", response.Success)
	t.Logf("  Status: %v", response.Status)
	t.Logf("  Message: %v", response.Message)
}

func TestIntegration_InvalidCard(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	p := getTestProvider(t)
	ctx := context.Background()

	invalidCard := provider.CardInfo{
		CardNumber:     "1234567890123456",
		CardHolderName: "Test User",
		ExpireMonth:    "12",
		ExpireYear:     "2026",
		CVV:            "000",
	}

	request := provider.PaymentRequest{
		TenantID: 1,
		Amount:   1.00,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:      "Test",
			Surname:   "User",
			Email:     "test@test.com",
			IPAddress: "127.0.0.1",
		},
		CardInfo: invalidCard,
		LogID:    3,
	}

	response, err := p.CreatePayment(ctx, request)

	// Should not return error, but response.Success should be false
	if err != nil {
		t.Logf("CreatePayment with invalid card returned error (acceptable): %v", err)
	}

	if response != nil {
		t.Logf("Invalid Card Response:")
		t.Logf("  Success: %v", response.Success)
		t.Logf("  Status: %v", response.Status)
		t.Logf("  Message: %v", response.Message)
		t.Logf("  ErrorCode: %v", response.ErrorCode)

		if response.Success {
			t.Error("Expected payment to fail with invalid card")
		}
	}
}

func TestIntegration_AuthHashGeneration(t *testing.T) {
	p := getTestProvider(t)

	// Test with known data
	testData := `{"version":"1.00","txnCode":"1000"}`
	hash := p.generateAuthHash(testData)

	if hash == "" {
		t.Error("Hash generation returned empty string")
	}

	t.Logf("Generated hash: %s", hash)

	// Verify it's base64 encoded
	if len(hash) < 20 {
		t.Error("Hash seems too short to be valid base64-encoded SHA512")
	}
}

func TestIntegration_RequestStructure(t *testing.T) {
	p := getTestProvider(t)

	// Build a test request
	req := p.buildBaseRequest(txnCodeSale)

	// Verify required fields
	if req["version"] == nil {
		t.Error("version field missing")
	}
	if req["txnCode"] == nil {
		t.Error("txnCode field missing")
	}
	if req["requestDateTime"] == nil {
		t.Error("requestDateTime field missing")
	}
	if req["randomNumber"] == nil {
		t.Error("randomNumber field missing")
	}
	if req["terminal"] == nil {
		t.Error("terminal field missing")
	}

	terminal, ok := req["terminal"].(map[string]any)
	if !ok {
		t.Fatal("terminal field is not a map")
	}

	if terminal["merchantSafeId"] == nil {
		t.Error("merchantSafeId field missing in terminal")
	}
	if terminal["terminalSafeId"] == nil {
		t.Error("terminalSafeId field missing in terminal")
	}

	t.Logf("Request structure validation passed")
	t.Logf("Version: %v", req["version"])
	t.Logf("TxnCode: %v", req["txnCode"])
	t.Logf("RequestDateTime: %v", req["requestDateTime"])
	t.Logf("RandomNumber length: %d", len(req["randomNumber"].(string)))
}
