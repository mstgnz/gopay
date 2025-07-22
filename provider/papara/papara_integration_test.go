package papara

import (
	"context"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// Papara public test credentials
const (
	testAPIKey = "test-api-key-papara-12345"
)

func TestPaparaProvider_Integration(t *testing.T) {
	// Initialize provider
	p := NewProvider().(*PaparaProvider)
	config := map[string]string{
		"apiKey":       testAPIKey,
		"environment":  "sandbox", // Use sandbox for testing
		"gopayBaseURL": "https://test.gopay.com",
	}

	err := p.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize Papara provider: %v", err)
	}

	ctx := context.Background()

	t.Run("CreatePayment", func(t *testing.T) {
		request := provider.PaymentRequest{
			ID:          "test-payment-" + time.Now().Format("20060102150405"),
			ReferenceID: "ref-" + time.Now().Format("20060102150405"),
			Amount:      10.50,
			Currency:    "TRY",
			Description: "Test payment for Papara integration",
			Customer: provider.Customer{
				ID:      "customer-123",
				Name:    "John",
				Surname: "Doe",
				Email:   "john.doe@example.com",
				Address: &provider.Address{
					City:    "Istanbul",
					Country: "Turkey",
					Address: "Test Address 123",
					ZipCode: "34000",
				},
			},
			Items: []provider.Item{
				{
					ID:          "item-1",
					Name:        "Test Item",
					Description: "Test item description",
					Category:    "Test Category",
					Price:       10.50,
					Quantity:    1,
				},
			},
		}

		response, err := p.CreatePayment(ctx, request)
		if err != nil {
			t.Fatalf("CreatePayment failed: %v", err)
		}

		if response == nil {
			t.Fatal("Response is nil")
		}

		if response.PaymentID == "" {
			t.Error("PaymentID should not be empty")
		}

		if response.Amount != request.Amount {
			t.Errorf("Expected amount %v, got %v", request.Amount, response.Amount)
		}

		if response.Currency != request.Currency {
			t.Errorf("Expected currency %v, got %v", request.Currency, response.Currency)
		}

		// Test GetPaymentStatus with the created payment
		t.Run("GetPaymentStatus", func(t *testing.T) {
			statusResponse, err := p.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: response.PaymentID})
			if err != nil {
				t.Fatalf("GetPaymentStatus failed: %v", err)
			}

			if statusResponse == nil {
				t.Fatal("Status response is nil")
			}

			if statusResponse.PaymentID != response.PaymentID {
				t.Errorf("Expected paymentID %v, got %v", response.PaymentID, statusResponse.PaymentID)
			}
		})
	})

	t.Run("Create3DPayment", func(t *testing.T) {
		request := provider.PaymentRequest{
			ID:          "test-3d-payment-" + time.Now().Format("20060102150405"),
			ReferenceID: "ref-3d-" + time.Now().Format("20060102150405"),
			Amount:      25.75,
			Currency:    "TRY",
			Description: "Test 3D payment for Papara integration",
			Use3D:       true,
			CallbackURL: "https://example.com/callback",
			Customer: provider.Customer{
				ID:      "customer-456",
				Name:    "Jane",
				Surname: "Smith",
				Email:   "jane.smith@example.com",
				Address: &provider.Address{
					City:    "Ankara",
					Country: "Turkey",
					Address: "Test Address 456",
					ZipCode: "06000",
				},
			},
		}

		response, err := p.Create3DPayment(ctx, request)
		if err != nil {
			t.Fatalf("Create3DPayment failed: %v", err)
		}

		if response == nil {
			t.Fatal("Response is nil")
		}

		if response.PaymentID == "" {
			t.Error("PaymentID should not be empty")
		}

		// For 3D payments, we should get a redirect URL or HTML content
		if response.RedirectURL == "" && response.HTML == "" {
			t.Error("For 3D payments, either RedirectURL or HTML should be provided")
		}
	})

	t.Run("RefundPayment", func(t *testing.T) {
		// First create a payment to refund
		request := provider.PaymentRequest{
			ID:          "test-refund-payment-" + time.Now().Format("20060102150405"),
			ReferenceID: "ref-refund-" + time.Now().Format("20060102150405"),
			Amount:      15.25,
			Currency:    "TRY",
			Description: "Test payment for refund",
			Customer: provider.Customer{
				ID:      "customer-789",
				Name:    "Bob",
				Surname: "Johnson",
				Email:   "bob.johnson@example.com",
			},
		}

		paymentResponse, err := p.CreatePayment(ctx, request)
		if err != nil {
			t.Fatalf("CreatePayment failed: %v", err)
		}

		// Note: In a real scenario, the payment should be completed before refunding
		// For testing purposes, we'll attempt a refund regardless of status

		refundRequest := provider.RefundRequest{
			PaymentID:      paymentResponse.PaymentID,
			RefundAmount:   5.00, // Partial refund
			Reason:         "Test refund",
			Description:    "Integration test refund",
			Currency:       request.Currency,
			ConversationID: "refund-conv-" + time.Now().Format("20060102150405"),
		}

		refundResponse, err := p.RefundPayment(ctx, refundRequest)
		if err != nil {
			// Refund might fail if payment is not completed, which is expected in tests
			t.Logf("RefundPayment failed (expected in test): %v", err)
			return
		}

		if refundResponse == nil {
			t.Fatal("Refund response is nil")
		}

		if refundResponse.PaymentID != paymentResponse.PaymentID {
			t.Errorf("Expected paymentID %v, got %v", paymentResponse.PaymentID, refundResponse.PaymentID)
		}

		if refundResponse.RefundAmount != refundRequest.RefundAmount {
			t.Errorf("Expected refund amount %v, got %v", refundRequest.RefundAmount, refundResponse.RefundAmount)
		}
	})

	t.Run("ValidateWebhook", func(t *testing.T) {
		// Test webhook validation with sample data
		testPayload := `{"paymentId":"test-123","status":"COMPLETED","amount":100.50}`

		// Generate signature using the provider's method
		expectedSignature := p.generateWebhookSignature(testPayload)

		headers := map[string]string{
			"X-Papara-Signature": expectedSignature,
			"Content-Type":       "application/json",
		}

		data := map[string]string{
			"payload": testPayload,
		}

		isValid, webhookData, err := p.ValidateWebhook(ctx, data, headers)
		if err != nil {
			t.Fatalf("ValidateWebhook failed: %v", err)
		}

		if !isValid {
			t.Error("Webhook validation should pass with correct signature")
		}

		if webhookData["paymentId"] != "test-123" {
			t.Errorf("Expected paymentId 'test-123', got %v", webhookData["paymentId"])
		}

		if webhookData["status"] != "COMPLETED" {
			t.Errorf("Expected status 'COMPLETED', got %v", webhookData["status"])
		}

		// Test with invalid signature
		headers["X-Papara-Signature"] = "invalid-signature"
		isValid, _, err = p.ValidateWebhook(ctx, data, headers)
		if err == nil {
			t.Error("ValidateWebhook should fail with invalid signature")
		}

		if isValid {
			t.Error("Webhook validation should fail with invalid signature")
		}
	})

	t.Run("ValidateAccountNumber", func(t *testing.T) {
		resp, err := p.ValidateAccountNumber(ctx, "9087654321")
		if err != nil {
			t.Fatalf("ValidateAccountNumber failed: %v", err)
		}
		if resp == nil || !resp.Succeeded {
			t.Error("ValidateAccountNumber should succeed for test account number")
		}
	})

	t.Run("ValidatePhoneNumber", func(t *testing.T) {
		resp, err := p.ValidatePhoneNumber(ctx, "905555555555")
		if err != nil {
			t.Fatalf("ValidatePhoneNumber failed: %v", err)
		}
		if resp == nil || !resp.Succeeded {
			t.Error("ValidatePhoneNumber should succeed for test phone number")
		}
	})

	t.Run("ValidateTCKN", func(t *testing.T) {
		resp, err := p.ValidateTCKN(ctx, "21111111888")
		if err != nil {
			t.Fatalf("ValidateTCKN failed: %v", err)
		}
		if resp == nil || !resp.Succeeded {
			t.Error("ValidateTCKN should succeed for test tckn")
		}
	})

	t.Run("GetAccountInfo", func(t *testing.T) {
		resp, err := p.GetAccountInfo(ctx)
		if err != nil {
			t.Fatalf("GetAccountInfo failed: %v", err)
		}
		if resp == nil || !resp.Succeeded {
			t.Error("GetAccountInfo should succeed for test account")
		}
	})
}

