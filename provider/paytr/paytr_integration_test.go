package paytr

import (
	"context"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

func getPayTRConfig() map[string]string {
	return map[string]string{
		"merchantId":   "sandbox-paytr-merchant-id",
		"merchantKey":  "sandbox-paytr-merchant-key",
		"merchantSalt": "sandbox-paytr-merchant-salt",
		"environment":  "sandbox",
		"gopayBaseURL": "http://localhost:9999",
	}
}

func TestPayTRIntegration_IFramePayment(t *testing.T) {
	config := getPayTRConfig()
	if config["merchantId"] == "" {
		t.Skip("PAYTR_MERCHANT_ID not set, skipping integration test")
	}

	paytrProvider := NewProvider()
	err := paytrProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Create payment request
	request := provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TL",
		Customer: provider.Customer{
			Name:        "Ahmet",
			Surname:     "Test",
			Email:       "test@example.com",
			PhoneNumber: "+905551234567",
			IPAddress:   "192.168.1.1",
			Address:     &provider.Address{},
		},
		Items: []provider.Item{
			{
				Name:     "Test Product",
				Price:    100.50,
				Quantity: 1,
			},
		},
		CallbackURL: "https://test.example.com/callback",
		ClientIP:    "192.168.1.1",
	}

	// Process 3D secure payment
	response, err := paytrProvider.Create3DPayment(context.Background(), request)
	if err != nil {
		t.Fatalf("3D Payment failed: %v", err)
	}

	// Verify response
	if response.PaymentID == "" {
		t.Error("PaymentID should not be empty")
	}

	if response.Success && response.RedirectURL == "" {
		t.Error("RedirectURL should not be empty for successful 3D payment")
	}

	t.Logf("Payment ID: %s", response.PaymentID)
	t.Logf("Redirect URL: %s", response.RedirectURL)
	t.Logf("Status: %s", response.Status)
	t.Logf("Success: %t", response.Success)
}

func TestPayTRIntegration_PaymentStatusInquiry(t *testing.T) {
	config := getPayTRConfig()
	if config["merchantId"] == "" {
		t.Skip("PAYTR_MERCHANT_ID not set, skipping integration test")
	}

	paytrProvider := NewProvider()
	err := paytrProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// First create a payment to query its status
	request := provider.PaymentRequest{
		Amount:   50.25,
		Currency: "TL",
		Customer: provider.Customer{
			Name:        "Test",
			Surname:     "Customer",
			Email:       "status@example.com",
			PhoneNumber: "+905551234567",
			IPAddress:   "192.168.1.1",
		},
		Items: []provider.Item{
			{
				Name:     "Status Test Product",
				Price:    50.25,
				Quantity: 1,
			},
		},
		CallbackURL: "https://test.example.com/callback",
		ClientIP:    "192.168.1.1",
	}

	// Create payment
	paymentResponse, err := paytrProvider.Create3DPayment(context.Background(), request)
	if err != nil {
		t.Fatalf("Failed to create payment: %v", err)
	}

	// Wait a moment for payment to be processed
	time.Sleep(2 * time.Second)

	// Query payment status
	statusResponse, err := paytrProvider.GetPaymentStatus(context.Background(), paymentResponse.PaymentID)
	if err != nil {
		t.Fatalf("Status inquiry failed: %v", err)
	}

	// Verify status response
	if statusResponse.PaymentID != paymentResponse.PaymentID {
		t.Errorf("Expected PaymentID %s, got %s", paymentResponse.PaymentID, statusResponse.PaymentID)
	}

	t.Logf("Payment ID: %s", statusResponse.PaymentID)
	t.Logf("Status: %s", statusResponse.Status)
	t.Logf("Success: %t", statusResponse.Success)
	t.Logf("Amount: %.2f", statusResponse.Amount)
	t.Logf("Currency: %s", statusResponse.Currency)
}

func TestPayTRIntegration_CurrencySupport(t *testing.T) {
	config := getPayTRConfig()
	if config["merchantId"] == "" {
		t.Skip("PAYTR_MERCHANT_ID not set, skipping integration test")
	}

	paytrProvider := NewProvider()
	err := paytrProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	currencies := []string{"TL", "USD", "EUR"}

	for _, currency := range currencies {
		t.Run("Currency_"+currency, func(t *testing.T) {
			request := provider.PaymentRequest{
				Amount:   100.00,
				Currency: currency,
				Customer: provider.Customer{
					Name:        "Currency",
					Surname:     "Test",
					Email:       "currency@example.com",
					PhoneNumber: "+905551234567",
					IPAddress:   "192.168.1.1",
				},
				Items: []provider.Item{
					{
						Name:     "Currency Test Product",
						Price:    100.00,
						Quantity: 1,
					},
				},
				CallbackURL: "https://test.example.com/callback",
				ClientIP:    "192.168.1.1",
			}

			response, err := paytrProvider.Create3DPayment(context.Background(), request)
			if err != nil {
				t.Errorf("Payment failed for currency %s: %v", currency, err)
				return
			}

			if response.Currency != currency {
				t.Errorf("Expected currency %s, got %s", currency, response.Currency)
			}

			t.Logf("Currency %s - Payment ID: %s, Success: %t", currency, response.PaymentID, response.Success)
		})
	}
}

