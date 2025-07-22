package payu

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
	p := NewProvider()
	if p == nil {
		t.Error("NewProvider should not return nil")
	}

	payuProvider, ok := p.(*PayUProvider)
	if !ok {
		t.Error("NewProvider should return *PayUProvider")
	}

	if payuProvider.client == nil {
		t.Error("PayU provider should have HTTP client initialized")
	}
}

func TestPayUProvider_Initialize(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]string
		wantErr bool
	}{
		{
			name: "Valid configuration",
			config: map[string]string{
				"merchantId":   "test-merchant-123",
				"secretKey":    "test-secret-key",
				"environment":  "sandbox",
				"gopayBaseURL": "https://test.gopay.com",
			},
			wantErr: false,
		},
		{
			name: "Production environment",
			config: map[string]string{
				"merchantId":  "prod-merchant-456",
				"secretKey":   "prod-secret-key",
				"environment": "production",
			},
			wantErr: false,
		},
		{
			name: "Missing merchantId",
			config: map[string]string{
				"secretKey":   "test-secret-key",
				"environment": "sandbox",
			},
			wantErr: true,
		},
		{
			name: "Missing secretKey",
			config: map[string]string{
				"merchantId":  "test-merchant-123",
				"environment": "sandbox",
			},
			wantErr: true,
		},
		{
			name:    "Empty configuration",
			config:  map[string]string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PayUProvider{}
			err := p.Initialize(tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if p.merchantID != tt.config["merchantId"] {
					t.Errorf("Expected merchantID %s, got %s", tt.config["merchantId"], p.merchantID)
				}
				if p.secretKey != tt.config["secretKey"] {
					t.Errorf("Expected secretKey %s, got %s", tt.config["secretKey"], p.secretKey)
				}

				expectedProduction := tt.config["environment"] == "production"
				if p.isProduction != expectedProduction {
					t.Errorf("Expected isProduction %v, got %v", expectedProduction, p.isProduction)
				}

				if expectedProduction && p.baseURL != apiProductionURL {
					t.Errorf("Expected production URL %s, got %s", apiProductionURL, p.baseURL)
				} else if !expectedProduction && p.baseURL != apiSandboxURL {
					t.Errorf("Expected sandbox URL %s, got %s", apiSandboxURL, p.baseURL)
				}
			}
		})
	}
}

func TestPayUProvider_validatePaymentRequest(t *testing.T) {
	p := &PayUProvider{}

	validRequest := provider.PaymentRequest{
		Amount:      100.0,
		Currency:    "TRY",
		ReferenceID: "order-123",
		Customer: provider.Customer{
			Email: "test@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  "5528790000000008",
			CVV:         "123",
			ExpireMonth: "12",
			ExpireYear:  "2030",
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
			name:    "Valid non-3D request",
			request: validRequest,
			is3D:    false,
			wantErr: false,
		},
		{
			name:    "Valid 3D request",
			request: validRequest,
			is3D:    true,
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
			name: "Negative amount",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Amount = -10.0
				return req
			}(),
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Missing reference ID",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.ReferenceID = ""
				return req
			}(),
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Missing callback URL for 3D",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CallbackURL = ""
				return req
			}(),
			is3D:    true,
			wantErr: true,
		},
		{
			name: "Invalid card number length - too short",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.CardNumber = "123456789012"
				return req
			}(),
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Invalid card number length - too long",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.CardNumber = "12345678901234567890"
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
			name: "Invalid CVV length - too short",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.CVV = "12"
				return req
			}(),
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Invalid CVV length - too long",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.CVV = "12345"
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
			name: "Invalid expiry month",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireMonth = "13"
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
			name: "Invalid expiry year",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireYear = "2019"
				return req
			}(),
			is3D:    false,
			wantErr: true,
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

