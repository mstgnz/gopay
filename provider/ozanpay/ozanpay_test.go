package ozanpay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	if ozanPayProvider.httpClient == nil {
		t.Error("HTTP client should be initialized")
	}

	// Note: We can't directly access timeout as it's in the config
	// The timeout is set during Initialize, so we'll test it there
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
				"providerKey": "test-merchant-id",
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
				"providerKey": "test-merchant-id",
				"environment": "production",
			},
			expectError: false,
			expectProd:  true,
			expectURL:   apiProductionURL,
		},
		{
			name: "Default to sandbox",
			config: map[string]string{
				"apiKey":      "test-api-key",
				"secretKey":   "test-secret-key",
				"providerKey": "test-merchant-id",
			},
			expectError: false,
			expectProd:  false,
			expectURL:   apiSandboxURL,
		},
		{
			name: "Missing apiKey",
			config: map[string]string{
				"secretKey":   "test-secret-key",
				"providerKey": "test-merchant-id",
				"environment": "sandbox",
			},
			expectError: true,
		},
		{
			name: "Missing secretKey - should work",
			config: map[string]string{
				"apiKey":      "test-api-key",
				"providerKey": "test-provider-key",
				"environment": "sandbox",
			},
			expectError: false,
			expectProd:  false,
			expectURL:   apiSandboxURL,
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

			if provider.providerKey != tt.config["providerKey"] {
				t.Errorf("Expected providerKey %s, got %s", tt.config["providerKey"], provider.providerKey)
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
		apiKey:       "test-api-key",
		providerKey:  "test-provider-key",
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
			Address: &provider.Address{
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
				// Check basic payment info
				if result["amount"] != int64(10050) { // 100.50 * 100
					t.Errorf("Expected amount 10050, got %v", result["amount"])
				}
				if result["currency"] != "USD" {
					t.Errorf("Expected currency 'USD', got %v", result["currency"])
				}
				if result["apiKey"] != "test-api-key" {
					t.Errorf("Expected apiKey 'test-api-key', got %v", result["apiKey"])
				}

				// Check card info (now flat in main object)
				if result["number"] != "4111111111111111" {
					t.Errorf("Expected number '4111111111111111', got %v", result["number"])
				}
				if result["cvv"] != "123" {
					t.Errorf("Expected cvv '123', got %v", result["cvv"])
				}

				// Check customer info (now flat in main object)
				if result["email"] != "john@example.com" {
					t.Errorf("Expected email 'john@example.com', got %v", result["email"])
				}
				if result["billingFirstName"] != "John" {
					t.Errorf("Expected billingFirstName 'John', got %v", result["billingFirstName"])
				}
				if result["billingLastName"] != "Doe" {
					t.Errorf("Expected billingLastName 'Doe', got %v", result["billingLastName"])
				}

				// Check address info (now flat in main object)
				if result["billingCity"] != "New York" {
					t.Errorf("Expected billingCity 'New York', got %v", result["billingCity"])
				}
				if result["billingCountry"] != "USA" {
					t.Errorf("Expected billingCountry 'USA', got %v", result["billingCountry"])
				}

				// Check 3D is disabled
				if result["is3d"] != false {
					t.Errorf("Expected is3d false, got %v", result["is3d"])
				}

				// Check basket items are present
				basketItems, ok := result["basketItems"].([]map[string]any)
				if !ok {
					t.Error("basketItems should be an array")
					return
				}
				if len(basketItems) != 1 {
					t.Errorf("Expected 1 basket item, got %d", len(basketItems))
				}
				if basketItems[0]["name"] != "Test Item" {
					t.Errorf("Expected item name 'Test Item', got %v", basketItems[0]["name"])
				}
			},
		},
		{
			name:    "3D payment request",
			request: request,
			force3D: true,
			validate: func(t *testing.T, result map[string]any) {
				// Check 3D is enabled
				if result["is3d"] != true {
					t.Errorf("Expected is3d true, got %v", result["is3d"])
				}

				// Check return URL is set
				expectedURL := "https://test.gopay.com/v1/callback/ozanpay?originalCallbackUrl=https://example.com/callback"
				if result["returnUrl"] != expectedURL {
					t.Errorf("Expected returnUrl '%s', got %v", expectedURL, result["returnUrl"])
				}

				// Check browser info is present for 3D payments
				browserInfo, ok := result["browserInfo"].(map[string]any)
				if !ok {
					t.Error("browserInfo should be a map for 3D payments")
					return
				}
				if browserInfo["language"] != "en-US" {
					t.Errorf("Expected browserInfo language 'en-US', got %v", browserInfo["language"])
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
				"status":  statusFailed,
				"code":    "14", // Invalid card number error code
				"message": "Invalid card number",
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
		apiKey:      "test-key",
		secretKey:   "test-secret",
		providerKey: "test-merchant",
		baseURL:     server.URL,
		httpClient: provider.NewProviderHTTPClient(&provider.HTTPClientConfig{
			BaseURL: server.URL,
			Timeout: 5 * time.Second,
		}),
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
		providerKey:  "test-merchant",
		baseURL:      server.URL,
		gopayBaseURL: "https://test.gopay.com",
		httpClient: provider.NewProviderHTTPClient(&provider.HTTPClientConfig{
			BaseURL: server.URL,
			Timeout: 5 * time.Second,
		}),
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
		apiKey:      "test-key",
		secretKey:   "test-secret",
		providerKey: "test-merchant",
		baseURL:     server.URL,
		httpClient: provider.NewProviderHTTPClient(&provider.HTTPClientConfig{
			BaseURL: server.URL,
			Timeout: 5 * time.Second,
		}),
	}

	ctx := context.Background()

	// Test with valid payment ID
	response, err := ozanPayProvider.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: "payment123"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if !response.Success {
		t.Error("Expected successful response")
	}

	// Test with empty payment ID
	_, err = ozanPayProvider.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: ""})
	if err == nil {
		t.Error("Expected error for empty payment ID")
	}
}

