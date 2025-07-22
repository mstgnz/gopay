// Stripe Integration Tests
// These tests make real API calls to Stripe's test environment.
// Uses Stripe's public test credentials for testing.
// Run: go test -v ./provider/stripe/ -run Integration

package stripe

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// Stripe public test credentials (safe for testing)
const (
	testSecretKey = "sk_test_26PHem9AhJZvU623DfE1x4sd"
	testPublicKey = "pk_test_TYooMQauvdEDq54NiTphI7jx"
)

func getStripeConfig() map[string]string {
	return map[string]string{
		"secretKey":    testSecretKey,
		"publicKey":    testPublicKey,
		"environment":  "sandbox", // Use sandbox/test environment
		"gopayBaseURL": "http://localhost:9999",
	}
}

func TestStripeIntegration_DirectPayment(t *testing.T) {
	config := getStripeConfig()

	stripeProvider := NewProvider()
	err := stripeProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize Stripe provider: %v", err)
	}

	request := provider.PaymentRequest{
		Amount:      25.99,
		Currency:    "USD",
		Description: "Integration test payment",
		ReferenceID: "int_test_" + time.Now().Format("20060102150405"),
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "User",
			Email:   "test@example.com",
			Address: &provider.Address{
				Address: "123 Test St",
				City:    "New York",
				Country: "US",
				ZipCode: "10001",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test User",
			CardNumber:     "4242424242424242", // Stripe test card
			ExpireMonth:    "12",
			ExpireYear:     "2028",
			CVV:            "123",
		},
	}

	ctx := context.Background()
	response, err := stripeProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("Payment failed: %v", err)
	}

	t.Logf("Payment Response: %+v", response)

	if !response.Success {
		t.Errorf("Payment should succeed, got: %s", response.Message)
	}

	if response.Status != provider.StatusSuccessful {
		t.Errorf("Expected status %s, got %s", provider.StatusSuccessful, response.Status)
	}

	if response.PaymentID == "" {
		t.Error("PaymentID should not be empty")
	}

	if response.Amount != request.Amount {
		t.Errorf("Expected amount %.2f, got %.2f", request.Amount, response.Amount)
	}

	if response.Currency != request.Currency {
		t.Errorf("Expected currency %s, got %s", request.Currency, response.Currency)
	}

	// Test payment status inquiry
	statusResponse, err := stripeProvider.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: response.PaymentID})
	if err != nil {
		t.Errorf("Failed to get payment status: %v", err)
	} else {
		t.Logf("Status Response: %+v", statusResponse)
		if statusResponse.PaymentID != response.PaymentID {
			t.Errorf("PaymentID mismatch in status response")
		}
	}
}

func TestStripeIntegration_DeclinedPayment(t *testing.T) {
	config := getStripeConfig()

	stripeProvider := NewProvider()
	err := stripeProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize Stripe provider: %v", err)
	}

	request := provider.PaymentRequest{
		Amount:      15.00,
		Currency:    "USD",
		Description: "Declined payment test",
		ReferenceID: "decline_test_" + time.Now().Format("20060102150405"),
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "User",
			Email:   "test@example.com",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test User",
			CardNumber:     "4000000000000002", // Stripe declined test card
			ExpireMonth:    "12",
			ExpireYear:     "2028",
			CVV:            "123",
		},
	}

	ctx := context.Background()
	response, err := stripeProvider.CreatePayment(ctx, request)

	// For declined cards, Stripe typically returns an error
	// This is expected behavior for a declined card
	if err != nil {
		t.Logf("Expected declined payment error: %v", err)
		// Verify error contains card declined information
		if !strings.Contains(err.Error(), "card_declined") && !strings.Contains(err.Error(), "declined") {
			t.Errorf("Expected card declined error, got: %v", err)
		}
		return
	}

	// If we get a response instead of an error, it should indicate failure
	t.Logf("Declined Payment Response: %+v", response)

	if response.Success {
		t.Error("Payment should be declined")
	}

	if response.Status == provider.StatusSuccessful {
		t.Errorf("Payment should not be successful")
	}
}

