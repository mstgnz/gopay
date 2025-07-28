package iyzico

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

	iyzicoProvider, ok := provider.(*IyzicoProvider)
	if !ok {
		t.Error("NewProvider() should return *IyzicoProvider")
	}

	if iyzicoProvider.httpClient == nil {
		t.Error("HTTP client should be initialized")
	}

	// Note: We can't directly access config.Timeout as it's unexported
	// The timeout is set during Initialize, so we'll test it there
}

func TestIyzicoProvider_Initialize(t *testing.T) {
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
				"environment": "production",
			},
			expectError: false,
			expectProd:  true,
			expectURL:   apiProductionURL,
		},
		{
			name: "Default to sandbox",
			config: map[string]string{
				"apiKey":    "test-api-key",
				"secretKey": "test-secret-key",
			},
			expectError: false,
			expectProd:  false,
			expectURL:   apiSandboxURL,
		},
		{
			name: "Invalid environment defaults to sandbox",
			config: map[string]string{
				"apiKey":      "test-api-key",
				"secretKey":   "test-secret-key",
				"environment": "invalid",
			},
			expectError: false,
			expectProd:  false,
			expectURL:   apiSandboxURL,
		},
		{
			name: "Missing apiKey",
			config: map[string]string{
				"secretKey":   "test-secret-key",
				"environment": "sandbox",
			},
			expectError: true,
		},
		{
			name: "Missing secretKey",
			config: map[string]string{
				"apiKey":      "test-api-key",
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
			provider := NewProvider().(*IyzicoProvider)
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

			if provider.isProduction != tt.expectProd {
				t.Errorf("Expected isProduction %v, got %v", tt.expectProd, provider.isProduction)
			}

			if provider.baseURL != tt.expectURL {
				t.Errorf("Expected baseURL %s, got %s", tt.expectURL, provider.baseURL)
			}
		})
	}
}