func TestOzanPayProvider_RefundPayment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"transactionId": "refund123", // OzanPay uses transactionId for refund ID
			"status":        statusApproved,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ozanPayProvider := &OzanPayProvider{
		apiKey:      "test-key",
		secretKey:   "test-secret",
		providerKey: "test-merchant",
		baseURL:     server.URL,
		httpClient: provider.NewProviderHTTPClient(&provider.HTTPClientConfig{
			BaseURL: server.URL,
			Timeout: 5 * time.Second,
		}),
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

func TestOzanPayProvider_ValidateWebhook(t *testing.T) {
	provider := NewProvider().(*OzanPayProvider)
	provider.secretKey = "test-secret"

	tests := []struct {
		name      string
		data      map[string]string
		headers   map[string]string
		expectErr bool
		expectMsg string
	}{
		{
			name: "Valid webhook with correct checksum",
			data: map[string]string{
				"transactionId": "123456",
				"referenceNo":   "gopay-123456",
				"amount":        "1000",
				"currency":      "TRY",
				"status":        "APPROVED",
				"message":       "Payment successful",
				"code":          "00",
				"checksum":      "", // Will be calculated below
			},
			headers:   map[string]string{},
			expectErr: false,
		},
		{
			name: "Invalid checksum",
			data: map[string]string{
				"transactionId": "123456",
				"referenceNo":   "gopay-123456",
				"amount":        "1000",
				"currency":      "TRY",
				"status":        "APPROVED",
				"message":       "Payment successful",
				"code":          "00",
				"checksum":      "wrong_checksum",
			},
			headers:   map[string]string{},
			expectErr: true,
			expectMsg: "invalid webhook checksum",
		},
		{
			name: "Missing checksum",
			data: map[string]string{
				"transactionId": "123456",
				"referenceNo":   "gopay-123456",
				"amount":        "1000",
				"currency":      "TRY",
				"status":        "APPROVED",
				"message":       "Payment successful",
				"code":          "00",
			},
			headers:   map[string]string{},
			expectErr: true,
			expectMsg: "missing checksum",
		},
		{
			name: "Missing required field referenceNo",
			data: map[string]string{
				"transactionId": "123456",
				"amount":        "1000",
				"currency":      "TRY",
				"status":        "APPROVED",
				"message":       "Payment successful",
				"code":          "00",
				"checksum":      "test_checksum",
			},
			headers:   map[string]string{},
			expectErr: true,
			expectMsg: "missing referenceNo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate correct checksum for valid test case
			if tt.name == "Valid webhook with correct checksum" {
				checksumString := tt.data["referenceNo"] + tt.data["amount"] + tt.data["currency"] +
					tt.data["status"] + tt.data["message"] + tt.data["code"] + provider.secretKey
				hash := sha256.Sum256([]byte(checksumString))
				tt.data["checksum"] = hex.EncodeToString(hash[:])
			}

			isValid, _, err := provider.ValidateWebhook(context.Background(), tt.data, tt.headers)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if isValid {
					t.Errorf("Expected invalid webhook but got valid")
				}
				if tt.expectMsg != "" && !strings.Contains(err.Error(), tt.expectMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %s", err.Error())
				}
				if !isValid {
					t.Errorf("Expected valid webhook but got invalid")
				}
			}
		})
	}
}