func TestStripeIntegration_3DSecurePayment(t *testing.T) {
	config := getStripeConfig()

	stripeProvider := NewProvider()
	err := stripeProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize Stripe provider: %v", err)
	}

	request := provider.PaymentRequest{
		Amount:      50.00,
		Currency:    "USD",
		Description: "3D Secure test payment",
		ReferenceID: "3d_test_" + time.Now().Format("20060102150405"),
		Use3D:       true,
		CallbackURL: "https://test.example.com/callback",
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "User",
			Email:   "test@example.com",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test User",
			CardNumber:     "4000000000003220", // Stripe 3D Secure test card
			ExpireMonth:    "12",
			ExpireYear:     "2028",
			CVV:            "123",
		},
	}

	ctx := context.Background()
	response, err := stripeProvider.Create3DPayment(ctx, request)

	if err != nil {
		t.Fatalf("3D Payment failed: %v", err)
	}

	t.Logf("3D Payment Response: %+v", response)

	if !response.Success {
		t.Errorf("3D Payment should start successfully, got: %s", response.Message)
	}

	if response.Status != provider.StatusPending {
		t.Errorf("Expected status %s, got %s", provider.StatusPending, response.Status)
	}

	if response.PaymentID == "" {
		t.Error("PaymentID should not be empty")
	}

	if response.RedirectURL == "" {
		t.Error("RedirectURL should not be empty for 3D Secure")
	}

	t.Logf("3D Secure Redirect URL: %s", response.RedirectURL)
}

func TestStripeIntegration_PaymentCancellation(t *testing.T) {
	config := getStripeConfig()

	stripeProvider := NewProvider()
	err := stripeProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize Stripe provider: %v", err)
	}

	// First create a payment intent that can be cancelled
	request := provider.PaymentRequest{
		Amount:      30.00,
		Currency:    "USD",
		Description: "Cancellation test payment",
		ReferenceID: "cancel_test_" + time.Now().Format("20060102150405"),
		Use3D:       true, // 3D payments start in pending state and can be cancelled
		CallbackURL: "https://test.example.com/callback",
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "User",
			Email:   "test@example.com",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test User",
			CardNumber:     "4000000000003220",
			ExpireMonth:    "12",
			ExpireYear:     "2028",
			CVV:            "123",
		},
	}

	ctx := context.Background()
	response, err := stripeProvider.Create3DPayment(ctx, request)

	if err != nil {
		t.Fatalf("Failed to create payment for cancellation test: %v", err)
	}

	if !response.Success || response.PaymentID == "" {
		t.Fatalf("Payment creation failed, cannot test cancellation")
	}

	t.Logf("Created payment for cancellation: %s", response.PaymentID)

	// Now cancel the payment
	cancelResponse, err := stripeProvider.CancelPayment(ctx, provider.CancelRequest{PaymentID: response.PaymentID, Reason: "Integration test cancellation"})

	if err != nil {
		t.Errorf("Failed to cancel payment: %v", err)
	} else {
		t.Logf("Cancel Response: %+v", cancelResponse)

		if cancelResponse.Success && cancelResponse.Status == provider.StatusCancelled {
			t.Log("Payment cancelled successfully")
		} else {
			t.Logf("Cancel response: success=%v, status=%s", cancelResponse.Success, cancelResponse.Status)
		}
	}
}

