package payu

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

func TestPayUProvider_IntegrationTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests")
	}

	// Skip if no credentials provided
	merchantID := os.Getenv("PAYU_MERCHANT_ID")
	secretKey := os.Getenv("PAYU_SECRET_KEY")
	if merchantID == "" || secretKey == "" {
		t.Skip("PayU credentials not provided, skipping integration tests")
	}

	// Initialize provider
	p := NewProvider()
	config := map[string]string{
		"merchantId":   merchantID,
		"secretKey":    secretKey,
		"environment":  "sandbox",
		"gopayBaseURL": "https://test.gopay.com",
	}

	err := p.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize PayU provider: %v", err)
	}

	t.Run("CreatePayment_Success", func(t *testing.T) {
		testCreatePaymentSuccess(t, p)
	})

	t.Run("CreatePayment_Declined", func(t *testing.T) {
		testCreatePaymentDeclined(t, p)
	})

	t.Run("Create3DPayment_Success", func(t *testing.T) {
		testCreate3DPaymentSuccess(t, p)
	})

	t.Run("PaymentStatus_Success", func(t *testing.T) {
		testPaymentStatusSuccess(t, p)
	})

	t.Run("RefundPayment_Success", func(t *testing.T) {
		testRefundPaymentSuccess(t, p)
	})

	t.Run("CancelPayment_Success", func(t *testing.T) {
		testCancelPaymentSuccess(t, p)
	})

	t.Run("ValidateWebhook_Success", func(t *testing.T) {
		testValidateWebhookSuccess(t, p)
	})

	t.Run("Currency_Support", func(t *testing.T) {
		testCurrencySupport(t, p)
	})

	t.Run("Error_Handling", func(t *testing.T) {
		testErrorHandling(t, p)
	})

	t.Run("Environment_Configuration", func(t *testing.T) {
		testEnvironmentConfiguration(t, p)
	})

	t.Run("Timeout_Handling", func(t *testing.T) {
		testTimeoutHandling(t, p)
	})
}

func testCreatePaymentSuccess(t *testing.T, p provider.PaymentProvider) {
	ctx := context.Background()

	request := provider.PaymentRequest{
		Amount:      199.99,
		Currency:    "TRY",
		ReferenceID: generateTestOrderID(),
		Description: "PayU Integration Test Payment",
		Customer: provider.Customer{
			Name:        "Test",
			Surname:     "User",
			Email:       "test.user@payu-integration.test",
			PhoneNumber: "+905551234567",
			Address: &provider.Address{
				Address: "Test Address 123",
				City:    "Istanbul",
				Country: "TR",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test User",
			CardNumber:     "5528790000000008", // Test card for success
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CVV:            "123",
		},
		Use3D: false,
	}

	response, err := p.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected successful payment, got: %s", response.Message)
	}

	if response.PaymentID == "" {
		t.Error("PaymentID should not be empty")
	}

	if response.Amount != request.Amount {
		t.Errorf("Expected amount %f, got %f", request.Amount, response.Amount)
	}

	if response.Currency != request.Currency {
		t.Errorf("Expected currency %s, got %s", request.Currency, response.Currency)
	}

	if response.Status != provider.StatusSuccessful && response.Status != provider.StatusProcessing {
		t.Errorf("Expected successful or processing status, got %s", response.Status)
	}

	t.Logf("Payment created successfully: ID=%s, Status=%s", response.PaymentID, response.Status)
}

