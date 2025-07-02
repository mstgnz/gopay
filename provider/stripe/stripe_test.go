package stripe

import (
	"context"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

func TestStripeProvider_Initialize(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]string
		wantErr bool
	}{
		{
			name: "Valid configuration",
			config: map[string]string{
				"secretKey":   "sk_test_123456789",
				"publicKey":   "pk_test_123456789",
				"environment": "sandbox",
			},
			wantErr: false,
		},
		{
			name: "Missing secret key",
			config: map[string]string{
				"publicKey":   "pk_test_123456789",
				"environment": "sandbox",
			},
			wantErr: true,
		},
		{
			name: "Production environment",
			config: map[string]string{
				"secretKey":   "sk_live_123456789",
				"publicKey":   "pk_live_123456789",
				"environment": "production",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider()
			err := p.Initialize(tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				stripeProvider := p.(*StripeProvider)
				if stripeProvider.secretKey != tt.config["secretKey"] {
					t.Errorf("secretKey = %v, want %v", stripeProvider.secretKey, tt.config["secretKey"])
				}
				if stripeProvider.publicKey != tt.config["publicKey"] {
					t.Errorf("publicKey = %v, want %v", stripeProvider.publicKey, tt.config["publicKey"])
				}
			}
		})
	}
}

func TestStripeProvider_validatePaymentRequest(t *testing.T) {
	p := &StripeProvider{}

	validRequest := provider.PaymentRequest{
		Amount:   100.0,
		Currency: "USD",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john.doe@example.com",
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     "4242424242424242",
			ExpireMonth:    "12",
			ExpireYear:     "2028",
			CVV:            "123",
		},
		CallbackURL: "https://example.com/callback",
	}

	tests := []struct {
		name    string
		request provider.PaymentRequest
		is3D    bool
		wantErr bool
	}{
		{
			name:    "Valid request",
			request: validRequest,
			is3D:    false,
			wantErr: false,
		},
		{
			name: "Zero amount",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Amount = 0
				return req
			}(),
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Missing currency",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Currency = ""
				return req
			}(),
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Missing customer email",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Customer.Email = ""
				return req
			}(),
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Missing card number",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.CardNumber = ""
				return req
			}(),
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Missing expiry month",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireMonth = ""
				return req
			}(),
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Missing expiry year",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireYear = ""
				return req
			}(),
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Missing CVV",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.CVV = ""
				return req
			}(),
			is3D:    false,
			wantErr: true,
		},
		{
			name: "3D without callback URL",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CallbackURL = ""
				return req
			}(),
			is3D:    true,
			wantErr: true,
		},
		{
			name:    "3D with callback URL",
			request: validRequest,
			is3D:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.validatePaymentRequest(tt.request, tt.is3D)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePaymentRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStripeProvider_mapToStripePaymentIntentRequest(t *testing.T) {
	p := &StripeProvider{
		gopayBaseURL: "https://gopay.example.com",
	}

	request := provider.PaymentRequest{
		Amount:         100.50,
		Currency:       "USD",
		Description:    "Test payment",
		ReferenceID:    "order_12345",
		ConversationID: "conv_67890",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john.doe@example.com",
			Address: provider.Address{
				Address: "123 Main St",
				City:    "New York",
				Country: "US",
				ZipCode: "10001",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     "4242424242424242",
			ExpireMonth:    "12",
			ExpireYear:     "2028",
			CVV:            "123",
		},
		CallbackURL: "https://example.com/callback",
	}

	t.Run("Non-3D payment", func(t *testing.T) {
		result := p.mapToStripePaymentIntentRequest(request, false)

		// Check amount conversion to cents
		if result["amount"] != int64(10050) {
			t.Errorf("Expected amount 10050, got %v", result["amount"])
		}

		// Check currency conversion to lowercase
		if result["currency"] != "usd" {
			t.Errorf("Expected currency 'usd', got %v", result["currency"])
		}

		// Check payment method data structure
		paymentMethodData, ok := result["payment_method_data"].(map[string]any)
		if !ok {
			t.Fatal("payment_method_data should be a map")
		}

		if paymentMethodData["type"] != "card" {
			t.Errorf("Expected payment method type 'card', got %v", paymentMethodData["type"])
		}

		// Check card data
		cardData, ok := paymentMethodData["card"].(map[string]any)
		if !ok {
			t.Fatal("card data should be a map")
		}

		if cardData["number"] != request.CardInfo.CardNumber {
			t.Errorf("Expected card number %s, got %v", request.CardInfo.CardNumber, cardData["number"])
		}

		// Check 3D Secure settings for non-3D
		paymentMethodOptions, ok := result["payment_method_options"].(map[string]any)
		if !ok {
			t.Fatal("payment_method_options should be a map")
		}

		cardOptions, ok := paymentMethodOptions["card"].(map[string]any)
		if !ok {
			t.Fatal("card options should be a map")
		}

		if cardOptions["request_three_d_secure"] != "automatic" {
			t.Errorf("Expected 3D secure 'automatic', got %v", cardOptions["request_three_d_secure"])
		}
	})

	t.Run("3D Secure payment", func(t *testing.T) {
		result := p.mapToStripePaymentIntentRequest(request, true)

		// Check 3D Secure settings
		paymentMethodOptions, ok := result["payment_method_options"].(map[string]any)
		if !ok {
			t.Fatal("payment_method_options should be a map")
		}

		cardOptions, ok := paymentMethodOptions["card"].(map[string]any)
		if !ok {
			t.Fatal("card options should be a map")
		}

		if cardOptions["request_three_d_secure"] != "any" {
			t.Errorf("Expected 3D secure 'any', got %v", cardOptions["request_three_d_secure"])
		}

		// Check return URL
		returnURL, ok := result["return_url"].(string)
		if !ok {
			t.Fatal("return_url should be a string")
		}

		expectedURL := "https://gopay.example.com/v1/callback/stripe?originalCallbackUrl=https://example.com/callback"
		if returnURL != expectedURL {
			t.Errorf("Expected return URL %s, got %s", expectedURL, returnURL)
		}
	})

	t.Run("Metadata mapping", func(t *testing.T) {
		result := p.mapToStripePaymentIntentRequest(request, false)

		metadata, ok := result["metadata"].(map[string]string)
		if !ok {
			t.Fatal("metadata should be a map[string]string")
		}

		if metadata["reference_id"] != request.ReferenceID {
			t.Errorf("Expected reference_id %s, got %s", request.ReferenceID, metadata["reference_id"])
		}

		if metadata["conversation_id"] != request.ConversationID {
			t.Errorf("Expected conversation_id %s, got %s", request.ConversationID, metadata["conversation_id"])
		}
	})
}

func TestStripeProvider_mapToPaymentResponse(t *testing.T) {
	p := &StripeProvider{}

	t.Run("Successful payment response", func(t *testing.T) {
		stripeResp := map[string]any{
			"id":       "pi_test_123456789",
			"amount":   int64(10050), // 100.50 USD in cents
			"currency": "usd",
			"status":   "succeeded",
			"charges": map[string]any{
				"data": []any{
					map[string]any{
						"id": "ch_test_123456789",
					},
				},
			},
		}

		result := p.mapToPaymentResponse(stripeResp, 200)

		if !result.Success {
			t.Error("Expected success to be true")
		}

		if result.Status != provider.StatusSuccessful {
			t.Errorf("Expected status %s, got %s", provider.StatusSuccessful, result.Status)
		}

		if result.PaymentID != "pi_test_123456789" {
			t.Errorf("Expected payment ID pi_test_123456789, got %s", result.PaymentID)
		}

		if result.Amount != 100.50 {
			t.Errorf("Expected amount 100.50, got %f", result.Amount)
		}

		if result.Currency != "USD" {
			t.Errorf("Expected currency USD, got %s", result.Currency)
		}

		if result.TransactionID != "ch_test_123456789" {
			t.Errorf("Expected transaction ID ch_test_123456789, got %s", result.TransactionID)
		}
	})

	t.Run("3D Secure required response", func(t *testing.T) {
		stripeResp := map[string]any{
			"id":       "pi_test_123456789",
			"amount":   int64(10050),
			"currency": "usd",
			"status":   "requires_action",
			"next_action": map[string]any{
				"redirect_to_url": map[string]any{
					"url": "https://js.stripe.com/v3/authorize-with-url...",
				},
			},
		}

		result := p.mapToPaymentResponse(stripeResp, 200)

		if !result.Success {
			t.Error("Expected success to be true")
		}

		if result.Status != provider.StatusPending {
			t.Errorf("Expected status %s, got %s", provider.StatusPending, result.Status)
		}

		if result.RedirectURL != "https://js.stripe.com/v3/authorize-with-url..." {
			t.Errorf("Expected redirect URL, got %s", result.RedirectURL)
		}
	})

	t.Run("Failed payment response", func(t *testing.T) {
		stripeResp := map[string]any{
			"error": map[string]any{
				"message": "Your card was declined.",
				"code":    "card_declined",
			},
		}

		result := p.mapToPaymentResponse(stripeResp, 402)

		if result.Success {
			t.Error("Expected success to be false")
		}

		if result.Status != provider.StatusFailed {
			t.Errorf("Expected status %s, got %s", provider.StatusFailed, result.Status)
		}

		if result.Message != "Your card was declined." {
			t.Errorf("Expected error message, got %s", result.Message)
		}

		if result.ErrorCode != "card_declined" {
			t.Errorf("Expected error code card_declined, got %s", result.ErrorCode)
		}
	})

	t.Run("Status mapping", func(t *testing.T) {
		statusTests := []struct {
			stripeStatus   string
			expectedStatus provider.PaymentStatus
		}{
			{"succeeded", provider.StatusSuccessful},
			{"requires_action", provider.StatusPending},
			{"requires_confirmation", provider.StatusPending},
			{"processing", provider.StatusProcessing},
			{"requires_capture", provider.StatusProcessing},
			{"canceled", provider.StatusCancelled},
			{"requires_payment_method", provider.StatusFailed},
		}

		for _, test := range statusTests {
			stripeResp := map[string]any{
				"id":     "pi_test_123456789",
				"status": test.stripeStatus,
			}

			result := p.mapToPaymentResponse(stripeResp, 200)

			if result.Status != test.expectedStatus {
				t.Errorf("For status %s, expected %s, got %s",
					test.stripeStatus, test.expectedStatus, result.Status)
			}
		}
	})
}

func TestStripeProvider_Complete3DPayment(t *testing.T) {
	p := &StripeProvider{}

	t.Run("Missing payment ID", func(t *testing.T) {
		ctx := context.Background()
		_, err := p.Complete3DPayment(ctx, "", "conv_123", map[string]string{})

		if err == nil {
			t.Error("Expected error for missing payment ID")
		}

		if err.Error() != "stripe: paymentID is required for 3D completion" {
			t.Errorf("Unexpected error message: %s", err.Error())
		}
	})
}

func TestStripeProvider_GetPaymentStatus(t *testing.T) {
	p := &StripeProvider{}

	t.Run("Missing payment ID", func(t *testing.T) {
		ctx := context.Background()
		_, err := p.GetPaymentStatus(ctx, "")

		if err == nil {
			t.Error("Expected error for missing payment ID")
		}

		if err.Error() != "stripe: paymentID is required" {
			t.Errorf("Unexpected error message: %s", err.Error())
		}
	})
}

func TestStripeProvider_CancelPayment(t *testing.T) {
	p := &StripeProvider{}

	t.Run("Missing payment ID", func(t *testing.T) {
		ctx := context.Background()
		_, err := p.CancelPayment(ctx, "", "test reason")

		if err == nil {
			t.Error("Expected error for missing payment ID")
		}

		if err.Error() != "stripe: paymentID is required" {
			t.Errorf("Unexpected error message: %s", err.Error())
		}
	})
}

func TestStripeProvider_RefundPayment(t *testing.T) {
	p := &StripeProvider{}

	t.Run("Missing payment ID", func(t *testing.T) {
		ctx := context.Background()
		request := provider.RefundRequest{
			PaymentID: "",
		}
		_, err := p.RefundPayment(ctx, request)

		if err == nil {
			t.Error("Expected error for missing payment ID")
		}

		if err.Error() != "stripe: paymentID is required for refund" {
			t.Errorf("Unexpected error message: %s", err.Error())
		}
	})
}

func TestStripeProvider_ValidateWebhook(t *testing.T) {
	p := &StripeProvider{}

	ctx := context.Background()
	data := map[string]string{"test": "data"}
	headers := map[string]string{"test": "header"}

	isValid, resultData, err := p.ValidateWebhook(ctx, data, headers)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !isValid {
		t.Error("Expected webhook validation to pass")
	}

	if len(resultData) != len(data) {
		t.Error("Expected result data to match input data")
	}
}