func TestPayUProvider_mapToPayURequest(t *testing.T) {
	p := &PayUProvider{
		merchantID:   "test-merchant",
		gopayBaseURL: "https://test.gopay.com",
	}

	request := provider.PaymentRequest{
		Amount:         100.50,
		Currency:       "TRY",
		ReferenceID:    "order-123",
		Description:    "Test payment",
		ConversationID: "conv-456",
		Customer: provider.Customer{
			Email:       "test@example.com",
			PhoneNumber: "+905551234567",
		},
		CardInfo: provider.CardInfo{
			CardNumber:     "5528790000000008",
			CVV:            "123",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CardHolderName: "Test User",
		},
		CallbackURL: "https://merchant.com/callback",
	}

	tests := []struct {
		name    string
		is3D    bool
		request provider.PaymentRequest
	}{
		{
			name:    "Non-3D payment request",
			is3D:    false,
			request: request,
		},
		{
			name:    "3D payment request",
			is3D:    true,
			request: request,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.mapToPayURequest(tt.request, tt.is3D)

			// Check basic fields
			if result["merchantId"] != p.merchantID {
				t.Errorf("Expected merchantId %s, got %v", p.merchantID, result["merchantId"])
			}
			if result["amount"] != "100.50" {
				t.Errorf("Expected amount '100.50', got %v", result["amount"])
			}
			if result["currency"] != tt.request.Currency {
				t.Errorf("Expected currency %s, got %v", tt.request.Currency, result["currency"])
			}
			if result["orderId"] != tt.request.ReferenceID {
				t.Errorf("Expected orderId %s, got %v", tt.request.ReferenceID, result["orderId"])
			}

			// Check card details
			if card, ok := result["card"].(map[string]any); ok {
				if card["number"] != tt.request.CardInfo.CardNumber {
					t.Errorf("Expected card number %s, got %v", tt.request.CardInfo.CardNumber, card["number"])
				}
				if card["cvv"] != tt.request.CardInfo.CVV {
					t.Errorf("Expected CVV %s, got %v", tt.request.CardInfo.CVV, card["cvv"])
				}
			} else {
				t.Error("Expected card details in request")
			}

			// Check customer details
			if customer, ok := result["customer"].(map[string]any); ok {
				if customer["email"] != tt.request.Customer.Email {
					t.Errorf("Expected email %s, got %v", tt.request.Customer.Email, customer["email"])
				}
			}

			// Check 3D-specific fields
			if tt.is3D {
				if result["successUrl"] != tt.request.CallbackURL {
					t.Errorf("Expected successUrl %s, got %v", tt.request.CallbackURL, result["successUrl"])
				}
				if result["notificationUrl"] == nil {
					t.Error("Expected notificationUrl for 3D payment")
				}
			}

			// Check signature exists
			if result["signature"] == nil {
				t.Error("Expected signature in request")
			}
		})
	}
}

func TestPayUProvider_mapToPaymentResponse(t *testing.T) {
	p := &PayUProvider{}

	tests := []struct {
		name     string
		response PayUResponse
		expected provider.PaymentResponse
	}{
		{
			name: "Successful payment",
			response: PayUResponse{
				Status:        statusSuccess,
				PaymentID:     "pay_123",
				TransactionID: "txn_456",
				Amount:        150.75,
				Currency:      "TRY",
				Message:       "Payment successful",
			},
			expected: provider.PaymentResponse{
				Success:       true,
				PaymentID:     "pay_123",
				TransactionID: "txn_456",
				Amount:        150.75,
				Currency:      "TRY",
				Status:        provider.StatusSuccessful,
				Message:       "Payment successful",
			},
		},
		{
			name: "Failed payment",
			response: PayUResponse{
				Status:       statusFailed,
				PaymentID:    "pay_789",
				Amount:       100.00,
				Currency:     "TRY",
				ErrorCode:    "INSUFFICIENT_FUNDS",
				ErrorMessage: "Not enough funds",
			},
			expected: provider.PaymentResponse{
				Success:   false,
				PaymentID: "pay_789",
				Amount:    100.00,
				Currency:  "TRY",
				Status:    provider.StatusFailed,
				ErrorCode: "INSUFFICIENT_FUNDS",
				Message:   "Not enough funds",
			},
		},
		{
			name: "3D Secure redirect",
			response: PayUResponse{
				Status:      statusPending,
				PaymentID:   "pay_3d_123",
				Amount:      200.00,
				Currency:    "TRY",
				RedirectURL: "https://secure.payu.tr/3dsecure?token=abc123",
				Message:     "Redirecting to 3D Secure",
			},
			expected: provider.PaymentResponse{
				Success:     true,
				PaymentID:   "pay_3d_123",
				Amount:      200.00,
				Currency:    "TRY",
				Status:      provider.StatusPending,
				RedirectURL: "https://secure.payu.tr/3dsecure?token=abc123",
				Message:     "Redirecting to 3D Secure",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.mapToPaymentResponse(tt.response)

			if result.Success != tt.expected.Success {
				t.Errorf("Expected success %v, got %v", tt.expected.Success, result.Success)
			}
			if result.PaymentID != tt.expected.PaymentID {
				t.Errorf("Expected paymentID %s, got %s", tt.expected.PaymentID, result.PaymentID)
			}
			if result.Amount != tt.expected.Amount {
				t.Errorf("Expected amount %f, got %f", tt.expected.Amount, result.Amount)
			}
			if result.Status != tt.expected.Status {
				t.Errorf("Expected status %s, got %s", tt.expected.Status, result.Status)
			}
			if result.ErrorCode != tt.expected.ErrorCode {
				t.Errorf("Expected errorCode %s, got %s", tt.expected.ErrorCode, result.ErrorCode)
			}
			if result.RedirectURL != tt.expected.RedirectURL {
				t.Errorf("Expected redirectURL %s, got %s", tt.expected.RedirectURL, result.RedirectURL)
			}
		})
	}
}

