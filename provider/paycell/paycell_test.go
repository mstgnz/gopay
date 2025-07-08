package paycell

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

func TestNewProvider(t *testing.T) {
	p := NewProvider()
	if p == nil {
		t.Fatal("NewProvider should return a non-nil provider")
	}

	paycellProvider, ok := p.(*PaycellProvider)
	if !ok {
		t.Fatal("NewProvider should return a PaycellProvider instance")
	}

	if paycellProvider.client == nil {
		t.Error("PaycellProvider should have a non-nil HTTP client")
	}

	if paycellProvider.client.Timeout != defaultTimeout {
		t.Errorf("Expected timeout %v, got %v", defaultTimeout, paycellProvider.client.Timeout)
	}
}

func TestPaycellProvider_Initialize(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			config: map[string]string{
				"username":     "test_user",
				"password":     "test_pass",
				"merchantId":   "test_merchant",
				"terminalId":   "test_terminal",
				"environment":  "sandbox",
				"gopayBaseURL": "https://test.gopay.com",
			},
			expectError: false,
		},
		{
			name: "production environment",
			config: map[string]string{
				"username":    "test_user",
				"password":    "test_pass",
				"merchantId":  "test_merchant",
				"terminalId":  "test_terminal",
				"environment": "production",
			},
			expectError: false,
		},
		{
			name: "missing username",
			config: map[string]string{
				"password":   "test_pass",
				"merchantId": "test_merchant",
				"terminalId": "test_terminal",
			},
			expectError: true,
			errorMsg:    "username, password, merchantId and terminalId are required",
		},
		{
			name: "missing password",
			config: map[string]string{
				"username":   "test_user",
				"merchantId": "test_merchant",
				"terminalId": "test_terminal",
			},
			expectError: true,
			errorMsg:    "username, password, merchantId and terminalId are required",
		},
		{
			name: "missing merchantId",
			config: map[string]string{
				"username":   "test_user",
				"password":   "test_pass",
				"terminalId": "test_terminal",
			},
			expectError: true,
			errorMsg:    "username, password, merchantId and terminalId are required",
		},
		{
			name: "missing terminalId",
			config: map[string]string{
				"username":   "test_user",
				"password":   "test_pass",
				"merchantId": "test_merchant",
			},
			expectError: true,
			errorMsg:    "username, password, merchantId and terminalId are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider().(*PaycellProvider)
			err := p.Initialize(tt.config)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				// Verify configuration is set correctly
				if p.username != tt.config["username"] {
					t.Errorf("Expected username '%s', got '%s'", tt.config["username"], p.username)
				}
				if p.password != tt.config["password"] {
					t.Errorf("Expected password '%s', got '%s'", tt.config["password"], p.password)
				}
				if p.merchantID != tt.config["merchantId"] {
					t.Errorf("Expected merchantId '%s', got '%s'", tt.config["merchantId"], p.merchantID)
				}
				if p.terminalID != tt.config["terminalId"] {
					t.Errorf("Expected terminalId '%s', got '%s'", tt.config["terminalId"], p.terminalID)
				}

				// Verify environment setting
				if tt.config["environment"] == "production" {
					if !p.isProduction || p.baseURL != apiProductionURL {
						t.Error("Production environment not set correctly")
					}
					if p.paymentManagementURL != paymentManagementProductionURL {
						t.Error("Production payment management URL not set correctly")
					}
				} else {
					if p.isProduction || p.baseURL != apiSandboxURL {
						t.Error("Sandbox environment not set correctly")
					}
					if p.paymentManagementURL != paymentManagementSandboxURL {
						t.Error("Sandbox payment management URL not set correctly")
					}
				}

				// Verify GoPay base URL
				expectedBaseURL := tt.config["gopayBaseURL"]
				if expectedBaseURL == "" {
					expectedBaseURL = "http://localhost:9999"
				}
				if p.gopayBaseURL != expectedBaseURL {
					t.Errorf("Expected gopayBaseURL '%s', got '%s'", expectedBaseURL, p.gopayBaseURL)
				}
			}
		})
	}
}