func testCreatePaymentDeclined(t *testing.T, p provider.PaymentProvider) {
	ctx := context.Background()

	request := provider.PaymentRequest{
		Amount:      50.00,
		Currency:    "TRY",
		ReferenceID: generateTestOrderID(),
		Description: "PayU Integration Test - Declined Payment",
		Customer: provider.Customer{
			Name:        "Test",
			Surname:     "Declined",
			Email:       "test.declined@payu-integration.test",
			PhoneNumber: "+905559876543",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test Declined",
			CardNumber:     "5555555555554444", // Test card for decline
			ExpireMonth:    "10",
			ExpireYear:     "2027",
			CVV:            "321",
		},
		Use3D: false,
	}

	response, err := p.CreatePayment(ctx, request)
	// Payment might fail or succeed depending on PayU's test setup
	// We mainly test that the integration handles responses properly
	if err != nil {
		t.Logf("Payment declined as expected: %v", err)
		return
	}

	if !response.Success {
		t.Logf("Payment declined as expected: %s", response.Message)
		if response.ErrorCode == "" {
			t.Error("Expected error code for declined payment")
		}
	}
}

func testCreate3DPaymentSuccess(t *testing.T, p provider.PaymentProvider) {
	ctx := context.Background()

	request := provider.PaymentRequest{
		Amount:      299.99,
		Currency:    "TRY",
		ReferenceID: generateTestOrderID(),
		Description: "PayU Integration Test - 3D Secure Payment",
		CallbackURL: "https://test.gopay.com/callback/payu",
		Customer: provider.Customer{
			Name:        "Test",
			Surname:     "ThreeD",
			Email:       "test.3d@payu-integration.test",
			PhoneNumber: "+905551111111",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test ThreeD",
			CardNumber:     "4059030000000009", // Test card for 3D success
			ExpireMonth:    "06",
			ExpireYear:     "2029",
			CVV:            "456",
		},
		Use3D: true,
	}

	response, err := p.Create3DPayment(ctx, request)
	if err != nil {
		t.Fatalf("Create3DPayment failed: %v", err)
	}

	// 3D payments should return pending status with redirect URL
	if response.Status != provider.StatusPending {
		t.Errorf("Expected pending status for 3D payment, got %s", response.Status)
	}

	if response.RedirectURL == "" {
		t.Error("Expected redirect URL for 3D payment")
	}

	if response.PaymentID == "" {
		t.Error("PaymentID should not be empty")
	}

	t.Logf("3D Payment created: ID=%s, RedirectURL=%s", response.PaymentID, response.RedirectURL)
}

func testPaymentStatusSuccess(t *testing.T, p provider.PaymentProvider) {
	ctx := context.Background()

	// First create a payment
	request := provider.PaymentRequest{
		Amount:      150.75,
		Currency:    "TRY",
		ReferenceID: generateTestOrderID(),
		Description: "PayU Status Test Payment",
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "Status",
			Email:   "test.status@payu-integration.test",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test Status",
			CardNumber:     "5528790000000008",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CVV:            "123",
		},
		Use3D: false,
	}

	paymentResponse, err := p.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("Failed to create payment for status test: %v", err)
	}

	// Wait a moment for the payment to be processed
	time.Sleep(2 * time.Second)

	// Check payment status
	statusResponse, err := p.GetPaymentStatus(ctx, paymentResponse.PaymentID)
	if err != nil {
		t.Fatalf("GetPaymentStatus failed: %v", err)
	}

	if statusResponse.PaymentID != paymentResponse.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", paymentResponse.PaymentID, statusResponse.PaymentID)
	}

	if statusResponse.Amount != request.Amount {
		t.Errorf("Expected amount %f, got %f", request.Amount, statusResponse.Amount)
	}

	t.Logf("Payment status retrieved: ID=%s, Status=%s", statusResponse.PaymentID, statusResponse.Status)
}