func TestIyzicoProvider_ValidatePaymentRequest(t *testing.T) {
	iyzicoProvider := &IyzicoProvider{}

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
			err := iyzicoProvider.validatePaymentRequest(tt.request, tt.is3D)

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

func TestIyzicoProvider_MapToIyzicoPaymentRequest(t *testing.T) {
	iyzicoProvider := &IyzicoProvider{}

	request := provider.PaymentRequest{
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{
			ID:      "customer123",
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
			Address: &provider.Address{
				City:    "Istanbul",
				Country: "Turkey",
				Address: "Test Address",
				ZipCode: "34000",
			},
		},
		CardInfo: provider.CardInfo{
			CardHolderName: "John Doe",
			CardNumber:     "5528790000000008",
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
		is3D     bool
		validate func(t *testing.T, result map[string]any)
	}{
		{
			name:    "Regular payment request",
			request: request,
			is3D:    false,
			validate: func(t *testing.T, result map[string]any) {
				if result["price"] != "100.50" {
					t.Errorf("Expected price '100.50', got %v", result["price"])
				}
				if result["currency"] != "TRY" {
					t.Errorf("Expected currency 'TRY', got %v", result["currency"])
				}
				if result["installment"] != 1 {
					t.Errorf("Expected installment 1, got %v", result["installment"])
				}
				if result["locale"] != defaultLocale {
					t.Errorf("Expected locale '%s', got %v", defaultLocale, result["locale"])
				}

				buyer, ok := result["buyer"].(map[string]any)
				if !ok {
					t.Error("buyer should be a map")
					return
				}
				if buyer["email"] != "john@example.com" {
					t.Errorf("Expected buyer email 'john@example.com', got %v", buyer["email"])
				}

				paymentCard, ok := result["paymentCard"].(map[string]any)
				if !ok {
					t.Error("paymentCard should be a map")
					return
				}
				if paymentCard["cardNumber"] != "5528790000000008" {
					t.Errorf("Expected card number '5528790000000008', got %v", paymentCard["cardNumber"])
				}
			},
		},
		{
			name:    "3D payment request",
			request: request,
			is3D:    true,
			validate: func(t *testing.T, result map[string]any) {
				// Updated to expect GoPay callback URL format with originalCallbackUrl parameter
				expectedPattern := "/v1/callback/iyzico?originalCallbackUrl=https://example.com/callback"
				callbackURL := result["callbackUrl"].(string)
				if !strings.Contains(callbackURL, expectedPattern) {
					t.Errorf("Expected callbackUrl to contain '%s', got %v", expectedPattern, callbackURL)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iyzicoProvider.mapToIyzicoPaymentRequest(tt.request, tt.is3D)
			tt.validate(t, result)
		})
	}
}

func TestIyzicoProvider_SendPaymentRequest(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse map[string]any
		statusCode     int
		expectError    bool
		expectedStatus provider.PaymentStatus
		expectedHTML   string
	}{
		{
			name: "Successful payment",
			serverResponse: map[string]any{
				"status":    statusSuccess,
				"paymentId": "payment123",
				"price":     "100.50",
				"currency":  "TRY",
			},
			statusCode:     200,
			expectError:    false,
			expectedStatus: provider.StatusSuccessful,
		},
		{
			name: "Failed payment",
			serverResponse: map[string]any{
				"status":       statusFailure,
				"errorCode":    "5007",
				"errorMessage": "Invalid card",
			},
			statusCode:     200,
			expectError:    false,
			expectedStatus: provider.StatusFailed,
		},
		{
			name: "3D Secure required",
			serverResponse: map[string]any{
				"status":             statusSuccess,
				"paymentId":          "payment123",
				"threeDSHtmlContent": "<html>3D Secure form</html>",
			},
			statusCode:     200,
			expectError:    false,
			expectedStatus: provider.StatusPending,
			expectedHTML:   "<html>3D Secure form</html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.serverResponse)
			}))
			defer server.Close()

			iyzicoProvider := &IyzicoProvider{
				apiKey:    "test-key",
				secretKey: "test-secret",
				baseURL:   server.URL,
				httpClient: provider.NewProviderHTTPClient(&provider.HTTPClientConfig{
					BaseURL: server.URL,
					Timeout: 5 * time.Second,
				}),
			}

			ctx := context.Background()
			requestData := map[string]any{
				"locale": "tr",
				"price":  "100.50",
			}

			response, err := iyzicoProvider.sendPaymentRequest(ctx, "/test", requestData)

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

			if response.Status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, response.Status)
			}

			if tt.expectedHTML != "" && response.HTML != tt.expectedHTML {
				t.Errorf("Expected HTML %s, got %s", tt.expectedHTML, response.HTML)
			}
		})
	}
}

func TestIyzicoProvider_CreatePayment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"status":    statusSuccess,
			"paymentId": "payment123",
			"price":     "100.50",
			"currency":  "TRY",
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	iyzicoProvider := &IyzicoProvider{
		apiKey:    "test-key",
		secretKey: "test-secret",
		baseURL:   server.URL,
		httpClient: provider.NewProviderHTTPClient(&provider.HTTPClientConfig{
			BaseURL: server.URL,
			Timeout: 5 * time.Second,
		}),
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
	response, err := iyzicoProvider.CreatePayment(ctx, request)

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

func TestIyzicoProvider_Create3DPayment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"status":             statusSuccess,
			"paymentId":          "payment123",
			"threeDSHtmlContent": "<html>3D form</html>",
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	iyzicoProvider := &IyzicoProvider{
		apiKey:    "test-key",
		secretKey: "test-secret",
		baseURL:   server.URL,
		httpClient: provider.NewProviderHTTPClient(&provider.HTTPClientConfig{
			BaseURL: server.URL,
			Timeout: 5 * time.Second,
		}),
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
		CallbackURL: "https://example.com/callback",
		Use3D:       true,
	}

	ctx := context.Background()
	response, err := iyzicoProvider.Create3DPayment(ctx, request)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if response.Status != provider.StatusPending {
		t.Errorf("Expected status pending, got %v", response.Status)
	}

	if response.HTML != "<html>3D form</html>" {
		t.Errorf("Expected HTML content, got %s", response.HTML)
	}
}

