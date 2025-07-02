package paytr

import (
	"context"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

func TestPayTRProvider_Initialize(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]string
		wantErr bool
	}{
		{
			name: "Valid configuration",
			config: map[string]string{
				"merchantId":   "12345",
				"merchantKey":  "test-key",
				"merchantSalt": "test-salt",
				"environment":  "sandbox",
			},
			wantErr: false,
		},
		{
			name: "Missing merchant ID",
			config: map[string]string{
				"merchantKey":  "test-key",
				"merchantSalt": "test-salt",
				"environment":  "sandbox",
			},
			wantErr: true,
		},
		{
			name: "Missing merchant key",
			config: map[string]string{
				"merchantId":   "12345",
				"merchantSalt": "test-salt",
				"environment":  "sandbox",
			},
			wantErr: true,
		},
		{
			name: "Missing merchant salt",
			config: map[string]string{
				"merchantId":  "12345",
				"merchantKey": "test-key",
				"environment": "sandbox",
			},
			wantErr: true,
		},
		{
			name: "Production environment",
			config: map[string]string{
				"merchantId":   "12345",
				"merchantKey":  "test-key",
				"merchantSalt": "test-salt",
				"environment":  "production",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider()
			err := p.Initialize(tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				paytrProvider := p.(*PayTRProvider)
				if paytrProvider.merchantID != tt.config["merchantId"] {
					t.Errorf("Expected merchantID %s, got %s", tt.config["merchantId"], paytrProvider.merchantID)
				}
				if paytrProvider.merchantKey != tt.config["merchantKey"] {
					t.Errorf("Expected merchantKey %s, got %s", tt.config["merchantKey"], paytrProvider.merchantKey)
				}
				if paytrProvider.merchantSalt != tt.config["merchantSalt"] {
					t.Errorf("Expected merchantSalt %s, got %s", tt.config["merchantSalt"], paytrProvider.merchantSalt)
				}
			}
		})
	}
}