func TestPaycellProvider_ValidatePaymentRequest(t *testing.T) {
	p := &PaycellProvider{}

	validRequest := provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:        "John",
			Surname:     "Doe",
			Email:       "john@example.com",
			PhoneNumber: "5551234567",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  "5528790000000008",
			ExpireMonth: "12",
			ExpireYear:  "2030",
			CVV:         "123",
		},
	}

	tests := []struct {
		name        string
		request     provider.PaymentRequest
		is3D        bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid request",
			request:     validRequest,
			is3D:        false,
			expectError: false,
		},
		{
			name: "valid 3D request",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CallbackURL = "https://example.com/callback"
				return req
			}(),
			is3D:        true,
			expectError: false,
		},
		{
			name: "invalid amount",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Amount = 0
				return req
			}(),
			is3D:        false,
			expectError: true,
			errorMsg:    "amount must be greater than 0",
		},
		{
			name: "missing currency",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Currency = ""
				return req
			}(),
			is3D:        false,
			expectError: true,
			errorMsg:    "currency is required",
		},
		{
			name: "missing customer phone number",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Customer.PhoneNumber = ""
				return req
			}(),
			is3D:        false,
			expectError: true,
			errorMsg:    "customer phone number is required",
		},
		{
			name: "invalid phone number format",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Customer.PhoneNumber = "123456789"
				return req
			}(),
			is3D:        false,
			expectError: true,
			errorMsg:    "phone number must be 10 digits",
		},
		{
			name: "missing card number",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.CardNumber = ""
				return req
			}(),
			is3D:        false,
			expectError: true,
			errorMsg:    "card number is required",
		},
		{
			name: "missing CVV",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.CVV = ""
				return req
			}(),
			is3D:        false,
			expectError: true,
			errorMsg:    "card CVV is required",
		},
		{
			name: "missing expire month",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireMonth = ""
				return req
			}(),
			is3D:        false,
			expectError: true,
			errorMsg:    "card expiry date is required",
		},
		{
			name: "missing expire year",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireYear = ""
				return req
			}(),
			is3D:        false,
			expectError: true,
			errorMsg:    "card expiry date is required",
		},
		{
			name: "3D payment missing callback URL",
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
			err := p.validatePaymentRequest(tt.request, tt.is3D)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestPaycellProvider_MapToPaycellRequest(t *testing.T) {
	p := &PaycellProvider{
		merchantID:   "test_merchant",
		terminalID:   "test_terminal",
		gopayBaseURL: "https://test.gopay.com",
	}

	request := provider.PaymentRequest{
		Amount:         100.50,
		Currency:       "TRY",
		Description:    "Test payment",
		ConversationID: "conv123",
		Customer: provider.Customer{
			Name:        "John",
			Surname:     "Doe",
			Email:       "john@example.com",
			PhoneNumber: "+90555123456",
			Address: &provider.Address{
				Country: "Turkey",
				City:    "Istanbul",
				Address: "Test address",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     "5528790000000008",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CVV:            "123",
		},
		Items: []provider.Item{
			{
				Name:     "Test Item",
				Price:    50.25,
				Quantity: 2,
			},
		},
	}

	tests := []struct {
		name        string
		request     provider.PaymentRequest
		is3D        bool
		callbackURL string
	}{
		{
			name:    "regular payment",
			request: request,
			is3D:    false,
		},
		{
			name: "3D payment with callback URL",
			request: func() provider.PaymentRequest {
				req := request
				req.CallbackURL = "https://example.com/callback"
				return req
			}(),
			is3D: true,
		},
		{
			name:    "3D payment without callback URL",
			request: request,
			is3D:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.mapToPaycellRequest(tt.request, tt.is3D)

			// Verify basic fields from real Paycell API format
			if result["merchantCode"] != p.merchantID {
				t.Errorf("Expected merchantCode '%s', got '%v'", p.merchantID, result["merchantCode"])
			}

			// Verify referenceNumber is present
			if _, exists := result["referenceNumber"]; !exists {
				t.Error("Expected referenceNumber to be present")
			}

			// Amount should be in cents (multiply by 100)
			expectedAmount := fmt.Sprintf("%.0f", tt.request.Amount*100)
			if result["amount"] != expectedAmount {
				t.Errorf("Expected amount '%s', got '%v'", expectedAmount, result["amount"])
			}
			if result["currency"] != tt.request.Currency {
				t.Errorf("Expected currency '%s', got '%s'", tt.request.Currency, result["currency"])
			}

			// Verify MSISDN (phone number processing)
			if _, exists := result["msisdn"]; !exists {
				t.Error("Expected msisdn to be present")
			}

			// Verify paymentType is set
			if result["paymentType"] != "SALE" {
				t.Errorf("Expected paymentType 'SALE', got '%v'", result["paymentType"])
			}

			// Verify requestHeader structure
			if requestHeader, exists := result["requestHeader"]; exists {
				if header, ok := requestHeader.(map[string]any); ok {
					if header["applicationName"] != p.username {
						t.Errorf("Expected applicationName '%s', got '%v'", p.username, header["applicationName"])
					}
					if header["applicationPwd"] != p.password {
						t.Errorf("Expected applicationPwd '%s', got '%v'", p.password, header["applicationPwd"])
					}
					if _, exists := header["transactionId"]; !exists {
						t.Error("Expected transactionId in requestHeader")
					}
					if _, exists := header["transactionDateTime"]; !exists {
						t.Error("Expected transactionDateTime in requestHeader")
					}
				} else {
					t.Error("Expected requestHeader to be a map[string]any")
				}
			} else {
				t.Error("Expected requestHeader to be present")
			}

			// The new Paycell API format doesn't use 3D flags in the request mapping
			// 3D secure handling is done at the API level, not in the request structure

			// Note: The new format doesn't include customer details, card details, or URLs
			// These are handled separately through the card token system
		})
	}
}

func TestPaycellProvider_MapToPaymentResponse(t *testing.T) {
	p := &PaycellProvider{}

	tests := []struct {
		name            string
		paycellResp     PaycellResponse
		expectedStatus  provider.PaymentStatus
		expectedSuccess bool
	}{
		{
			name: "successful payment",
			paycellResp: PaycellResponse{
				Success:       true,
				Status:        statusSuccess,
				PaymentID:     "pay123",
				TransactionID: "txn123",
				Amount:        "100.50",
				Currency:      "TRY",
				Message:       "Payment successful",
			},
			expectedStatus:  provider.StatusSuccessful,
			expectedSuccess: true,
		},
		{
			name: "pending payment",
			paycellResp: PaycellResponse{
				Success:       false,
				Status:        statusPending,
				PaymentID:     "pay123",
				TransactionID: "txn123",
				Amount:        "100.50",
				Currency:      "TRY",
				Message:       "Payment pending",
			},
			expectedStatus:  provider.StatusPending,
			expectedSuccess: false,
		},
		{
			name: "failed payment",
			paycellResp: PaycellResponse{
				Success:       false,
				Status:        statusFailed,
				PaymentID:     "pay123",
				TransactionID: "txn123",
				Amount:        "100.50",
				Currency:      "TRY",
				Message:       "Payment failed",
				ErrorCode:     errorCodeInsufficientFunds,
			},
			expectedStatus:  provider.StatusFailed,
			expectedSuccess: false,
		},
		{
			name: "cancelled payment",
			paycellResp: PaycellResponse{
				Success:       false,
				Status:        statusCancelled,
				PaymentID:     "pay123",
				TransactionID: "txn123",
				Amount:        "100.50",
				Currency:      "TRY",
				Message:       "Payment cancelled",
			},
			expectedStatus:  provider.StatusCancelled,
			expectedSuccess: true,
		},
		{
			name: "3D payment with redirect",
			paycellResp: PaycellResponse{
				Success:       false,
				Status:        statusPending,
				PaymentID:     "pay123",
				TransactionID: "txn123",
				Amount:        "100.50",
				Currency:      "TRY",
				Message:       "3D authentication required",
				RedirectURL:   "https://3ds.paycell.com/auth",
				HTML:          "<form>...</form>",
			},
			expectedStatus:  provider.StatusPending,
			expectedSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.mapToPaymentResponse(tt.paycellResp)

			if result.Success != tt.expectedSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectedSuccess, result.Success)
			}
			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, result.Status)
			}
			if result.PaymentID != tt.paycellResp.PaymentID {
				t.Errorf("Expected paymentId '%s', got '%s'", tt.paycellResp.PaymentID, result.PaymentID)
			}
			if result.TransactionID != tt.paycellResp.TransactionID {
				t.Errorf("Expected transactionId '%s', got '%s'", tt.paycellResp.TransactionID, result.TransactionID)
			}
			expectedAmount := 100.50 // Since all test cases use "100.50"
			if result.Amount != expectedAmount {
				t.Errorf("Expected amount %f, got %f", expectedAmount, result.Amount)
			}
			if result.Currency != tt.paycellResp.Currency {
				t.Errorf("Expected currency '%s', got '%s'", tt.paycellResp.Currency, result.Currency)
			}
			if result.Message != tt.paycellResp.Message {
				t.Errorf("Expected message '%s', got '%s'", tt.paycellResp.Message, result.Message)
			}
			if result.ErrorCode != tt.paycellResp.ErrorCode {
				t.Errorf("Expected errorCode '%s', got '%s'", tt.paycellResp.ErrorCode, result.ErrorCode)
			}
			if result.RedirectURL != tt.paycellResp.RedirectURL {
				t.Errorf("Expected redirectUrl '%s', got '%s'", tt.paycellResp.RedirectURL, result.RedirectURL)
			}
			if result.HTML != tt.paycellResp.HTML {
				t.Errorf("Expected html '%s', got '%s'", tt.paycellResp.HTML, result.HTML)
			}
		})
	}
}