func TestPayUProvider_mapPayUStatus(t *testing.T) {
	p := &PayUProvider{}

	tests := []struct {
		payuStatus     string
		expectedStatus provider.PaymentStatus
	}{
		{statusSuccess, provider.StatusSuccessful},
		{statusPending, provider.StatusPending},
		{statusFailed, provider.StatusFailed},
		{statusCancelled, provider.StatusCancelled},
		{statusRefunded, provider.StatusRefunded},
		{statusAuthorized, provider.StatusProcessing},
		{"UNKNOWN_STATUS", provider.StatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.payuStatus, func(t *testing.T) {
			result := p.mapPayUStatus(tt.payuStatus)
			if result != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, result)
			}
		})
	}
}

func TestPayUProvider_generateSignature(t *testing.T) {
	p := &PayUProvider{
		merchantID: "test-merchant",
		secretKey:  "test-secret-key",
	}

	data := map[string]any{
		"amount":  "100.00",
		"orderId": "order-123",
	}

	signature := p.generateSignature(data)

	if signature == "" {
		t.Error("Signature should not be empty")
	}

	// Test consistency - same data should produce same signature
	signature2 := p.generateSignature(data)
	if signature != signature2 {
		t.Error("Same data should produce same signature")
	}

	// Test different data produces different signature
	data2 := map[string]any{
		"amount":  "200.00",
		"orderId": "order-456",
	}
	signature3 := p.generateSignature(data2)
	if signature == signature3 {
		t.Error("Different data should produce different signatures")
	}
}

func TestPayUProvider_calculateWebhookSignature(t *testing.T) {
	p := &PayUProvider{
		secretKey: "test-secret-key",
	}

	payload := `{"paymentId":"pay_123","status":"SUCCESS"}`
	signature := p.calculateWebhookSignature(payload)

	if signature == "" {
		t.Error("Webhook signature should not be empty")
	}

	// Test consistency
	signature2 := p.calculateWebhookSignature(payload)
	if signature != signature2 {
		t.Error("Same payload should produce same signature")
	}
}

func TestPayUProvider_ValidateWebhook(t *testing.T) {
	p := &PayUProvider{
		secretKey: "test-secret-key",
	}

	payload := `{"paymentId":"pay_123","status":"SUCCESS","amount":100.50}`
	expectedSignature := p.calculateWebhookSignature(payload)

	tests := []struct {
		name        string
		data        map[string]string
		headers     map[string]string
		expectValid bool
		expectError bool
	}{
		{
			name: "Valid webhook",
			data: map[string]string{
				"payload": payload,
			},
			headers: map[string]string{
				"X-PayU-Signature": expectedSignature,
			},
			expectValid: true,
			expectError: false,
		},
		{
			name: "Invalid signature",
			data: map[string]string{
				"payload": payload,
			},
			headers: map[string]string{
				"X-PayU-Signature": "invalid_signature",
			},
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Missing signature header",
			data:        map[string]string{"payload": payload},
			headers:     map[string]string{},
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Missing payload",
			data:        map[string]string{},
			headers:     map[string]string{"X-PayU-Signature": expectedSignature},
			expectValid: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			valid, result, err := p.ValidateWebhook(ctx, tt.data, tt.headers)

			if (err != nil) != tt.expectError {
				t.Errorf("Expected error %v, got %v", tt.expectError, err != nil)
			}

			if valid != tt.expectValid {
				t.Errorf("Expected valid %v, got %v", tt.expectValid, valid)
			}

			if tt.expectValid && result == nil {
				t.Error("Expected result data for valid webhook")
			}
		})
	}
}

