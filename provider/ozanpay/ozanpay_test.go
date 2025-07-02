package ozanpay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

func TestNewProvider(t *testing.T) {
	provider := NewProvider()
	if provider == nil {
		t.Error("NewProvider() should not return nil")
	}

	ozanPayProvider, ok := provider.(*OzanPayProvider)
	if !ok {
		t.Error("NewProvider() should return *OzanPayProvider")
	}

	if ozanPayProvider.client == nil {
		t.Error("HTTP client should be initialized")
	}

	if ozanPayProvider.client.Timeout != defaultTimeout {
		t.Errorf("HTTP client timeout should be %v, got %v", defaultTimeout, ozanPayProvider.client.Timeout)
	}
}

func TestOzanPayProvider_Initialize(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]string
		expectError bool
		expectProd  bool
		expectURL   string
	}{
		{
			name: "Valid sandbox config",
			config: map[string]string{
				"apiKey":      "test-api-key",
				"secretKey":   "test-secret-key",
				"merchantId":  "test-merchant-id",
				"environment": "sandbox",
			},
			expectError: false,
			expectProd:  false,
			expectURL:   apiSandboxURL,
		},
		{
			name: "Valid production config",
			config: map[string]string{
				"apiKey":      "test-api-key",
				"secretKey":   "test-secret-key",
				"merchantId":  "test-merchant-id",
				"environment": "production",
			},
			expectError: false,
			expectProd:  true,
			expectURL:   apiProductionURL,
		},
		{
			name: "Default to sandbox",
			config: map[string]string{
				"apiKey":     "test-api-key",
				"secretKey":  "test-secret-key",
				"merchantId": "test-merchant-id",
			},
			expectError: false,
			expectProd:  false,
			expectURL:   apiSandboxURL,
		},
		{
			name: "Missing apiKey",
			config: map[string]string{
				"secretKey":   "test-secret-key",
				"merchantId":  "test-merchant-id",
				"environment": "sandbox",
			},
			expectError: true,
		},
		{
			name: "Missing secretKey",
			config: map[string]string{
				"apiKey":      "test-api-key",
				"merchantId":  "test-merchant-id",
				"environment": "sandbox",
			},
			expectError: true,
		},
		{
			name: "Missing merchantId",
			config: map[string]string{
				"apiKey":      "test-api-key",
				"secretKey":   "test-secret-key",
				"environment": "sandbox",
			},
			expectError: true,
		},
		{
			name:        "Empty config",
			config:      map[string]string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewProvider().(*OzanPayProvider)
			err := provider.Initialize(tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if provider.apiKey != tt.config["apiKey"] {
				t.Errorf("Expected apiKey %s, got %s", tt.config["apiKey"], provider.apiKey)
			}

			if provider.secretKey != tt.config["secretKey"] {
				t.Errorf("Expected secretKey %s, got %s", tt.config["secretKey"], provider.secretKey)
			}

			if provider.merchantID != tt.config["merchantId"] {
				t.Errorf("Expected merchantId %s, got %s", tt.config["merchantId"], provider.merchantID)
			}

			if provider.isProduction != tt.expectProd {
				t.Errorf("Expected isProduction %v, got %v", tt.expectProd, provider.isProduction)
			}

			if provider.baseURL != tt.expectURL {
				t.Errorf("Expected baseURL %s, got %s", tt.expectURL, provider.baseURL)
			}
		})
	}
}