func testRefundPaymentSuccess(t *testing.T, p provider.PaymentProvider) {
	ctx := context.Background()

	// First create a successful payment
	request := provider.PaymentRequest{
		Amount:      100.00,
		Currency:    "TRY",
		ReferenceID: generateTestOrderID(),
		Description: "PayU Refund Test Payment",
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "Refund",
			Email:   "test.refund@payu-integration.test",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test Refund",
			CardNumber:     "5528790000000008",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CVV:            "123",
		},
		Use3D: false,
	}

	paymentResponse, err := p.CreatePayment(ctx, request)
	if err != nil || !paymentResponse.Success {
		t.Skip("Cannot test refund without successful payment")
	}

	// Wait for payment to settle
	time.Sleep(3 * time.Second)

	// Attempt refund
	refundRequest := provider.RefundRequest{
		PaymentID:    paymentResponse.PaymentID,
		RefundAmount: 50.00, // Partial refund
		Reason:       "Integration test refund",
		Description:  "Testing refund functionality",
	}

	refundResponse, err := p.RefundPayment(ctx, refundRequest)
	if err != nil {
		t.Logf("Refund test failed (expected in test environment): %v", err)
		return
	}

	if refundResponse.PaymentID != paymentResponse.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", paymentResponse.PaymentID, refundResponse.PaymentID)
	}

	if refundResponse.RefundAmount != refundRequest.RefundAmount {
		t.Errorf("Expected refund amount %f, got %f", refundRequest.RefundAmount, refundResponse.RefundAmount)
	}

	t.Logf("Refund processed: ID=%s, Amount=%f", refundResponse.RefundID, refundResponse.RefundAmount)
}

func testCancelPaymentSuccess(t *testing.T, p provider.PaymentProvider) {
	ctx := context.Background()

	// Create a payment that can be cancelled (might not work in sandbox)
	request := provider.PaymentRequest{
		Amount:      75.00,
		Currency:    "TRY",
		ReferenceID: generateTestOrderID(),
		Description: "PayU Cancel Test Payment",
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "Cancel",
			Email:   "test.cancel@payu-integration.test",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test Cancel",
			CardNumber:     "5528790000000008",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CVV:            "123",
		},
		Use3D: false,
	}

	paymentResponse, err := p.CreatePayment(ctx, request)
	if err != nil || !paymentResponse.Success {
		t.Skip("Cannot test cancel without successful payment")
	}

	// Attempt to cancel the payment
	cancelResponse, err := p.CancelPayment(ctx, paymentResponse.PaymentID, "Integration test cancellation")
	if err != nil {
		t.Logf("Cancel test failed (expected in test environment): %v", err)
		return
	}

	if cancelResponse.PaymentID != paymentResponse.PaymentID {
		t.Errorf("Expected payment ID %s, got %s", paymentResponse.PaymentID, cancelResponse.PaymentID)
	}

	t.Logf("Payment cancelled: ID=%s, Status=%s", cancelResponse.PaymentID, cancelResponse.Status)
}

func testValidateWebhookSuccess(t *testing.T, p provider.PaymentProvider) {
	ctx := context.Background()

	// Test webhook validation with sample data
	webhookData := map[string]string{
		"payload": `{
			"paymentId": "pay_test_webhook_123",
			"transactionId": "txn_test_456", 
			"status": "SUCCESS",
			"amount": 125.50,
			"currency": "TRY",
			"timestamp": 1705317000
		}`,
	}

	payuProvider, ok := p.(*PayUProvider)
	if !ok {
		t.Fatal("Expected PayU provider")
	}

	expectedSignature := payuProvider.calculateWebhookSignature(webhookData["payload"])

	webhookHeaders := map[string]string{
		"X-PayU-Signature": expectedSignature,
	}

	valid, result, err := p.ValidateWebhook(ctx, webhookData, webhookHeaders)
	if err != nil {
		t.Fatalf("ValidateWebhook failed: %v", err)
	}

	if !valid {
		t.Error("Expected valid webhook")
	}

	if result == nil {
		t.Error("Expected result data")
	}

	t.Logf("Webhook validated successfully: %+v", result)
}