func TestStripeIntegration_PaymentRefund(t *testing.T) {
	config := getStripeConfig()

	stripeProvider := NewProvider()
	err := stripeProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize Stripe provider: %v", err)
	}

	// First create a successful payment
	request := provider.PaymentRequest{
		Amount:      40.00,
		Currency:    "USD",
		Description: "Refund test payment",
		ReferenceID: "refund_test_" + time.Now().Format("20060102150405"),
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "User",
			Email:   "test@example.com",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test User",
			CardNumber:     "4242424242424242",
			ExpireMonth:    "12",
			ExpireYear:     "2028",
			CVV:            "123",
		},
	}

	ctx := context.Background()
	response, err := stripeProvider.CreatePayment(ctx, request)

	if err != nil || !response.Success {
		t.Fatalf("Failed to create payment for refund test: %v, success: %v", err, response.Success)
	}

	t.Logf("Created payment for refund: %s", response.PaymentID)

	// Wait a moment for the payment to be fully processed
	time.Sleep(2 * time.Second)

	// Now refund the payment (partial refund)
	refundRequest := provider.RefundRequest{
		PaymentID:    response.PaymentID,
		RefundAmount: 20.00,                   // Partial refund
		Reason:       "requested_by_customer", // Use valid Stripe reason
		Description:  "Testing partial refund functionality",
	}

	refundResponse, err := stripeProvider.RefundPayment(ctx, refundRequest)

	if err != nil {
		t.Errorf("Failed to refund payment: %v", err)
	} else {
		t.Logf("Refund Response: %+v", refundResponse)

		if refundResponse.Success {
			t.Log("Payment refunded successfully")

			if refundResponse.RefundAmount != refundRequest.RefundAmount {
				t.Errorf("Expected refund amount %.2f, got %.2f",
					refundRequest.RefundAmount, refundResponse.RefundAmount)
			}

			if refundResponse.RefundID == "" {
				t.Error("RefundID should not be empty")
			}
		} else {
			t.Errorf("Refund failed: %s", refundResponse.Message)
		}
	}
}

func TestStripeIntegration_InvalidCard(t *testing.T) {
	config := getStripeConfig()

	stripeProvider := NewProvider()
	err := stripeProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize Stripe provider: %v", err)
	}

	request := provider.PaymentRequest{
		Amount:      10.00,
		Currency:    "USD",
		Description: "Invalid card test",
		ReferenceID: "invalid_test_" + time.Now().Format("20060102150405"),
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "User",
			Email:   "test@example.com",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "Test User",
			CardNumber:     "4000000000000127", // Stripe incorrect CVC test card
			ExpireMonth:    "12",
			ExpireYear:     "2028",
			CVV:            "123",
		},
	}

	ctx := context.Background()
	response, err := stripeProvider.CreatePayment(ctx, request)

	// For invalid cards, Stripe typically returns an error
	// This is expected behavior for an invalid card
	if err != nil {
		t.Logf("Expected invalid card error: %v", err)
		// Verify error contains card invalid information
		if !strings.Contains(err.Error(), "incorrect_cvc") && !strings.Contains(err.Error(), "invalid") {
			t.Errorf("Expected invalid card error, got: %v", err)
		}
		return
	}

	// If we get a response instead of an error, it should indicate failure
	t.Logf("Invalid Card Response: %+v", response)

	if response.Success {
		t.Error("Payment with invalid card should fail")
	}
}

func TestStripeIntegration_WebhookValidation(t *testing.T) {
	config := getStripeConfig()

	provider := NewProvider()
	err := provider.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize Stripe provider: %v", err)
	}

	// Test webhook validation
	webhookData := map[string]string{
		"id":   "evt_test_webhook",
		"type": "payment_intent.succeeded",
	}

	headers := map[string]string{
		"stripe-signature": "test_signature",
	}

	ctx := context.Background()
	isValid, resultData, err := provider.ValidateWebhook(ctx, webhookData, headers)

	if err != nil {
		t.Errorf("Webhook validation failed: %v", err)
	}

	if !isValid {
		t.Error("Webhook validation should pass")
	}

	if len(resultData) != len(webhookData) {
		t.Error("Result data should match input data")
	}

	t.Log("Webhook validation test passed")
}