func TestOzanPayProvider_ValidatePaymentRequest(t *testing.T) {
	ozanPayProvider := &OzanPayProvider{}

	validRequest := provider.PaymentRequest{
		Amount:   100.0,
		Currency: "USD",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  "4111111111111111",
			CVV:         "123",
			ExpireMonth: "12",
			ExpireYear:  "2030",
		},
		CallbackURL: "https://example.com/callback",
	}

	tests := []struct {
		name        string
		request     provider.PaymentRequest
		is3D        bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid request",
			request:     validRequest,
			is3D:        false,
			expectError: false,
		},
		{
			name:        "Valid 3D request",
			request:     validRequest,
			is3D:        true,
			expectError: false,
		},
		{
			name: "Invalid amount - zero",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Amount = 0
				return req
			}(),
			expectError: true,
			errorMsg:    "amount must be greater than 0",
		},
		{
			name: "Invalid amount - negative",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Amount = -10
				return req
			}(),
			expectError: true,
			errorMsg:    "amount must be greater than 0",
		},
		{
			name: "Missing currency",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Currency = ""
				return req
			}(),
			expectError: true,
			errorMsg:    "currency is required",
		},
		{
			name: "Missing customer email",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Customer.Email = ""
				return req
			}(),
			expectError: true,
			errorMsg:    "customer email is required",
		},
		{
			name: "Missing customer name",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Customer.Name = ""
				return req
			}(),
			expectError: true,
			errorMsg:    "customer name and surname are required",
		},
		{
			name: "Missing customer surname",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Customer.Surname = ""
				return req
			}(),
			expectError: true,
			errorMsg:    "customer name and surname are required",
		},
		{
			name: "Missing card number",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.CardNumber = ""
				return req
			}(),
			expectError: true,
			errorMsg:    "card number is required",
		},
		{
			name: "Missing CVV",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.CVV = ""
				return req
			}(),
			expectError: true,
			errorMsg:    "CVV is required",
		},
		{
			name: "Missing expire month",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireMonth = ""
				return req
			}(),
			expectError: true,
			errorMsg:    "card expiration month and year are required",
		},
		{
			name: "Missing expire year",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireYear = ""
				return req
			}(),
			expectError: true,
			errorMsg:    "card expiration month and year are required",
		},
		{
			name: "Missing callback URL for 3D",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CallbackURL = ""
				return req
			}(),
			is3D:        true,
			expectError: true,
			errorMsg:    "callback URL is required for 3D secure payments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ozanPayProvider.validatePaymentRequest(tt.request, tt.is3D)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestOzanPayProvider_MapToOzanPayRequest(t *testing.T) {
	ozanPayProvider := &OzanPayProvider{
		merchantID:   "test-merchant",
		gopayBaseURL: "https://test.gopay.com",
	}

	request := provider.PaymentRequest{
		Amount:   100.50,
		Currency: "USD",
		Customer: provider.Customer{
			ID:      "customer123",
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
			Address: provider.Address{
				City:    "New York",
				Country: "USA",
				Address: "Test Address",
				ZipCode: "10001",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     "4111111111111111",
			CVV:            "123",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
		},
		Items: []provider.Item{
			{
				ID:       "item1",
				Name:     "Test Item",
				Category: "Electronics",
				Price:    100.50,
			},
		},
		CallbackURL: "https://example.com/callback",
	}

	tests := []struct {
		name     string
		request  provider.PaymentRequest
		force3D  bool
		validate func(t *testing.T, result map[string]any)
	}{
		{
			name:    "Regular payment request",
			request: request,
			force3D: false,
			validate: func(t *testing.T, result map[string]any) {
				if result["amount"] != int64(10050) { // 100.50 * 100
					t.Errorf("Expected amount 10050, got %v", result["amount"])
				}
				if result["currency"] != "USD" {
					t.Errorf("Expected currency 'USD', got %v", result["currency"])
				}
				if result["merchantId"] != "test-merchant" {
					t.Errorf("Expected merchantId 'test-merchant', got %v", result["merchantId"])
				}

				customer, ok := result["customer"].(map[string]any)
				if !ok {
					t.Error("customer should be a map")
					return
				}
				if customer["email"] != "john@example.com" {
					t.Errorf("Expected customer email 'john@example.com', got %v", customer["email"])
				}

				card, ok := result["card"].(map[string]any)
				if !ok {
					t.Error("card should be a map")
					return
				}
				if card["number"] != "4111111111111111" {
					t.Errorf("Expected card number '4111111111111111', got %v", card["number"])
				}
			},
		},
		{
			name:    "3D payment request",
			request: request,
			force3D: true,
			validate: func(t *testing.T, result map[string]any) {
				secure3d, ok := result["secure3d"].(map[string]any)
				if !ok {
					t.Error("secure3d should be a map")
					return
				}
				if secure3d["enabled"] != true {
					t.Errorf("Expected secure3d enabled to be true, got %v", secure3d["enabled"])
				}

				expectedURL := "https://test.gopay.com/v1/callback/ozanpay?originalCallbackUrl=https://example.com/callback"
				if secure3d["returnUrl"] != expectedURL {
					t.Errorf("Expected returnUrl '%s', got %v", expectedURL, secure3d["returnUrl"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ozanPayProvider.mapToOzanPayRequest(tt.request, tt.force3D)
			tt.validate(t, result)
		})
	}
}

func TestOzanPayProvider_MapToPaymentResponse(t *testing.T) {
	ozanPayProvider := &OzanPayProvider{}

	tests := []struct {
		name            string
		response        map[string]any
		expectedStatus  provider.PaymentStatus
		expectedSuccess bool
		expectedMessage string
	}{
		{
			name: "Successful payment",
			response: map[string]any{
				"id":            "payment123",
				"transactionId": "txn456",
				"status":        statusApproved,
				"amount":        float64(10050), // 100.50 in minor units
				"currency":      "USD",
			},
			expectedStatus:  provider.StatusSuccessful,
			expectedSuccess: true,
			expectedMessage: "Payment successful",
		},
		{
			name: "Failed payment",
			response: map[string]any{
				"status":       statusFailed,
				"errorCode":    errorCodeInvalidCard,
				"errorMessage": "Invalid card number",
			},
			expectedStatus:  provider.StatusFailed,
			expectedSuccess: false,
			expectedMessage: "Invalid card number",
		},
		{
			name: "Pending payment with redirect",
			response: map[string]any{
				"id":          "payment789",
				"status":      statusPending,
				"redirectUrl": "https://3ds.ozanpay.com/redirect",
			},
			expectedStatus:  provider.StatusPending,
			expectedSuccess: false,
			expectedMessage: "Payment pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ozanPayProvider.mapToPaymentResponse(tt.response)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, result.Status)
			}

			if result.Success != tt.expectedSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectedSuccess, result.Success)
			}

			if result.Message != tt.expectedMessage {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMessage, result.Message)
			}
		})
	}
}