func TestOzanPayProvider_GetRequiredConfig(t *testing.T) {
	provider := NewProvider().(*OzanPayProvider)

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

func TestOzanPayProvider_ValidateConfig(t *testing.T) {
	provider := NewProvider().(*OzanPayProvider)

	tests := []struct {
		name        string
		config      map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid sandbox config",
			config: map[string]string{
				"apiKey":      "OZANPAY_API_KEY_123456789",
				"secretKey":   "OZANPAY_SECRET_KEY_123456789",
				"merchantId":  "MERCHANT123456",
				"environment": "sandbox",
			},
			expectError: false,
		},
		{
			name: "valid production config",
			config: map[string]string{
				"apiKey":      "OZANPAY_API_KEY_PROD123456789",
				"secretKey":   "OZANPAY_SECRET_KEY_PROD123456789",
				"merchantId":  "PRODMERCHANT123456",
				"environment": "production",
			},
			expectError: false,
		},
		{
			name: "missing apiKey",
			config: map[string]string{
				"secretKey":   "OZANPAY_SECRET_KEY_123456789",
				"merchantId":  "MERCHANT123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'apiKey' is missing",
		},
		{
			name: "missing secretKey",
			config: map[string]string{
				"apiKey":      "OZANPAY_API_KEY_123456789",
				"merchantId":  "MERCHANT123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'secretKey' is missing",
		},
		{
			name: "missing merchantId",
			config: map[string]string{
				"apiKey":      "OZANPAY_API_KEY_123456789",
				"secretKey":   "OZANPAY_SECRET_KEY_123456789",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'merchantId' is missing",
		},
		{
			name: "empty apiKey",
			config: map[string]string{
				"apiKey":      "",
				"secretKey":   "OZANPAY_SECRET_KEY_123456789",
				"merchantId":  "MERCHANT123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'apiKey' cannot be empty",
		},
		{
			name: "invalid environment",
			config: map[string]string{
				"apiKey":      "OZANPAY_API_KEY_123456789",
				"secretKey":   "OZANPAY_SECRET_KEY_123456789",
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
				"secretKey":   "OZANPAY_SECRET_KEY_123456789",
				"merchantId":  "MERCHANT123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 10 characters",
		},
		{
			name: "merchantId too short",
			config: map[string]string{
				"apiKey":      "OZANPAY_API_KEY_123456789",
				"secretKey":   "OZANPAY_SECRET_KEY_123456789",
				"merchantId":  "ABC",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 5 characters",
		},
		{
			name: "secretKey too short",
			config: map[string]string{
				"apiKey":      "OZANPAY_API_KEY_123456789",
				"secretKey":   "short",
				"merchantId":  "MERCHANT123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 10 characters",
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