func TestPayUProvider_Integration_CreatePayment(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == endpointPayment {
			response := PayUResponse{
				Status:        statusSuccess,
				PaymentID:     "pay_test_123",
				TransactionID: "txn_test_456",
				Amount:        100.50,
				Currency:      "TRY",
				Message:       "Payment successful",
				Timestamp:     time.Now().Unix(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	p := &PayUProvider{
		merchantID: "test-merchant",
		secretKey:  "test-secret-key",
		baseURL:    server.URL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}

	request := provider.PaymentRequest{
		Amount:      100.50,
		Currency:    "TRY",
		ReferenceID: "order-123",
		Customer: provider.Customer{
			Email: "test@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  "5528790000000008",
			CVV:         "123",
			ExpireMonth: "12",
			ExpireYear:  "2030",
		},
	}

	ctx := context.Background()
	response, err := p.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful payment")
	}

	if response.PaymentID != "pay_test_123" {
		t.Errorf("Expected paymentID 'pay_test_123', got %s", response.PaymentID)
	}

	if response.Amount != 100.50 {
		t.Errorf("Expected amount 100.50, got %f", response.Amount)
	}
}

func TestPayUProvider_Integration_GetPaymentStatus(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/payment/pay_test_123" {
			response := PayUResponse{
				Status:        statusSuccess,
				PaymentID:     "pay_test_123",
				TransactionID: "txn_test_456",
				Amount:        100.50,
				Currency:      "TRY",
				Message:       "Payment successful",
				Timestamp:     time.Now().Unix(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	p := &PayUProvider{
		merchantID: "test-merchant",
		secretKey:  "test-secret-key",
		baseURL:    server.URL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	response, err := p.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: "pay_test_123"})

	if err != nil {
		t.Fatalf("GetPaymentStatus failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful status response")
	}

	if response.PaymentID != "pay_test_123" {
		t.Errorf("Expected paymentID 'pay_test_123', got %s", response.PaymentID)
	}
}

func TestPayUProvider_GetRequiredConfig(t *testing.T) {
	provider := NewProvider().(*PayUProvider)

	tests := []struct {
		name        string
		environment string
		expected    int
	}{
		{"sandbox environment", "sandbox", 3},
		{"production environment", "production", 3},
		{"test environment", "test", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetRequiredConfig(tt.environment)
			if len(result) != tt.expected {
				t.Errorf("GetRequiredConfig() returned %d fields, want %d", len(result), tt.expected)
			}

			// Check required fields
			expectedFields := []string{"merchantId", "secretKey", "environment"}
			for i, field := range result {
				if field.Key != expectedFields[i] {
					t.Errorf("Expected field %s, got %s", expectedFields[i], field.Key)
				}
				if !field.Required {
					t.Errorf("Field %s should be required", field.Key)
				}
				if field.Type != "string" {
					t.Errorf("Field %s should be string type", field.Key)
				}
			}
		})
	}
}

func TestPayUProvider_ValidateConfig(t *testing.T) {
	provider := NewProvider().(*PayUProvider)

	tests := []struct {
		name        string
		config      map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid sandbox config",
			config: map[string]string{
				"merchantId":  "123456",
				"secretKey":   "PAYU_SECRET_KEY_123",
				"environment": "sandbox",
			},
			expectError: false,
		},
		{
			name: "valid production config",
			config: map[string]string{
				"merchantId":  "654321",
				"secretKey":   "PAYU_PROD_SECRET_KEY_789",
				"environment": "production",
			},
			expectError: false,
		},
		{
			name: "missing merchantId",
			config: map[string]string{
				"secretKey":   "PAYU_SECRET_KEY_123",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'merchantId' is missing",
		},
		{
			name: "missing secretKey",
			config: map[string]string{
				"merchantId":  "123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'secretKey' is missing",
		},
		{
			name: "missing environment",
			config: map[string]string{
				"merchantId": "123456",
				"secretKey":  "PAYU_SECRET_KEY_123",
			},
			expectError: true,
			errorMsg:    "required field 'environment' is missing",
		},
		{
			name: "empty merchantId",
			config: map[string]string{
				"merchantId":  "",
				"secretKey":   "PAYU_SECRET_KEY_123",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'merchantId' cannot be empty",
		},
		{
			name: "empty secretKey",
			config: map[string]string{
				"merchantId":  "123456",
				"secretKey":   "",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'secretKey' cannot be empty",
		},
		{
			name: "invalid environment",
			config: map[string]string{
				"merchantId":  "123456",
				"secretKey":   "PAYU_SECRET_KEY_123",
				"environment": "invalid_env",
			},
			expectError: true,
			errorMsg:    "environment must be one of",
		},
		{
			name: "merchantId too short",
			config: map[string]string{
				"merchantId":  "12",
				"secretKey":   "PAYU_SECRET_KEY_123",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 3 characters",
		},
		{
			name: "secretKey too short",
			config: map[string]string{
				"merchantId":  "123456",
				"secretKey":   "short",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 10 characters",
		},
		{
			name: "merchantId too long",
			config: map[string]string{
				"merchantId":  "12345678901234567890123456789012345",
				"secretKey":   "PAYU_SECRET_KEY_123",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must not exceed 20 characters",
		},
		{
			name: "secretKey too long",
			config: map[string]string{
				"merchantId":  "123456",
				"secretKey":   "12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must not exceed 100 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %s", err.Error())
				}
			}
		})
	}
}