func TestOzanPayProvider_CreatePayment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"id":       "payment123",
			"status":   statusApproved,
			"amount":   float64(10050),
			"currency": "USD",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ozanPayProvider := &OzanPayProvider{
		apiKey:     "test-key",
		secretKey:  "test-secret",
		merchantID: "test-merchant",
		baseURL:    server.URL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	request := provider.PaymentRequest{
		Amount:   100.50,
		Currency: "USD",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  "4111111111111111",
			CVV:         "123",
			ExpireMonth: "12",
			ExpireYear:  "2030",
		},
	}

	ctx := context.Background()
	response, err := ozanPayProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if !response.Success {
		t.Error("Expected successful payment")
	}

	if response.PaymentID != "payment123" {
		t.Errorf("Expected payment ID 'payment123', got %s", response.PaymentID)
	}
}

func TestOzanPayProvider_Create3DPayment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"id":          "payment456",
			"status":      statusPending,
			"redirectUrl": "https://3ds.ozanpay.com/redirect",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ozanPayProvider := &OzanPayProvider{
		apiKey:       "test-key",
		secretKey:    "test-secret",
		merchantID:   "test-merchant",
		baseURL:      server.URL,
		gopayBaseURL: "https://test.gopay.com",
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	request := provider.PaymentRequest{
		Amount:   100.50,
		Currency: "USD",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  "4111111111111111",
			CVV:         "123",
			ExpireMonth: "12",
			ExpireYear:  "2030",
		},
		CallbackURL: "https://example.com/callback",
		Use3D:       true,
	}

	ctx := context.Background()
	response, err := ozanPayProvider.Create3DPayment(ctx, request)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if response.Status != provider.StatusPending {
		t.Errorf("Expected status pending, got %v", response.Status)
	}

	if response.RedirectURL != "https://3ds.ozanpay.com/redirect" {
		t.Errorf("Expected redirect URL, got %s", response.RedirectURL)
	}
}