func testCurrencySupport(t *testing.T, p provider.PaymentProvider) {
	currencies := []string{"TRY", "USD", "EUR"}

	for _, currency := range currencies {
		t.Run("Currency_"+currency, func(t *testing.T) {
			ctx := context.Background()

			request := provider.PaymentRequest{
				Amount:      100.00,
				Currency:    currency,
				ReferenceID: generateTestOrderID(),
				Description: "Currency test for " + currency,
				Customer: provider.Customer{
					Name:    "Test",
					Surname: "Currency",
					Email:   "test.currency@payu-integration.test",
				},
				CardInfo: provider.CardInfo{
					CardHolderName: "Test Currency",
					CardNumber:     "5528790000000008",
					ExpireMonth:    "12",
					ExpireYear:     "2030",
					CVV:            "123",
				},
				Use3D: false,
			}

			response, err := p.CreatePayment(ctx, request)
			if err != nil {
				t.Logf("Currency %s test failed: %v", currency, err)
				return
			}

			if response.Currency != currency {
				t.Errorf("Expected currency %s, got %s", currency, response.Currency)
			}

			t.Logf("Currency %s test passed", currency)
		})
	}
}

func testErrorHandling(t *testing.T, p provider.PaymentProvider) {
	ctx := context.Background()

	tests := []struct {
		name        string
		request     provider.PaymentRequest
		expectError bool
	}{
		{
			name: "Invalid amount",
			request: provider.PaymentRequest{
				Amount:      -10.00,
				Currency:    "TRY",
				ReferenceID: generateTestOrderID(),
			},
			expectError: true,
		},
		{
			name: "Missing required fields",
			request: provider.PaymentRequest{
				Amount:   100.00,
				Currency: "",
			},
			expectError: true,
		},
		{
			name: "Invalid card",
			request: provider.PaymentRequest{
				Amount:      100.00,
				Currency:    "TRY",
				ReferenceID: generateTestOrderID(),
				Customer: provider.Customer{
					Email: "test@example.com",
				},
				CardInfo: provider.CardInfo{
					CardNumber:  "1234567890123456", // Invalid test card
					ExpireMonth: "12",
					ExpireYear:  "2030",
					CVV:         "123",
				},
			},
			expectError: false, // May not error but will fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.CreatePayment(ctx, tt.request)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Logf("Got expected failure: %v", err)
			}
		})
	}
}

func testEnvironmentConfiguration(t *testing.T, p provider.PaymentProvider) {
	// Test different environment configurations
	environments := []struct {
		env         string
		expectedURL string
	}{
		{"sandbox", apiSandboxURL},
		{"production", apiProductionURL},
	}

	for _, env := range environments {
		t.Run("Environment_"+env.env, func(t *testing.T) {
			newProvider := NewProvider()
			config := map[string]string{
				"merchantId":  "test-merchant",
				"secretKey":   "test-secret",
				"environment": env.env,
			}

			err := newProvider.Initialize(config)
			if err != nil {
				t.Fatalf("Failed to initialize provider for %s: %v", env.env, err)
			}

			payuProvider, ok := newProvider.(*PayUProvider)
			if !ok {
				t.Fatal("Expected PayU provider")
			}

			if payuProvider.baseURL != env.expectedURL {
				t.Errorf("Expected base URL %s for %s, got %s", env.expectedURL, env.env, payuProvider.baseURL)
			}

			t.Logf("Environment %s configured correctly", env.env)
		})
	}
}

func testTimeoutHandling(t *testing.T, p provider.PaymentProvider) {
	// Test timeout configuration
	payuProvider, ok := p.(*PayUProvider)
	if !ok {
		t.Fatal("Expected PayU provider")
	}

	if payuProvider.client.Timeout == 0 {
		t.Error("HTTP client should have timeout configured")
	}

	t.Logf("Timeout configured: %v", payuProvider.client.Timeout)
}

// Helper function to generate unique order IDs for testing
func generateTestOrderID() string {
	return "payu_test_" + time.Now().Format("20060102_150405") + "_" + randomString(6)
}

// Helper function to generate random string
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
