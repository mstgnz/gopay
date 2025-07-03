package nkolay

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
				"secretKey":  "test-secret-key",
				"merchantId": "test-merchant-id",
			},
			expectError: true,
		},
		{
			name: "Missing secretKey",
			config: map[string]string{
				"apiKey":     "test-api-key",
				"merchantId": "test-merchant-id",
			},
			expectError: true,
		},
		{
			name: "Missing merchantId",
			config: map[string]string{
				"apiKey":    "test-api-key",
				"secretKey": "test-secret-key",
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
			CardNumber:  "5528790000000008",
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
			name: "3D request missing callback URL",
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
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestNkolayProvider_CreatePayment(t *testing.T) {
	// Mock server for testing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Check auth headers
		if r.Header.Get("X-Nkolay-ApiKey") == "" {
			t.Error("Expected X-Nkolay-ApiKey header")
		}

		// Mock successful response
		response := NkolayResponse{
			Success:       true,
			Status:        statusSuccess,
			PaymentID:     "nkolay_payment_123",
			TransactionID: "txn_456789",
			Amount:        100.50,
			Currency:      "TRY",
			Message:       "Payment successful",
			Timestamp:     time.Now().Unix(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with mock server
	nkolayProvider := &NkolayProvider{
		apiKey:     "test-api-key",
		secretKey:  "test-secret-key",
		merchantID: "test-merchant",
		baseURL:    server.URL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}

	request := provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  "5528790000000008",
			CVV:         "123",
			ExpireMonth: "12",
			ExpireYear:  "2030",
		},
	}

	ctx := context.Background()
	response, err := nkolayProvider.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful response")
	}

	if response.Status != provider.StatusSuccessful {
		t.Errorf("Expected status successful, got %v", response.Status)
	}

	if response.PaymentID != "nkolay_payment_123" {
		t.Errorf("Expected payment ID nkolay_payment_123, got %s", response.PaymentID)
	}

	if response.Amount != 100.50 {
		t.Errorf("Expected amount 100.50, got %v", response.Amount)
	}
}

func TestNkolayProvider_GetPaymentStatus(t *testing.T) {
	// Mock server for testing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Mock successful response
		response := NkolayResponse{
			Success:       true,
			Status:        statusSuccess,
			PaymentID:     "test-payment-id",
			TransactionID: "txn_123",
			Amount:        100.50,
			Currency:      "TRY",
			Message:       "Payment found",
			Timestamp:     time.Now().Unix(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	nkolayProvider := &NkolayProvider{
		apiKey:     "test-api-key",
		secretKey:  "test-secret-key",
		merchantID: "test-merchant",
		baseURL:    server.URL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}

	ctx := context.Background()
	response, err := nkolayProvider.GetPaymentStatus(ctx, "test-payment-id")

	if err != nil {
		t.Fatalf("GetPaymentStatus failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful response")
	}

	if response.PaymentID != "test-payment-id" {
		t.Errorf("Expected payment ID test-payment-id, got %s", response.PaymentID)
	}
}

func TestGenerateSignature(t *testing.T) {
	nkolayProvider := &NkolayProvider{
		secretKey: "test-secret-key",
	}

	tests := []struct {
		name string
		data string
	}{
		{
			name: "Simple string",
			data: "test-data",
		},
		{
			name: "JSON data",
			data: `{"amount":100.50,"currency":"TRY"}`,
		},
		{
			name: "Empty string",
			data: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature1 := nkolayProvider.generateSignature(tt.data)
			signature2 := nkolayProvider.generateSignature(tt.data)

			// Signatures should be consistent
			if signature1 != signature2 {
				t.Error("Signatures should be consistent for same data")
			}
		})
	}
}

func TestNkolayProvider_ValidateWebhook(t *testing.T) {
	nkolayProvider := &NkolayProvider{
		secretKey: "test-secret-key",
	}

	validPayload := `{"paymentId":"test-123","status":"SUCCESS","amount":100.50}`
	validSignature := nkolayProvider.generateSignature(validPayload)
	timestamp := "1642248600"

	tests := []struct {
		name      string
		data      map[string]string
		headers   map[string]string
		expectOK  bool
		expectErr bool
	}{
		{
			name: "Valid webhook",
			data: map[string]string{
				"payload": validPayload,
			},
			headers: map[string]string{
				"X-Nkolay-Signature": validSignature,
				"X-Nkolay-Timestamp": timestamp,
			},
			expectOK:  true,
			expectErr: false,
		},
		{
			name: "Missing signature",
			data: map[string]string{
				"payload": validPayload,
			},
			headers: map[string]string{
				"X-Nkolay-Timestamp": timestamp,
			},
			expectOK:  false,
			expectErr: true,
		},
		{
			name: "Invalid signature",
			data: map[string]string{
				"payload": validPayload,
			},
			headers: map[string]string{
				"X-Nkolay-Signature": "invalid-signature",
				"X-Nkolay-Timestamp": timestamp,
			},
			expectOK:  false,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			valid, _, err := nkolayProvider.ValidateWebhook(ctx, tt.data, tt.headers)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}

			if valid != tt.expectOK {
				t.Errorf("Expected valid=%v, got %v", tt.expectOK, valid)
			}
		})
	}
}