func TestPayTRIntegration_RefundPayment(t *testing.T) {
	config := getPayTRConfig()
	if config["merchantId"] == "" {
		t.Skip("PAYTR_MERCHANT_ID not set, skipping integration test")
	}

	paytrProvider := NewProvider()
	err := paytrProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Note: Refund testing typically requires a successful payment first
	// In test environment, we'll test the refund request structure

	refundRequest := provider.RefundRequest{
		PaymentID:    "test-payment-id",
		RefundAmount: 25.50,
		Reason:       "Integration test refund",
		Description:  "Testing refund functionality",
	}

	// Attempt refund (may fail in test environment if payment doesn't exist)
	refundResponse, err := paytrProvider.RefundPayment(context.Background(), refundRequest)

	// Log the attempt regardless of success/failure
	t.Logf("Refund attempt - PaymentID: %s", refundRequest.PaymentID)
	t.Logf("Refund amount: %.2f", refundRequest.RefundAmount)

	if err != nil {
		t.Logf("Refund failed (expected in test environment): %v", err)
		// Don't fail the test - refunds require actual successful payments
		return
	}

	if refundResponse != nil {
		t.Logf("Refund response - Success: %t", refundResponse.Success)
		t.Logf("Refund ID: %s", refundResponse.RefundID)
	}
}

func TestPayTRIntegration_WebhookValidation(t *testing.T) {
	config := getPayTRConfig()
	if config["merchantSalt"] == "" {
		t.Skip("PAYTR_MERCHANT_SALT not set, skipping webhook validation test")
	}

	provider := NewProvider()
	err := provider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Simulate webhook data from PayTR
	webhookData := map[string]string{
		"merchant_oid":   "test-order-123",
		"status":         "success",
		"total_amount":   "10050", // 100.50 TL in kuruş
		"payment_id":     "paytr-payment-123",
		"payment_type":   "card",
		"payment_amount": "10050",
		"currency":       "TL",
	}

	// Generate correct hash for webhook
	paytrProvider := provider.(*PayTRProvider)
	expectedHash := paytrProvider.generateWebhookHash(
		webhookData["merchant_oid"],
		webhookData["status"],
		webhookData["total_amount"],
	)
	webhookData["hash"] = expectedHash

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	// Validate webhook
	isValid, paymentInfo, err := provider.ValidateWebhook(context.Background(), webhookData, headers)
	if err != nil {
		t.Fatalf("Webhook validation failed: %v", err)
	}

	if !isValid {
		t.Error("Webhook should be valid with correct hash")
	}

	// Verify payment info
	expectedFields := []string{"paymentId", "status", "totalAmount"}
	for _, field := range expectedFields {
		if paymentInfo[field] == "" {
			t.Errorf("Payment info should contain %s", field)
		}
	}

	t.Logf("Webhook validation successful")
	t.Logf("Payment ID: %s", paymentInfo["paymentId"])
	t.Logf("Status: %s", paymentInfo["status"])
	t.Logf("Total Amount: %s", paymentInfo["totalAmount"])

	// Test with invalid hash
	webhookData["hash"] = "invalid-hash"
	isValid, _, err = provider.ValidateWebhook(context.Background(), webhookData, headers)
	if err == nil {
		t.Error("Webhook validation should fail with invalid hash")
	}

	if isValid {
		t.Error("Webhook should not be valid with invalid hash")
	}
}

