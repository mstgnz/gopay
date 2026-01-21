package ziraat

import (
	"context"
	"strings"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

func TestNewProvider(t *testing.T) {
	p := NewProvider()
	if p == nil {
		t.Fatal("NewProvider should return a non-nil provider")
	}

	ziraatProvider, ok := p.(*ZiraatProvider)
	if !ok {
		t.Fatal("NewProvider should return a ZiraatProvider instance")
	}

	// HTTP client is created only after Initialize() is called
	if ziraatProvider.httpClient != nil {
		t.Error("ZiraatProvider should have nil HTTP client before Initialize()")
	}

	// Test that Initialize creates the client properly
	config := map[string]string{
		"username":    "test_user",
		"password":    "test_pass",
		"storeKey":    "test_store_key",
		"environment": "sandbox",
	}

	err := ziraatProvider.Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if ziraatProvider.httpClient == nil {
		t.Error("ZiraatProvider should have a non-nil HTTP client after Initialize()")
	}
}

func TestZiraatProvider_Initialize(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			config: map[string]string{
				"username":    "test_user",
				"password":    "test_pass",
				"storeKey":    "test_store_key",
				"environment": "sandbox",
			},
			expectError: false,
		},
		{
			name: "production environment",
			config: map[string]string{
				"username":    "test_user",
				"password":    "test_pass",
				"storeKey":    "test_store_key",
				"environment": "production",
			},
			expectError: false,
		},
		{
			name: "missing username",
			config: map[string]string{
				"password":    "test_pass",
				"storeKey":    "test_store_key",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "username, password and storeKey are required",
		},
		{
			name: "missing password",
			config: map[string]string{
				"username":    "test_user",
				"storeKey":    "test_store_key",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "username, password and storeKey are required",
		},
		{
			name: "missing storeKey",
			config: map[string]string{
				"username":    "test_user",
				"password":    "test_pass",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "username, password and storeKey are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider().(*ZiraatProvider)
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
				if p.storeKey != tt.config["storeKey"] {
					t.Errorf("Expected storeKey '%s', got '%s'", tt.config["storeKey"], p.storeKey)
				}

				// Verify environment setting
				if tt.config["environment"] == "production" {
					if !p.isProduction || p.baseURL != apiProductionURL {
						t.Error("Production environment not set correctly")
					}
				} else {
					if p.isProduction || p.baseURL != apiSandboxURL {
						t.Error("Sandbox environment not set correctly")
					}
				}

				// Verify 3D gateway URL
				if p.threeDPostURL != api3DSandboxURL {
					t.Errorf("Expected threeDPostURL '%s', got '%s'", api3DSandboxURL, p.threeDPostURL)
				}
			}
		})
	}
}