func TestOzanPayProvider_GetPaymentStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"id":     "payment123",
			"status": statusApproved,
			"amount": float64(10050),
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ozanPayProvider := &OzanPayProvider{
		apiKey:     "test-key",
		secretKey:  "test-secret",
		merchantID: "test-merchant",
		baseURL:    server.URL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	ctx := context.Background()

	// Test with valid payment ID
	response, err := ozanPayProvider.GetPaymentStatus(ctx, "payment123")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if !response.Success {
		t.Error("Expected successful response")
	}

	// Test with empty payment ID
	_, err = ozanPayProvider.GetPaymentStatus(ctx, "")
	if err == nil {
		t.Error("Expected error for empty payment ID")
	}
}

func TestOzanPayProvider_RefundPayment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"id":     "refund123",
			"status": statusApproved,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ozanPayProvider := &OzanPayProvider{
		apiKey:     "test-key",
		secretKey:  "test-secret",
		merchantID: "test-merchant",
		baseURL:    server.URL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	refundRequest := provider.RefundRequest{
		PaymentID:    "payment123",
		RefundAmount: 50.0,
		Reason:       "Customer request",
	}

	ctx := context.Background()
	response, err := ozanPayProvider.RefundPayment(ctx, refundRequest)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if !response.Success {
		t.Error("Expected successful refund")
	}

	if response.RefundID != "refund123" {
		t.Errorf("Expected refund ID 'refund123', got %s", response.RefundID)
	}
}

func TestOzanPayProvider_GenerateSignature(t *testing.T) {
	ozanPayProvider := &OzanPayProvider{
		secretKey: "test-secret-key",
	}

	data := "POST/api/v1/payments2024-01-01T00:00:00Z{\"test\":\"data\"}"
	signature := ozanPayProvider.generateSignature(data)

	if signature == "" {
		t.Error("Signature should not be empty")
	}

	// Test that same data produces same signature
	signature2 := ozanPayProvider.generateSignature(data)
	if signature != signature2 {
		t.Error("Same data should produce same signature")
	}

	// Test that different data produces different signature
	signature3 := ozanPayProvider.generateSignature("different-data")
	if signature == signature3 {
		t.Error("Different data should produce different signature")
	}
}

func TestOzanPayProvider_ValidateWebhook(t *testing.T) {
	ozanPayProvider := &OzanPayProvider{
		secretKey: "test-secret-key",
	}

	data := map[string]string{
		"id":     "payment123",
		"status": "APPROVED",
	}

	// Calculate correct signature
	rawJson, _ := json.Marshal(data)
	correctSignature := ozanPayProvider.generateSignature(string(rawJson))

	tests := []struct {
		name        string
		data        map[string]string
		headers     map[string]string
		expectValid bool
		expectError bool
	}{
		{
			name: "Valid webhook",
			data: data,
			headers: map[string]string{
				"X-Ozan-Signature": correctSignature,
			},
			expectValid: true,
			expectError: false,
		},
		{
			name: "Invalid signature",
			data: data,
			headers: map[string]string{
				"X-Ozan-Signature": "invalid-signature",
			},
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Missing signature header",
			data:        data,
			headers:     map[string]string{},
			expectValid: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			valid, result, err := ozanPayProvider.ValidateWebhook(ctx, tt.data, tt.headers)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if valid != tt.expectValid {
				t.Errorf("Expected valid %v, got %v", tt.expectValid, valid)
			}

			if tt.expectValid {
				if result["paymentId"] != "payment123" {
					t.Errorf("Expected paymentId 'payment123', got %v", result["paymentId"])
				}
				if result["status"] != "APPROVED" {
					t.Errorf("Expected status 'APPROVED', got %v", result["status"])
				}
			}
		})
	}
}