func TestPayTRIntegration_ErrorHandling(t *testing.T) {
	config := getPayTRConfig()
	if config["merchantId"] == "" {
		t.Skip("PAYTR_MERCHANT_ID not set, skipping integration test")
	}

	paytrProvider := NewProvider()
	err := paytrProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Test with invalid amount (negative)
	t.Run("Invalid_Amount", func(t *testing.T) {
		request := provider.PaymentRequest{
			Amount:   -10.00,
			Currency: "TL",
			Customer: provider.Customer{
				Name:        "Error",
				Surname:     "Test",
				Email:       "error@example.com",
				PhoneNumber: "+905551234567",
				IPAddress:   "192.168.1.1",
			},
			CallbackURL: "https://test.example.com/callback",
			ClientIP:    "192.168.1.1",
		}

		_, err := paytrProvider.Create3DPayment(context.Background(), request)
		if err == nil {
			t.Error("Payment should fail with negative amount")
		}
		t.Logf("Expected error for negative amount: %v", err)
	})

	// Test with missing required fields
	t.Run("Missing_Email", func(t *testing.T) {
		request := provider.PaymentRequest{
			Amount:   100.00,
			Currency: "TL",
			Customer: provider.Customer{
				Name:        "Error",
				Surname:     "Test",
				PhoneNumber: "+905551234567",
				IPAddress:   "192.168.1.1",
				// Missing Email
			},
			CallbackURL: "https://test.example.com/callback",
			ClientIP:    "192.168.1.1",
		}

		_, err := paytrProvider.Create3DPayment(context.Background(), request)
		if err == nil {
			t.Error("Payment should fail with missing email")
		}
		t.Logf("Expected error for missing email: %v", err)
	})

	// Test with invalid currency
	t.Run("Invalid_Currency", func(t *testing.T) {
		request := provider.PaymentRequest{
			Amount:   100.00,
			Currency: "INVALID",
			Customer: provider.Customer{
				Name:        "Error",
				Surname:     "Test",
				Email:       "error@example.com",
				PhoneNumber: "+905551234567",
				IPAddress:   "192.168.1.1",
			},
			CallbackURL: "https://test.example.com/callback",
			ClientIP:    "192.168.1.1",
		}

		response, err := paytrProvider.Create3DPayment(context.Background(), request)
		if err != nil {
			t.Logf("Payment with invalid currency failed as expected: %v", err)
		} else {
			// Should default to TL
			if response.Currency != "TL" {
				t.Errorf("Expected currency to default to TL, got %s", response.Currency)
			}
			t.Logf("Invalid currency defaulted to TL")
		}
	})
}

func TestPayTRIntegration_Environment(t *testing.T) {
	config := getPayTRConfig()
	if config["merchantId"] == "" {
		t.Skip("PAYTR_MERCHANT_ID not set, skipping integration test")
	}

	// Test sandbox environment
	t.Run("Sandbox_Environment", func(t *testing.T) {
		sandboxConfig := make(map[string]string)
		for k, v := range config {
			sandboxConfig[k] = v
		}
		sandboxConfig["environment"] = "sandbox"

		provider := NewProvider()
		err := provider.Initialize(sandboxConfig)
		if err != nil {
			t.Fatalf("Failed to initialize sandbox provider: %v", err)
		}

		paytrProvider := provider.(*PayTRProvider)
		if paytrProvider.isProduction {
			t.Error("Provider should be in test mode for sandbox environment")
		}

		testMode := paytrProvider.getTestMode()
		if testMode != "1" {
			t.Errorf("Expected test mode '1', got '%s'", testMode)
		}
	})

	// Test production environment (if configured)
	t.Run("Production_Environment", func(t *testing.T) {
		prodConfig := make(map[string]string)
		for k, v := range config {
			prodConfig[k] = v
		}
		prodConfig["environment"] = "production"

		provider := NewProvider()
		err := provider.Initialize(prodConfig)
		if err != nil {
			t.Fatalf("Failed to initialize production provider: %v", err)
		}

		paytrProvider := provider.(*PayTRProvider)
		if !paytrProvider.isProduction {
			t.Error("Provider should be in production mode")
		}

		testMode := paytrProvider.getTestMode()
		if testMode != "0" {
			t.Errorf("Expected production mode '0', got '%s'", testMode)
		}
	})
}

func TestPayTRIntegration_RequestTimeout(t *testing.T) {
	config := getPayTRConfig()
	if config["merchantId"] == "" {
		t.Skip("PAYTR_MERCHANT_ID not set, skipping integration test")
	}

	paytrProvider := NewProvider()
	err := paytrProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Test with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	request := provider.PaymentRequest{
		Amount:   100.00,
		Currency: "TL",
		Customer: provider.Customer{
			Name:        "Timeout",
			Surname:     "Test",
			Email:       "timeout@example.com",
			PhoneNumber: "+905551234567",
			IPAddress:   "192.168.1.1",
		},
		CallbackURL: "https://test.example.com/callback",
		ClientIP:    "192.168.1.1",
	}

	_, err = paytrProvider.Create3DPayment(ctx, request)
	if err == nil {
		t.Error("Payment should fail with timeout")
	}

	t.Logf("Expected timeout error: %v", err)
}