func TestZiraatProvider_ValidatePaymentRequest(t *testing.T) {
	p := &ZiraatProvider{}

	validRequest := provider.PaymentRequest{
		TenantID: 1,
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
			name:        "valid 3D request",
			request:     validRequest,
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
			is3D:        true,
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
			is3D:        true,
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
			is3D:        true,
			expectError: true,
			errorMsg:    "customer email is required",
		},
		{
			name: "missing card number",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.CardNumber = ""
				return req
			}(),
			is3D:        true,
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
			is3D:        true,
			expectError: true,
			errorMsg:    "CVV is required",
		},
		{
			name: "missing expiration month",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireMonth = ""
				return req
			}(),
			is3D:        true,
			expectError: true,
			errorMsg:    "card expiration month and year are required",
		},
		{
			name: "missing expiration year",
			request: func() provider.PaymentRequest {
				req := validRequest
				req.CardInfo.ExpireYear = ""
				return req
			}(),
			is3D:        true,
			expectError: true,
			errorMsg:    "card expiration month and year are required",
		},
		{
			name: "missing callback URL for 3D",
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

func TestZiraatProvider_Calculate3DHash(t *testing.T) {
	p := &ZiraatProvider{
		storeKey: "TEST1234",
	}

	tests := []struct {
		name    string
		params  map[string]string
		wantErr bool
	}{
		{
			name: "valid params",
			params: map[string]string{
				"clientid":  "100200127",
				"amount":    "100.00",
				"okurl":     "https://example.com/ok",
				"failUrl":   "https://example.com/fail",
				"TranType":  "Auth",
				"currency":  "949",
				"rnd":       "1234567890.123456",
				"storetype": "3D_PAY",
			},
			wantErr: false,
		},
		{
			name: "params with hash excluded",
			params: map[string]string{
				"clientid": "100200127",
				"amount":   "100.00",
				"HASH":     "should_be_excluded",
				"encoding": "should_be_excluded",
				"storekey": "should_be_excluded",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := p.calculate3DHash(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("calculate3DHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && hash == "" {
				t.Error("calculate3DHash() returned empty hash")
			}
		})
	}
}

func TestZiraatProvider_Build3DFormParams(t *testing.T) {
	p := &ZiraatProvider{
		username: "test_user",
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
			CardNumber:     "5528790000000008",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CVV:            "123",
			CardHolderName: "John Doe",
		},
		InstallmentCount: 1,
	}

	callbackURL := "https://example.com/callback"
	params := p.build3DFormParams(request, callbackURL)

	// Check required fields
	requiredFields := []string{"clientid", "amount", "okurl", "failUrl", "TranType", "callbackUrl", "currency", "rnd", "storetype", "hashAlgorithm", "pan", "cv2", "Ecom_Payment_Card_ExpDate_Year", "Ecom_Payment_Card_ExpDate_Month", "cardType"}
	for _, field := range requiredFields {
		if _, ok := params[field]; !ok {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// Check values
	if params["clientid"] != "test_user" {
		t.Errorf("Expected clientid 'test_user', got '%s'", params["clientid"])
	}
	if params["amount"] != "100.50" {
		t.Errorf("Expected amount '100.50', got '%s'", params["amount"])
	}
	if params["cardType"] != "2" { // MasterCard (starts with 5)
		t.Errorf("Expected cardType '2' for MasterCard, got '%s'", params["cardType"])
	}
	if params["Ecom_Payment_Card_ExpDate_Year"] != "30" {
		t.Errorf("Expected year '30', got '%s'", params["Ecom_Payment_Card_ExpDate_Year"])
	}

	// Check that password and storekey are NOT in params
	if _, ok := params["password"]; ok {
		t.Error("password should not be in form params")
	}
	if _, ok := params["storekey"]; ok {
		t.Error("storekey should not be in form params")
	}
}

func TestZiraatProvider_CreatePaymentAlwaysUses3D(t *testing.T) {
	p := NewProvider().(*ZiraatProvider)
	config := map[string]string{
		"username":    "test_user",
		"password":    "test_pass",
		"storeKey":    "test_store_key",
		"environment": "sandbox",
	}
	err := p.Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	request := provider.PaymentRequest{
		TenantID: 1,
		Amount:   100.50,
		Currency: "TRY",
		Customer: provider.Customer{
			Name:    "John",
			Surname: "Doe",
			Email:   "john@example.com",
		},
		CardInfo: provider.CardInfo{
			CardNumber:     "5528790000000008",
			ExpireMonth:    "12",
			ExpireYear:     "2030",
			CVV:            "123",
			CardHolderName: "John Doe",
		},
		CallbackURL: "https://example.com/callback",
	}

	ctx := context.Background()

	// CreatePayment should always use 3D (returns HTML form)
	response, err := p.CreatePayment(ctx, request)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response should not be nil")
	}

	// Should return HTML form for 3D
	if response.HTML == "" {
		t.Error("CreatePayment should return HTML form for 3D Secure")
	}

	if !strings.Contains(response.HTML, "threeDForm") {
		t.Error("HTML should contain 3D form")
	}
}

func TestZiraatProvider_Complete3DPayment(t *testing.T) {
	p := NewProvider().(*ZiraatProvider)
	config := map[string]string{
		"username":    "test_user",
		"password":    "test_pass",
		"storeKey":    "test_store_key",
		"environment": "sandbox",
	}
	err := p.Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	callbackState := &provider.CallbackState{
		TenantID:         1,
		PaymentID:        "test_payment_123",
		OriginalCallback: "https://example.com/callback",
		Amount:           100.50,
		Currency:         "TRY",
		LogID:            1,
		Provider:         "ziraat",
		Environment:      "sandbox",
	}

	tests := []struct {
		name        string
		data        map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "missing HASH",
			data: map[string]string{
				"mdStatus": "1",
				"Response": "Approved",
			},
			expectError: true,
			errorMsg:    "missing HASH",
		},
		{
			name: "valid callback data",
			data: map[string]string{
				"HASH":     "test_hash_value",
				"mdStatus": "1",
				"Response": "Approved",
				"TransId":  "test_trans_123",
				"oid":      "test_order_123",
			},
			expectError: false,
		},
		{
			name: "failed payment",
			data: map[string]string{
				"HASH":     "test_hash_value",
				"mdStatus": "5",
				"Response": "Declined",
				"ErrMsg":   "Payment failed",
				"TransId":  "test_trans_123",
				"oid":      "test_order_123",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			response, err := p.Complete3DPayment(ctx, callbackState, tt.data)

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
				if response == nil {
					t.Fatal("Response should not be nil")
				}
			}
		})
	}
}