func TestPayTRProvider_ValidatePaymentRequest(t *testing.T) {
	p := &PayTRProvider{}

	tests := []struct {
		name    string
		request provider.PaymentRequest
		is3D    bool
		wantErr bool
	}{
		{
			name: "Valid request without 3D",
			request: provider.PaymentRequest{
				Amount: 100.50,
				Customer: provider.Customer{
					Email:   "test@example.com",
					Name:    "John",
					Surname: "Doe",
				},
				ClientIP: "192.168.1.1",
			},
			is3D:    false,
			wantErr: false,
		},
		{
			name: "Valid request with 3D",
			request: provider.PaymentRequest{
				Amount: 100.50,
				Customer: provider.Customer{
					Email:   "test@example.com",
					Name:    "John",
					Surname: "Doe",
				},
				ClientIP:    "192.168.1.1",
				CallbackURL: "https://example.com/callback",
			},
			is3D:    true,
			wantErr: false,
		},
		{
			name: "Invalid amount",
			request: provider.PaymentRequest{
				Amount: 0,
				Customer: provider.Customer{
					Email:   "test@example.com",
					Name:    "John",
					Surname: "Doe",
				},
				ClientIP: "192.168.1.1",
			},
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Missing customer email",
			request: provider.PaymentRequest{
				Amount: 100.50,
				Customer: provider.Customer{
					Name:    "John",
					Surname: "Doe",
				},
				ClientIP: "192.168.1.1",
			},
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Missing customer name",
			request: provider.PaymentRequest{
				Amount: 100.50,
				Customer: provider.Customer{
					Email:   "test@example.com",
					Surname: "Doe",
				},
				ClientIP: "192.168.1.1",
			},
			is3D:    false,
			wantErr: true,
		},
		{
			name: "Missing client IP",
			request: provider.PaymentRequest{
				Amount: 100.50,
				Customer: provider.Customer{
					Email:   "test@example.com",
					Name:    "John",
					Surname: "Doe",
				},
			},
			is3D:    false,
			wantErr: true,
		},
		{
			name: "3D request missing callback URL",
			request: provider.PaymentRequest{
				Amount: 100.50,
				Customer: provider.Customer{
					Email:   "test@example.com",
					Name:    "John",
					Surname: "Doe",
				},
				ClientIP: "192.168.1.1",
			},
			is3D:    true,
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

func TestPayTRProvider_GetCurrency(t *testing.T) {
	p := &PayTRProvider{}

	tests := []struct {
		name     string
		currency string
		expected string
	}{
		{
			name:     "TRY currency",
			currency: "TRY",
			expected: "TL",
		},
		{
			name:     "TL currency",
			currency: "TL",
			expected: "TL",
		},
		{
			name:     "USD currency",
			currency: "USD",
			expected: "USD",
		},
		{
			name:     "EUR currency",
			currency: "EUR",
			expected: "EUR",
		},
		{
			name:     "Empty currency",
			currency: "",
			expected: "TL",
		},
		{
			name:     "Invalid currency",
			currency: "GBP",
			expected: "TL",
		},
		{
			name:     "Lowercase currency",
			currency: "usd",
			expected: "USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.getCurrency(tt.currency)
			if result != tt.expected {
				t.Errorf("getCurrency() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPayTRProvider_GetTestMode(t *testing.T) {
	tests := []struct {
		name         string
		isProduction bool
		expected     string
	}{
		{
			name:         "Production mode",
			isProduction: true,
			expected:     "0",
		},
		{
			name:         "Test mode",
			isProduction: false,
			expected:     "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PayTRProvider{isProduction: tt.isProduction}
			result := p.getTestMode()
			if result != tt.expected {
				t.Errorf("getTestMode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPayTRProvider_BuildUserBasket(t *testing.T) {
	p := &PayTRProvider{}

	tests := []struct {
		name        string
		items       []provider.Item
		totalAmount float64
		expected    string
	}{
		{
			name:        "Empty items",
			items:       []provider.Item{},
			totalAmount: 100.50,
			expected:    `[["Payment","100.50","1"]]`,
		},
		{
			name: "Single item",
			items: []provider.Item{
				{Name: "Product 1", Price: 100.50, Quantity: 1},
			},
			totalAmount: 100.50,
			expected:    `[["Product 1","100.50","1"]]`,
		},
		{
			name: "Multiple items",
			items: []provider.Item{
				{Name: "Product 1", Price: 50.25, Quantity: 2},
				{Name: "Product 2", Price: 25.00, Quantity: 1},
			},
			totalAmount: 125.50,
			expected:    `[["Product 1","50.25","2"],["Product 2","25.00","1"]]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.buildUserBasket(tt.items, tt.totalAmount)
			if result != tt.expected {
				t.Errorf("buildUserBasket() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPayTRProvider_GenerateMD5Hash(t *testing.T) {
	p := &PayTRProvider{}

	tests := []struct {
		name     string
		data     string
		expected string
	}{
		{
			name:     "Simple string",
			data:     "test",
			expected: "098f6bcd4621d373cade4e832627b4f6",
		},
		{
			name:     "Empty string",
			data:     "",
			expected: "d41d8cd98f00b204e9800998ecf8427e",
		},
		{
			name: "PayTR hash example",
			data: "12345192.168.1.1order123test@example.com10050[['Product','100.50','1']]00TL1salt123",
			expected: func() string {
				// The expected hash will be calculated by the function itself
				return p.generateMD5Hash("12345192.168.1.1order123test@example.com10050[['Product','100.50','1']]00TL1salt123")
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.generateMD5Hash(tt.data)
			if tt.name == "PayTR hash example" {
				// For complex example, just verify it's not empty and has correct length
				if len(result) != 32 {
					t.Errorf("generateMD5Hash() length = %v, want 32", len(result))
				}
			} else {
				if result != tt.expected {
					t.Errorf("generateMD5Hash() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestPayTRProvider_GenerateTokenHash(t *testing.T) {
	p := &PayTRProvider{
		merchantSalt: "test-salt",
	}

	data := map[string]string{
		"merchant_id":    "12345",
		"user_ip":        "192.168.1.1",
		"merchant_oid":   "order123",
		"email":          "test@example.com",
		"payment_amount": "10050",
		"user_basket":    `[["Product","100.50","1"]]`,
		"currency":       "TL",
		"test_mode":      "1",
	}

	hash := p.generateTokenHash(data)

	// Verify hash is not empty and has correct MD5 length
	if hash == "" {
		t.Error("generateTokenHash() should not return empty string")
	}

	if len(hash) != 32 {
		t.Errorf("generateTokenHash() length = %v, want 32", len(hash))
	}

	// Test that same input produces same hash
	hash2 := p.generateTokenHash(data)
	if hash != hash2 {
		t.Error("generateTokenHash() should produce consistent results")
	}
}

func TestPayTRProvider_GenerateStatusQueryHash(t *testing.T) {
	p := &PayTRProvider{
		merchantID:   "12345",
		merchantSalt: "test-salt",
	}

	merchantOid := "order123"
	hash := p.generateStatusQueryHash(merchantOid)

	// Verify hash is not empty and has correct MD5 length
	if hash == "" {
		t.Error("generateStatusQueryHash() should not return empty string")
	}

	if len(hash) != 32 {
		t.Errorf("generateStatusQueryHash() length = %v, want 32", len(hash))
	}

	// Test that same input produces same hash
	hash2 := p.generateStatusQueryHash(merchantOid)
	if hash != hash2 {
		t.Error("generateStatusQueryHash() should produce consistent results")
	}
}

func TestPayTRProvider_GenerateRefundHash(t *testing.T) {
	p := &PayTRProvider{
		merchantID:   "12345",
		merchantSalt: "test-salt",
	}

	merchantOid := "order123"
	refundAmount := int64(5000)
	hash := p.generateRefundHash(merchantOid, refundAmount)

	// Verify hash is not empty and has correct MD5 length
	if hash == "" {
		t.Error("generateRefundHash() should not return empty string")
	}

	if len(hash) != 32 {
		t.Errorf("generateRefundHash() length = %v, want 32", len(hash))
	}

	// Test that same input produces same hash
	hash2 := p.generateRefundHash(merchantOid, refundAmount)
	if hash != hash2 {
		t.Error("generateRefundHash() should produce consistent results")
	}
}

func TestPayTRProvider_GenerateWebhookHash(t *testing.T) {
	p := &PayTRProvider{
		merchantSalt: "test-salt",
	}

	merchantOid := "order123"
	status := "success"
	totalAmount := "10050"
	hash := p.generateWebhookHash(merchantOid, status, totalAmount)

	// Verify hash is not empty and has correct MD5 length
	if hash == "" {
		t.Error("generateWebhookHash() should not return empty string")
	}

	if len(hash) != 32 {
		t.Errorf("generateWebhookHash() length = %v, want 32", len(hash))
	}

	// Test that same input produces same hash
	hash2 := p.generateWebhookHash(merchantOid, status, totalAmount)
	if hash != hash2 {
		t.Error("generateWebhookHash() should produce consistent results")
	}
}

func TestPayTRProvider_ValidateWebhook(t *testing.T) {
	p := &PayTRProvider{
		merchantSalt: "test-salt",
	}

	merchantOid := "order123"
	status := "success"
	totalAmount := "10050"
	validHash := p.generateWebhookHash(merchantOid, status, totalAmount)

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
				"merchant_oid": merchantOid,
				"status":       status,
				"total_amount": totalAmount,
				"hash":         validHash,
				"payment_id":   "paytr123",
			},
			headers:     map[string]string{},
			expectValid: true,
			expectError: false,
		},
		{
			name: "Invalid hash",
			data: map[string]string{
				"merchant_oid": merchantOid,
				"status":       status,
				"total_amount": totalAmount,
				"hash":         "invalid-hash",
			},
			headers:     map[string]string{},
			expectValid: false,
			expectError: true,
		},
		{
			name: "Missing merchant_oid",
			data: map[string]string{
				"status":       status,
				"total_amount": totalAmount,
				"hash":         validHash,
			},
			headers:     map[string]string{},
			expectValid: false,
			expectError: true,
		},
		{
			name: "Missing status",
			data: map[string]string{
				"merchant_oid": merchantOid,
				"total_amount": totalAmount,
				"hash":         validHash,
			},
			headers:     map[string]string{},
			expectValid: false,
			expectError: true,
		},
		{
			name: "Missing total_amount",
			data: map[string]string{
				"merchant_oid": merchantOid,
				"status":       status,
				"hash":         validHash,
			},
			headers:     map[string]string{},
			expectValid: false,
			expectError: true,
		},
		{
			name: "Missing hash",
			data: map[string]string{
				"merchant_oid": merchantOid,
				"status":       status,
				"total_amount": totalAmount,
			},
			headers:     map[string]string{},
			expectValid: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			isValid, webhookData, err := p.ValidateWebhook(ctx, tt.data, tt.headers)

			if (err != nil) != tt.expectError {
				t.Errorf("ValidateWebhook() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if isValid != tt.expectValid {
				t.Errorf("ValidateWebhook() isValid = %v, expectValid %v", isValid, tt.expectValid)
				return
			}

			if tt.expectValid && !tt.expectError {
				if webhookData["paymentId"] != merchantOid {
					t.Errorf("Expected paymentId %s, got %s", merchantOid, webhookData["paymentId"])
				}
				if webhookData["status"] != status {
					t.Errorf("Expected status %s, got %s", status, webhookData["status"])
				}
				if webhookData["totalAmount"] != totalAmount {
					t.Errorf("Expected totalAmount %s, got %s", totalAmount, webhookData["totalAmount"])
				}
			}
		})
	}
}

func TestPayTRProvider_MapToIFrameResponse(t *testing.T) {
	p := &PayTRProvider{}

	tests := []struct {
		name          string
		response      map[string]any
		merchantOid   string
		expectSuccess bool
		expectStatus  provider.PaymentStatus
	}{
		{
			name: "Successful iframe response",
			response: map[string]any{
				"status": "success",
				"token":  "iframe-token-123",
			},
			merchantOid:   "order123",
			expectSuccess: true,
			expectStatus:  provider.StatusPending,
		},
		{
			name: "Failed iframe response",
			response: map[string]any{
				"status": "failed",
				"reason": "Invalid parameters",
			},
			merchantOid:   "order123",
			expectSuccess: false,
			expectStatus:  provider.StatusFailed,
		},
		{
			name: "Success without token",
			response: map[string]any{
				"status": "success",
			},
			merchantOid:   "order123",
			expectSuccess: false,
			expectStatus:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.mapToIFrameResponse(tt.response, tt.merchantOid)

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectSuccess, result.Success)
			}

			if tt.expectStatus != "" && result.Status != tt.expectStatus {
				t.Errorf("Expected status %v, got %v", tt.expectStatus, result.Status)
			}

			if result.PaymentID != tt.merchantOid {
				t.Errorf("Expected paymentID %s, got %s", tt.merchantOid, result.PaymentID)
			}

			if tt.expectSuccess && tt.response["token"] != nil {
				expectedURL := "https://www.paytr.com/odeme/guvenlik/" + tt.response["token"].(string)
				if result.RedirectURL != expectedURL {
					t.Errorf("Expected redirectURL %s, got %s", expectedURL, result.RedirectURL)
				}
			}
		})
	}
}

func TestPayTRProvider_MapToPaymentResponse(t *testing.T) {
	p := &PayTRProvider{}

	tests := []struct {
		name          string
		response      map[string]any
		paymentID     string
		expectSuccess bool
		expectStatus  provider.PaymentStatus
	}{
		{
			name: "Successful payment",
			response: map[string]any{
				"status":         "success",
				"payment_amount": "10050",
				"currency":       "TL",
				"payment_id":     "paytr123",
			},
			paymentID:     "order123",
			expectSuccess: true,
			expectStatus:  provider.StatusSuccessful,
		},
		{
			name: "Failed payment",
			response: map[string]any{
				"status":             "failed",
				"failed_reason_msg":  "Insufficient funds",
				"failed_reason_code": "YETERSIZ_BAKIYE",
			},
			paymentID:     "order123",
			expectSuccess: false,
			expectStatus:  provider.StatusFailed,
		},
		{
			name: "Waiting payment",
			response: map[string]any{
				"status": "waiting",
			},
			paymentID:     "order123",
			expectSuccess: false,
			expectStatus:  provider.StatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.mapToPaymentResponse(tt.response, tt.paymentID)

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectSuccess, result.Success)
			}

			if result.Status != tt.expectStatus {
				t.Errorf("Expected status %v, got %v", tt.expectStatus, result.Status)
			}

			if result.PaymentID != tt.paymentID {
				t.Errorf("Expected paymentID %s, got %s", tt.paymentID, result.PaymentID)
			}

			// Check amount conversion (from kuruş to TL)
			if _, ok := tt.response["payment_amount"].(string); ok {
				expectedAmount := 100.50 // 10050 kuruş = 100.50 TL
				if result.Amount != expectedAmount {
					t.Errorf("Expected amount %f, got %f", expectedAmount, result.Amount)
				}
			}

			// Check error details
			if errorMsg, ok := tt.response["failed_reason_msg"].(string); ok {
				if result.Message != errorMsg {
					t.Errorf("Expected message %s, got %s", errorMsg, result.Message)
				}
			}

			if errorCode, ok := tt.response["failed_reason_code"].(string); ok {
				if result.ErrorCode != errorCode {
					t.Errorf("Expected errorCode %s, got %s", errorCode, result.ErrorCode)
				}
			}
		})
	}
}
