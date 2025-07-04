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
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
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
			name: "missing customer email",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Customer.Email = ""
				return req
			}(),
			is3D:        false,
			expectError: true,
			errorMsg:    "customer email is required",
		},
		{
			name: "missing customer name",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Customer.Name = ""
				return req
			}(),
			is3D:        false,
			expectError: true,
			errorMsg:    "customer name is required",
		},
		{
			name: "missing customer surname",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.Customer.Surname = ""
				return req
			}(),
			is3D:        false,
			expectError: true,
			errorMsg:    "customer surname is required",
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
			errorMsg:    "CVV is required",
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
			errorMsg:    "expire month is required",
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
			errorMsg:    "expire year is required",
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
			Address: provider.Address{
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

			// Verify basic fields
			if result["merchantId"] != p.merchantID {
				t.Errorf("Expected merchantId '%s', got '%s'", p.merchantID, result["merchantId"])
			}
			if result["terminalId"] != p.terminalID {
				t.Errorf("Expected terminalId '%s', got '%s'", p.terminalID, result["terminalId"])
			}

			// Verify orderId is present
			if _, exists := result["orderId"]; !exists {
				t.Error("Expected orderId to be present")
			}

			// Amount should be string format
			expectedAmount := fmt.Sprintf("%.2f", tt.request.Amount)
			if result["amount"] != expectedAmount {
				t.Errorf("Expected amount '%s', got '%v'", expectedAmount, result["amount"])
			}
			if result["currency"] != tt.request.Currency {
				t.Errorf("Expected currency '%s', got '%s'", tt.request.Currency, result["currency"])
			}

			// Verify customer data (flat structure)
			expectedCustomerName := tt.request.Customer.Name + " " + tt.request.Customer.Surname
			if result["customerName"] != expectedCustomerName {
				t.Errorf("Expected customerName '%s', got '%s'", expectedCustomerName, result["customerName"])
			}
			if result["customerEmail"] != tt.request.Customer.Email {
				t.Errorf("Expected customerEmail '%s', got '%s'", tt.request.Customer.Email, result["customerEmail"])
			}

			// Verify card data (flat structure)
			if result["cardNumber"] != tt.request.CardInfo.CardNumber {
				t.Errorf("Expected cardNumber '%s', got '%s'", tt.request.CardInfo.CardNumber, result["cardNumber"])
			}

			// Verify 3D specific fields
			if tt.is3D {
				if result["secure3d"] != "true" {
					t.Error("Expected secure3d to be 'true' string for 3D payments")
				}

				successURL := result["successUrl"].(string)
				failureURL := result["failureUrl"].(string)

				if tt.request.CallbackURL != "" {
					// Expect GoPay callback URL format with originalCallbackUrl parameter
					expectedPattern := "/v1/callback/paycell?originalCallbackUrl=" + tt.request.CallbackURL
					if !strings.Contains(successURL, expectedPattern) {
						t.Errorf("Expected successUrl to contain '%s', got '%s'", expectedPattern, successURL)
					}
					if !strings.Contains(failureURL, expectedPattern) {
						t.Errorf("Expected failureUrl to contain '%s', got '%s'", expectedPattern, failureURL)
					}
				} else {
					// Expect direct GoPay callback URL
					expectedPattern := "/v1/callback/paycell"
					if !strings.Contains(successURL, expectedPattern) {
						t.Errorf("Expected successUrl to contain '%s', got '%s'", expectedPattern, successURL)
					}
					if !strings.Contains(failureURL, expectedPattern) {
						t.Errorf("Expected failureUrl to contain '%s', got '%s'", expectedPattern, failureURL)
					}
				}
			} else {
				if _, exists := result["secure3d"]; exists {
					t.Error("secure3d should not be set for non-3D payments")
				}
				if _, exists := result["successUrl"]; exists {
					t.Error("successUrl should not be set for non-3D payments")
				}
				if _, exists := result["failureUrl"]; exists {
					t.Error("failureUrl should not be set for non-3D payments")
				}
			}

			// Note: Items are no longer included in the new Paycell API format
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
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
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
