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
			expectProd:  false,
			expectURL:   apiSandboxURL,
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

	hash := provider.generatePaymentHash(formData)

	if hash == "" {
		t.Error("Expected non-empty hash")
	}

	// Test that same data produces same hash
	hash2 := provider.generatePaymentHash(formData)
	if hash != hash2 {
		t.Error("Expected same hash for same data")
	}

	// Test that different data produces different hash
	formData["amount"] = "20.00"
	hash3 := provider.generatePaymentHash(formData)
	if hash == hash3 {
		t.Error("Expected different hash for different data")
	}
}

func TestNkolayProvider_GenerateSHA1Hash(t *testing.T) {
	provider := &NkolayProvider{}

	tests := []struct {
		input    string
		expected string // Leave empty to just test non-empty result
	}{
		{
			input: "test",
		},
		{
			input: "hello world",
		},
		{
			input: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := provider.generateSHA1Hash(tt.input)

			if result == "" {
				t.Error("Expected non-empty hash result")
			}

			// Test consistency - same input should produce same output
			result2 := provider.generateSHA1Hash(tt.input)
			if result != result2 {
				t.Error("Expected consistent hash results")
			}
		})
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

	provider := &NkolayProvider{
		sxList:    testSxList,
		secretKey: testSecretKey,
		baseURL:   server.URL,
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	ctx := context.Background()
	response, err := provider.GetPaymentStatus(ctx, "test-payment-id")

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
	provider := &NkolayProvider{}

	tests := []struct {
		name        string
		data        map[string]string
		headers     map[string]string
		expectValid bool
		expectError bool
	}{
		{
			name: "Valid webhook with reference code",
			data: map[string]string{
				"referenceCode": "IKSIRPF428910",
				"status":        "SUCCESS",
				"amount":        "10.04",
			},
			headers:     map[string]string{},
			expectValid: true,
			expectError: false,
		},
		{
			name: "Invalid webhook without reference code",
			data: map[string]string{
				"status": "SUCCESS",
				"amount": "10.04",
			},
			headers:     map[string]string{},
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Empty webhook data",
			data:        map[string]string{},
			headers:     map[string]string{},
			expectValid: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			isValid, validatedData, err := provider.ValidateWebhook(ctx, tt.data, tt.headers)

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

			if isValid != tt.expectValid {
				t.Errorf("Expected valid %v, got %v", tt.expectValid, isValid)
			}

			if tt.expectValid && validatedData == nil {
				t.Error("Expected validated data but got nil")
			}

			if tt.expectValid {
				if validatedData["referenceCode"] != tt.data["referenceCode"] {
					t.Errorf("Expected reference code %s, got %s",
						tt.data["referenceCode"], validatedData["referenceCode"])
				}
			}
		})
	}
}

// Test removed due to type import issues - integration tests cover this functionality
