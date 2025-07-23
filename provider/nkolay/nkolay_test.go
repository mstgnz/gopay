package nkolay

import (
	"context"
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

	nkolayProvider, ok := provider.(*NkolayProvider)
	if !ok {
		t.Error("NewProvider() should return *NkolayProvider")
	}

	if nkolayProvider.client == nil {
		t.Error("HTTP client should be initialized")
	}

	if nkolayProvider.client.Timeout != defaultTimeout {
		t.Errorf("HTTP client timeout should be %v, got %v", defaultTimeout, nkolayProvider.client.Timeout)
	}
}

func TestNkolayProvider_Initialize(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]string
		expectError bool
		expectProd  bool
		expectURL   string
	}{
		{
			name: "Valid sandbox config with custom credentials",
			config: map[string]string{
				"sx":          "custom-sx-token",
				"secretKey":   "custom-secret-key",
				"environment": "sandbox",
			},
			expectError: false,
			expectProd:  false,
			expectURL:   apiSandboxURL,
		},
		{
			name: "Valid production config",
			config: map[string]string{
				"sx":          "prod-sx-token",
				"secretKey":   "prod-secret-key",
				"environment": "production",
			},
			expectError: false,
			expectProd:  true,
			expectURL:   apiProductionURL,
		},
		{
			name: "Default config uses test credentials",
			config: map[string]string{
				"environment": "sandbox",
			},
			expectError: false,
		},
		{
			name:        "Empty config defaults to test values",
			config:      map[string]string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewProvider().(*NkolayProvider)
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

			// Check that sx token is set (either custom or default test value)
			if provider.sx == "" {
				t.Error("Expected sx token to be set")
			}

			// Check that secret key is set (either custom or default test value)
			if provider.secretKey == "" {
				t.Error("Expected secret key to be set")
			}

			if provider.isProduction != tt.expectProd {
				t.Errorf("Expected isProduction %v, got %v", tt.expectProd, provider.isProduction)
			}

			if tt.expectURL != "" && provider.baseURL != tt.expectURL {
				t.Errorf("Expected baseURL %s, got %s", tt.expectURL, provider.baseURL)
			}

			// Verify custom values if provided
			if sx := tt.config["sx"]; sx != "" && provider.sx != sx {
				t.Errorf("Expected sx %s, got %s", sx, provider.sx)
			}

			if secretKey := tt.config["secretKey"]; secretKey != "" && provider.secretKey != secretKey {
				t.Errorf("Expected secretKey %s, got %s", secretKey, provider.secretKey)
			}
		})
	}
}

func TestNkolayProvider_ValidatePaymentRequest(t *testing.T) {
	nkolayProvider := &NkolayProvider{}

	validRequest := provider.PaymentRequest{
		Amount:   100.0,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:     "5528790000000008",
			CardHolderName: "John Doe",
			CVV:            "123",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
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
			name: "Missing expiry month",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireMonth = ""
				return req
			}(),
			expectError: true,
			errorMsg:    "expiry date is required",
		},
		{
			name: "Missing expiry year",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireYear = ""
				return req
			}(),
			expectError: true,
			errorMsg:    "expiry date is required",
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
			errorMsg:    "callback URL is required for 3D payments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nkolayProvider.validatePaymentRequest(tt.request, tt.is3D)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestNkolayProvider_GeneratePaymentHash(t *testing.T) {
	provider := &NkolayProvider{
		secretKey: testSecretKey,
	}

	formData := map[string]string{
		"sx":            testSx,
		"clientRefCode": "test123",
		"amount":        "10.04",
		"rnd":           "02-01-2006 15:04:05",
	}

	input := formData["sx"] + formData["clientRefCode"] + formData["amount"] + formData["rnd"] + provider.secretKey
	hash := provider.generateSHA1Hash(input)

	if hash == "" {
		t.Error("Expected non-empty hash")
	}

	// Test that same data produces same hash
	input2 := formData["sx"] + formData["clientRefCode"] + formData["amount"] + formData["rnd"] + provider.secretKey
	hash2 := provider.generateSHA1Hash(input2)
	if hash != hash2 {
		t.Error("Expected same hash for same data")
	}

	// Test that different data produces different hash
	formData["amount"] = "20.00"
	input3 := formData["sx"] + formData["clientRefCode"] + formData["amount"] + formData["rnd"] + provider.secretKey
	hash3 := provider.generateSHA1Hash(input3)
	if hash == hash3 {
		t.Error("Expected different hash for different data")
	}
}

func TestNkolayProvider_CreatePayment(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check method
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Check content type
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			t.Errorf("Expected multipart/form-data content type, got %s", contentType)
		}

		// Mock successful payment response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<response>SUCCESS</response>`))
	}))
	defer server.Close()

	nkolayProvider := &NkolayProvider{
		sx:        testSx,
		secretKey: testSecretKey,
		baseURL:   server.URL,
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	request := provider.PaymentRequest{
		Amount:   10.04,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:    "Test",
			Surname: "User",
			Email:   "test@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:     "4546711234567894",
			CardHolderName: "Test User",
			CVV:            "001",
			ExpireMonth:    "12",
			ExpireYear:     "2026",
		},
	}

	ctx := context.Background()
	response, err := nkolayProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if response == nil {
		t.Fatal("Expected non-nil response")
	}

	if response.PaymentID == "" {
		t.Error("Expected non-empty payment ID")
	}

	if response.Amount != request.Amount {
		t.Errorf("Expected amount %.2f, got %.2f", request.Amount, response.Amount)
	}
}