func TestPaparaProvider_ErrorHandling(t *testing.T) {
	// Test error handling without actual API calls
	p := NewProvider().(*PaparaProvider)
	ctx := context.Background()

	t.Run("CreatePayment_InvalidRequest", func(t *testing.T) {
		invalidRequest := provider.PaymentRequest{
			Amount:   0, // Invalid amount
			Currency: "",
		}

		_, err := p.CreatePayment(ctx, invalidRequest)
		if err == nil {
			t.Error("CreatePayment should fail with invalid request")
		}
	})

	t.Run("GetPaymentStatus_EmptyPaymentID", func(t *testing.T) {
		_, err := p.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: ""})
		if err == nil {
			t.Error("GetPaymentStatus should fail with empty paymentID")
		}
	})

	t.Run("CancelPayment_EmptyPaymentID", func(t *testing.T) {
		_, err := p.CancelPayment(ctx, provider.CancelRequest{PaymentID: "", Reason: "test reason"})
		if err == nil {
			t.Error("CancelPayment should fail with empty paymentID")
		}
	})

	t.Run("RefundPayment_EmptyPaymentID", func(t *testing.T) {
		refundRequest := provider.RefundRequest{
			PaymentID: "", // Empty payment ID
		}

		_, err := p.RefundPayment(ctx, refundRequest)
		if err == nil {
			t.Error("RefundPayment should fail with empty paymentID")
		}
	})

	t.Run("Complete3DPayment_EmptyPaymentID", func(t *testing.T) {
		emptyCallbackState := &provider.CallbackState{
			PaymentID: "",
			TenantID:  1,
		}
		_, err := p.Complete3DPayment(ctx, emptyCallbackState, map[string]string{})
		if err == nil {
			t.Error("Complete3DPayment should fail with empty paymentID")
		}
	})
}