func TestIyzicoProvider_GetPaymentStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"status":    statusSuccess,
			"paymentId": "payment123",
			"price":     "100.50",
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	iyzicoProvider := &IyzicoProvider{
		apiKey:    "test-key",
		secretKey: "test-secret",
		baseURL:   server.URL,
		httpClient: provider.NewProviderHTTPClient(&provider.HTTPClientConfig{
			BaseURL: server.URL,
			Timeout: 5 * time.Second,
		}),
	}

	ctx := context.Background()

	// Test with valid payment ID
	response, err := iyzicoProvider.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: "payment123"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if !response.Success {
		t.Error("Expected successful response")
	}

	// Test with empty payment ID
	_, err = iyzicoProvider.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: ""})
	if err == nil {
		t.Error("Expected error for empty payment ID")
	}
}

func TestIyzicoProvider_RefundPayment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"status":               statusSuccess,
			"paymentTransactionId": "refund123",
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	iyzicoProvider := &IyzicoProvider{
		apiKey:    "test-key",
		secretKey: "test-secret",
		baseURL:   server.URL,
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
	response, err := iyzicoProvider.RefundPayment(ctx, refundRequest)

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

func TestGenerateAuthString(t *testing.T) {
	iyzicoProvider := &IyzicoProvider{
		apiKey:    "test-api-key",
		secretKey: "test-secret-key",
	}

	uri := "/payment/auth"
	body := `{"price":"100.00","currency":"TRY"}`

	authString := iyzicoProvider.generateAuthString(uri, body)

	if !strings.HasPrefix(authString, "IYZWS test-api-key:") {
		t.Errorf("Auth string should start with 'IYZWS test-api-key:', got %s", authString)
	}
}

func TestSortAndConcatRequest(t *testing.T) {
	jsonString := `{"currency":"TRY","price":"100.00","locale":"tr"}`
	result := sortAndConcatRequest(jsonString)

	expected := "currencyTRYlocaletrprice100.00"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
		hasError bool
	}{
		{"input 100.50", "100.50", 100.50, false},
		{"input 100,50", "100,50", 100.50, false},
		{"input 1000.99", "1000.99", 1000.99, false},
		{"input 1000,99", "1000,99", 1000.99, false},
		{"input invalid", "invalid", 0.0, true},
		{"input ", "", 0.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFloat(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %s", err.Error())
				}
				if result != tt.expected {
					t.Errorf("parseFloat(%s) = %f, want %f", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestIyzicoProvider_GetRequiredConfig(t *testing.T) {
	provider := NewProvider().(*IyzicoProvider)

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
			expectedFields := []string{"apiKey", "secretKey", "environment"}
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

func TestIyzicoProvider_ValidateConfig(t *testing.T) {
	provider := NewProvider().(*IyzicoProvider)

	tests := []struct {
		name        string
		config      map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid sandbox config",
			config: map[string]string{
				"apiKey":      "sandbox-BIOoONNaqF8UZZmP3fake123",
				"secretKey":   "sandbox-NjQwOTRkMDBkZmE1fake456",
				"environment": "sandbox",
			},
			expectError: false,
		},
		{
			name: "valid production config",
			config: map[string]string{
				"apiKey":      "production-BIOoONNaqF8UZZmP3real123",
				"secretKey":   "production-NjQwOTRkMDBkZmE1real456",
				"environment": "production",
			},
			expectError: false,
		},
		{
			name: "missing apiKey",
			config: map[string]string{
				"secretKey":   "sandbox-NjQwOTRkMDBkZmE1fake456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'apiKey' is missing",
		},
		{
			name: "missing secretKey",
			config: map[string]string{
				"apiKey":      "sandbox-BIOoONNaqF8UZZmP3fake123",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'secretKey' is missing",
		},
		{
			name: "empty apiKey",
			config: map[string]string{
				"apiKey":      "",
				"secretKey":   "sandbox-NjQwOTRkMDBkZmE1fake456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'apiKey' cannot be empty",
		},
		{
			name: "invalid environment",
			config: map[string]string{
				"apiKey":      "sandbox-BIOoONNaqF8UZZmP3fake123",
				"secretKey":   "sandbox-NjQwOTRkMDBkZmE1fake456",
				"environment": "invalid_env",
			},
			expectError: true,
			errorMsg:    "environment must be one of",
		},
		{
			name: "apiKey too short",
			config: map[string]string{
				"apiKey":      "short",
				"secretKey":   "sandbox-NjQwOTRkMDBkZmE1fake456",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 20 characters",
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