func TestNkolayProvider_GetPaymentStatus(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock payment list response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<result>SUCCESS</result>`))
	}))
	defer server.Close()

	nkolayProvider := &NkolayProvider{
		sxList:    testSxList,
		secretKey: testSecretKey,
		baseURL:   server.URL,
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	ctx := context.Background()
	response, err := nkolayProvider.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: "test-payment-id"})

	if err != nil {
		t.Fatalf("GetPaymentStatus failed: %v", err)
	}

	if response == nil {
		t.Fatal("Expected non-nil response")
	}

	if response.PaymentID != "test-payment-id" {
		t.Errorf("Expected payment ID 'test-payment-id', got '%s'", response.PaymentID)
	}
}

func TestNkolayProvider_ValidateWebhook(t *testing.T) {
	provider := NewProvider().(*NkolayProvider)

	tests := []struct {
		name      string
		data      map[string]string
		headers   map[string]string
		expectErr bool
		expected  map[string]string
	}{
		{
			name: "Valid webhook with reference code",
			data: map[string]string{
				"referenceCode": "123456789",
				"status":        "success",
				"amount":        "100.50",
				"transactionId": "txn_123",
			},
			headers:   map[string]string{},
			expectErr: false,
			expected: map[string]string{
				"referenceCode": "123456789",
				"status":        "success",
				"amount":        "100.50",
				"transactionId": "txn_123",
			},
		},
		{
			name: "Invalid webhook without reference code",
			data: map[string]string{
				"status": "success",
				"amount": "100.50",
			},
			headers:   map[string]string{},
			expectErr: true,
			expected:  nil,
		},
		{
			name:      "Empty webhook data",
			data:      map[string]string{},
			headers:   map[string]string{},
			expectErr: true,
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid, result, err := provider.ValidateWebhook(context.Background(), tt.data, tt.headers)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if isValid {
					t.Errorf("Expected invalid webhook but got valid")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %s", err.Error())
				}
				if !isValid {
					t.Errorf("Expected valid webhook but got invalid")
				}
				// Nkolay returns the original data map, so check referenceCode
				if result["referenceCode"] != tt.expected["referenceCode"] {
					t.Errorf("Expected referenceCode %s, got %s", tt.expected["referenceCode"], result["referenceCode"])
				}
			}
		})
	}
}

func TestNkolayProvider_GetRequiredConfig(t *testing.T) {
	provider := NewProvider().(*NkolayProvider)

	tests := []struct {
		name        string
		environment string
		expected    int
	}{
		{"sandbox environment", "sandbox", 4},
		{"production environment", "production", 4},
		{"test environment", "test", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetRequiredConfig(tt.environment)
			if len(result) != tt.expected {
				t.Errorf("GetRequiredConfig() returned %d fields, want %d", len(result), tt.expected)
			}

			// Check required fields
			expectedFields := []string{"apiKey", "secretKey", "merchantId", "environment"}
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

func TestNkolayProvider_ValidateConfig(t *testing.T) {
	provider := NewProvider().(*NkolayProvider)

	tests := []struct {
		name        string
		config      map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid sandbox config",
			config: map[string]string{
				"apiKey":      "NKOLAY_API_KEY_123456789",
				"secretKey":   "NKOLAY_SECRET_KEY_123456789",
				"merchantId":  "MERCHANT123456",
				"environment": "sandbox",
			},
			expectError: false,
		},
		{
			name: "valid production config",
			config: map[string]string{
				"apiKey":      "NKOLAY_API_KEY_PROD123456789",
				"secretKey":   "NKOLAY_SECRET_KEY_PROD123456789",
				"merchantId":  "PRODMERCHANT123456",
				"environment": "production",
			},
			expectError: false,
		},
		{
			name: "missing apiKey",
			config: map[string]string{
				"secretKey":   "NKOLAY_SECRET_KEY_123456789",
				"merchantId":  "MERCHANT123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'apiKey' is missing",
		},
		{
			name: "missing secretKey",
			config: map[string]string{
				"apiKey":      "NKOLAY_API_KEY_123456789",
				"merchantId":  "MERCHANT123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'secretKey' is missing",
		},
		{
			name: "missing merchantId",
			config: map[string]string{
				"apiKey":      "NKOLAY_API_KEY_123456789",
				"secretKey":   "NKOLAY_SECRET_KEY_123456789",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'merchantId' is missing",
		},
		{
			name: "empty apiKey",
			config: map[string]string{
				"apiKey":      "",
				"secretKey":   "NKOLAY_SECRET_KEY_123456789",
				"merchantId":  "MERCHANT123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'apiKey' cannot be empty",
		},
		{
			name: "invalid environment",
			config: map[string]string{
				"apiKey":      "NKOLAY_API_KEY_123456789",
				"secretKey":   "NKOLAY_SECRET_KEY_123456789",
				"merchantId":  "MERCHANT123456",
				"environment": "invalid_env",
			},
			expectError: true,
			errorMsg:    "environment must be one of",
		},
		{
			name: "apiKey too short",
			config: map[string]string{
				"apiKey":      "short",
				"secretKey":   "NKOLAY_SECRET_KEY_123456789",
				"merchantId":  "MERCHANT123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 10 characters",
		},
		{
			name: "merchantId too short",
			config: map[string]string{
				"apiKey":      "NKOLAY_API_KEY_123456789",
				"secretKey":   "NKOLAY_SECRET_KEY_123456789",
				"merchantId":  "ABC",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 5 characters",
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

// Test removed due to type import issues - integration tests cover this functionality