func TestPaycellProvider_GenerateSignature(t *testing.T) {
	p := &PaycellProvider{}

	tests := []struct {
		name     string
		data     string
		expected string
	}{
		{
			name:     "simple string",
			data:     "test",
			expected: "098f6bcd4621d373cade4e832627b4f6",
		},
		{
			name: "complex auth string",
			data: "POST|/api/payments|{\"amount\":10050}|1234567890|secret",
			expected: func() string {
				hash := md5.Sum([]byte("POST|/api/payments|{\"amount\":10050}|1234567890|secret"))
				return hex.EncodeToString(hash[:])
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.generateSignature(tt.data)
			if result != tt.expected {
				t.Errorf("Expected signature '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestPaycellProvider_CreatePayment(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.URL.Path != endpointProvision {
			t.Errorf("Expected path %s, got %s", endpointProvision, r.URL.Path)
		}

		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("X-Paycell-Username") == "" {
			t.Error("Expected X-Paycell-Username header")
		}
		if r.Header.Get("X-Paycell-Signature") == "" {
			t.Error("Expected X-Paycell-Signature header")
		}

		// Mock successful response
		response := PaycellResponse{
			Success:       true,
			Status:        statusSuccess,
			PaymentID:     "pay123",
			TransactionID: "txn123",
			Amount:        "100.50",
			Currency:      "TRY",
			Message:       "Payment successful",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Initialize provider
	p := NewProvider().(*PaycellProvider)
	config := map[string]string{
		"username":   "test_user",
		"password":   "test_pass",
		"merchantId": "test_merchant",
		"terminalId": "test_terminal",
	}
	err := p.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Override baseURL to use test server
	p.baseURL = server.URL

	// Create payment request
	request := provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:        "John",
			Surname:     "Doe",
			Email:       "john@example.com",
			PhoneNumber: "5551234567",
		},
		CardInfo: provider.CardInfo{
			CardNumber:  "5528790000000008",
			ExpireMonth: "12",
			ExpireYear:  "2030",
			CVV:         "123",
		},
	}

	// Test payment creation
	ctx := context.Background()
	response, err := p.CreatePayment(ctx, request)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if !response.Success {
		t.Error("Expected successful payment")
	}
	if response.Status != provider.StatusSuccessful {
		t.Errorf("Expected status %v, got %v", provider.StatusSuccessful, response.Status)
	}
	if response.Amount != request.Amount {
		t.Errorf("Expected amount %f, got %f", request.Amount, response.Amount)
	}
}

func TestPaycellProvider_ValidateWebhook(t *testing.T) {
	p := &PaycellProvider{
		password: "test_secret",
	}

	ctx := context.Background()

	// Test data
	data := map[string]string{
		"paymentId":     "pay123",
		"status":        "SUCCESS",
		"transactionId": "txn123",
		"amount":        "10050",
		"currency":      "TRY",
	}

	// Generate valid signature
	rawData, _ := json.Marshal(data)
	validSignature := p.generateSignature(string(rawData))

	tests := []struct {
		name          string
		data          map[string]string
		headers       map[string]string
		expectedValid bool
		expectError   bool
		errorMsg      string
	}{
		{
			name: "valid webhook",
			data: data,
			headers: map[string]string{
				"X-Paycell-Signature": validSignature,
			},
			expectedValid: true,
			expectError:   false,
		},
		{
			name:          "missing signature",
			data:          data,
			headers:       map[string]string{},
			expectedValid: false,
			expectError:   true,
			errorMsg:      "missing webhook signature",
		},
		{
			name: "invalid signature",
			data: data,
			headers: map[string]string{
				"X-Paycell-Signature": "invalid_signature",
			},
			expectedValid: false,
			expectError:   true,
			errorMsg:      "invalid webhook signature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, result, err := p.ValidateWebhook(ctx, tt.data, tt.headers)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			}

			if valid != tt.expectedValid {
				t.Errorf("Expected valid %v, got %v", tt.expectedValid, valid)
			}

			if tt.expectedValid && result != nil {
				if result["paymentId"] != data["paymentId"] {
					t.Errorf("Expected paymentId '%s', got '%s'", data["paymentId"], result["paymentId"])
				}
			}
		})
	}
}

func TestPaycellProvider_GetRequiredConfig(t *testing.T) {
	provider := NewProvider().(*PaycellProvider)

	tests := []struct {
		name        string
		environment string
		expected    int
	}{
		{"sandbox environment", "sandbox", 5},
		{"production environment", "production", 5},
		{"test environment", "test", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetRequiredConfig(tt.environment)
			if len(result) != tt.expected {
				t.Errorf("GetRequiredConfig() returned %d fields, want %d", len(result), tt.expected)
			}

			// Check required fields
			expectedFields := []string{"username", "password", "merchantId", "terminalId", "environment"}
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

func TestPaycellProvider_ValidateConfig(t *testing.T) {
	provider := NewProvider().(*PaycellProvider)

	tests := []struct {
		name        string
		config      map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid sandbox config",
			config: map[string]string{
				"username":    "PAYCELL_USER_TEST",
				"password":    "PAYCELL_PASS_123456",
				"merchantId":  "MERCHANT123",
				"terminalId":  "VP123456",
				"environment": "sandbox",
			},
			expectError: false,
		},
		{
			name: "valid production config",
			config: map[string]string{
				"username":    "PAYCELL_USER_PROD",
				"password":    "PAYCELL_PASS_PROD123456",
				"merchantId":  "PRODMERCHANT123",
				"terminalId":  "VPPROD123456",
				"environment": "production",
			},
			expectError: false,
		},
		{
			name: "missing username",
			config: map[string]string{
				"password":    "PAYCELL_PASS_123456",
				"merchantId":  "MERCHANT123",
				"terminalId":  "VP123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'username' is missing",
		},
		{
			name: "missing password",
			config: map[string]string{
				"username":    "PAYCELL_USER_TEST",
				"merchantId":  "MERCHANT123",
				"terminalId":  "VP123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'password' is missing",
		},
		{
			name: "missing merchantId",
			config: map[string]string{
				"username":    "PAYCELL_USER_TEST",
				"password":    "PAYCELL_PASS_123456",
				"terminalId":  "VP123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'merchantId' is missing",
		},
		{
			name: "missing terminalId",
			config: map[string]string{
				"username":    "PAYCELL_USER_TEST",
				"password":    "PAYCELL_PASS_123456",
				"merchantId":  "MERCHANT123",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'terminalId' is missing",
		},
		{
			name: "empty username",
			config: map[string]string{
				"username":    "",
				"password":    "PAYCELL_PASS_123456",
				"merchantId":  "MERCHANT123",
				"terminalId":  "VP123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'username' cannot be empty",
		},
		{
			name: "invalid environment",
			config: map[string]string{
				"username":    "PAYCELL_USER_TEST",
				"password":    "PAYCELL_PASS_123456",
				"merchantId":  "MERCHANT123",
				"terminalId":  "VP123456",
				"environment": "invalid_env",
			},
			expectError: true,
			errorMsg:    "environment must be one of",
		},
		{
			name: "username too short",
			config: map[string]string{
				"username":    "AB",
				"password":    "PAYCELL_PASS_123456",
				"merchantId":  "MERCHANT123",
				"terminalId":  "VP123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 3 characters",
		},
		{
			name: "password too short",
			config: map[string]string{
				"username":    "PAYCELL_USER_TEST",
				"password":    "12345",
				"merchantId":  "MERCHANT123",
				"terminalId":  "VP123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 6 characters",
		},
		{
			name: "merchantId too short",
			config: map[string]string{
				"username":    "PAYCELL_USER_TEST",
				"password":    "PAYCELL_PASS_123456",
				"merchantId":  "ABCD",
				"terminalId":  "VP123456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 5 characters",
		},
		{
			name: "terminalId too short",
			config: map[string]string{
				"username":    "PAYCELL_USER_TEST",
				"password":    "PAYCELL_PASS_123456",
				"merchantId":  "MERCHANT123",
				"terminalId":  "ABCD",
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
