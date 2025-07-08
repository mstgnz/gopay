package papara

import (
	"strings"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

func TestPaparaProvider_Initialize(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]string
		wantErr bool
	}{
		{
			name: "valid config",
			config: map[string]string{
				"apiKey":      "test-api-key",
				"environment": "sandbox",
			},
			wantErr: false,
		},
		{
			name: "missing api key",
			config: map[string]string{
				"environment": "sandbox",
			},
			wantErr: true,
		},
		{
			name: "production environment",
			config: map[string]string{
				"apiKey":      "test-api-key",
				"environment": "production",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider().(*PaparaProvider)
			err := p.Initialize(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("PaparaProvider.Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if p.apiKey != tt.config["apiKey"] {
					t.Errorf("Expected apiKey %v, got %v", tt.config["apiKey"], p.apiKey)
				}

				expectedProduction := tt.config["environment"] == "production"
				if p.isProduction != expectedProduction {
					t.Errorf("Expected isProduction %v, got %v", expectedProduction, p.isProduction)
				}

				expectedURL := apiSandboxURL
				if expectedProduction {
					expectedURL = apiProductionURL
				}
				if p.baseURL != expectedURL {
					t.Errorf("Expected baseURL %v, got %v", expectedURL, p.baseURL)
				}
			}
		})
	}
}

func TestPaparaProvider_validatePaymentRequest(t *testing.T) {
	p := &PaparaProvider{}

	tests := []struct {
		name    string
		request provider.PaymentRequest
		is3D    bool
		wantErr bool
	}{
		{
			name: "valid request",
			request: provider.PaymentRequest{
				Amount:   100.0,
				Currency: "TRY",
				Customer: provider.Customer{
					Email: "test@example.com",
				},
			},
			is3D:    false,
			wantErr: false,
		},
		{
			name: "valid 3D request",
			request: provider.PaymentRequest{
				Amount:      100.0,
				Currency:    "TRY",
				CallbackURL: "https://example.com/callback",
				Customer: provider.Customer{
					Email: "test@example.com",
				},
			},
			is3D:    true,
			wantErr: false,
		},
		{
			name: "invalid amount",
			request: provider.PaymentRequest{
				Amount:   0,
				Currency: "TRY",
				Customer: provider.Customer{
					Email: "test@example.com",
				},
			},
			is3D:    false,
			wantErr: true,
		},
		{
			name: "missing currency",
			request: provider.PaymentRequest{
				Amount: 100.0,
				Customer: provider.Customer{
					Email: "test@example.com",
				},
			},
			is3D:    false,
			wantErr: true,
		},
		{
			name: "missing email",
			request: provider.PaymentRequest{
				Amount:   100.0,
				Currency: "TRY",
				Customer: provider.Customer{},
			},
			is3D:    false,
			wantErr: true,
		},
		{
			name: "3D missing callback URL",
			request: provider.PaymentRequest{
				Amount:   100.0,
				Currency: "TRY",
				Customer: provider.Customer{
					Email: "test@example.com",
				},
			},
			is3D:    true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.validatePaymentRequest(tt.request, tt.is3D)
			if (err != nil) != tt.wantErr {
				t.Errorf("PaparaProvider.validatePaymentRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPaparaProvider_mapToPaparaRequest(t *testing.T) {
	p := &PaparaProvider{
		gopayBaseURL: "http://localhost:9999",
	}

	request := provider.PaymentRequest{
		Amount:         100.50,
		Currency:       "TRY",
		ReferenceID:    "test-ref-123",
		Description:    "Test payment",
		ConversationID: "conv-123",
		CallbackURL:    "https://example.com/callback",
	}

	result := p.mapToPaparaRequest(request, true)

	if result["amount"] != request.Amount {
		t.Errorf("Expected amount %v, got %v", request.Amount, result["amount"])
	}

	if result["currency"] != request.Currency {
		t.Errorf("Expected currency %v, got %v", request.Currency, result["currency"])
	}

	if result["referenceId"] != request.ReferenceID {
		t.Errorf("Expected referenceId %v, got %v", request.ReferenceID, result["referenceId"])
	}

	if result["orderDescription"] != request.Description {
		t.Errorf("Expected orderDescription %v, got %v", request.Description, result["orderDescription"])
	}

	if result["conversationId"] != request.ConversationID {
		t.Errorf("Expected conversationId %v, got %v", request.ConversationID, result["conversationId"])
	}

	expectedNotificationURL := "http://localhost:9999/v1/webhooks/papara"
	if result["notificationUrl"] != expectedNotificationURL {
		t.Errorf("Expected notificationUrl %v, got %v", expectedNotificationURL, result["notificationUrl"])
	}

	if result["redirectUrl"] != request.CallbackURL {
		t.Errorf("Expected redirectUrl %v, got %v", request.CallbackURL, result["redirectUrl"])
	}
}

func TestPaparaProvider_mapToPaymentResponse(t *testing.T) {
	p := &PaparaProvider{}

	tests := []struct {
		name           string
		paparaResp     PaparaResponse
		expectedStatus provider.PaymentStatus
		expectedError  bool
	}{
		{
			name: "successful payment",
			paparaResp: PaparaResponse{
				Succeeded: true,
				Data: PaparaData{
					ID:       "payment-123",
					Amount:   100.50,
					Currency: "TRY",
					Status:   statusCompleted,
				},
			},
			expectedStatus: provider.StatusSuccessful,
			expectedError:  false,
		},
		{
			name: "pending payment",
			paparaResp: PaparaResponse{
				Succeeded: true,
				Data: PaparaData{
					ID:       "payment-123",
					Amount:   100.50,
					Currency: "TRY",
					Status:   statusPending,
				},
			},
			expectedStatus: provider.StatusPending,
			expectedError:  false,
		},
		{
			name: "failed payment",
			paparaResp: PaparaResponse{
				Succeeded: false,
				Error: PaparaError{
					Code:    "INSUFFICIENT_FUNDS",
					Message: "Insufficient funds",
				},
			},
			expectedStatus: provider.StatusFailed,
			expectedError:  true,
		},
		{
			name: "refunded payment",
			paparaResp: PaparaResponse{
				Succeeded: true,
				Data: PaparaData{
					ID:       "payment-123",
					Amount:   100.50,
					Currency: "TRY",
					Status:   statusRefunded,
				},
			},
			expectedStatus: provider.StatusRefunded,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.mapToPaymentResponse(tt.paparaResp)

			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, result.Status)
			}

			if result.Success == tt.expectedError {
				t.Errorf("Expected success %v, got %v", !tt.expectedError, result.Success)
			}

			if tt.paparaResp.Succeeded {
				if result.PaymentID != tt.paparaResp.Data.ID {
					t.Errorf("Expected paymentID %v, got %v", tt.paparaResp.Data.ID, result.PaymentID)
				}

				if result.Amount != tt.paparaResp.Data.Amount {
					t.Errorf("Expected amount %v, got %v", tt.paparaResp.Data.Amount, result.Amount)
				}

				if result.Currency != tt.paparaResp.Data.Currency {
					t.Errorf("Expected currency %v, got %v", tt.paparaResp.Data.Currency, result.Currency)
				}
			} else {
				if result.ErrorCode != tt.paparaResp.Error.Code {
					t.Errorf("Expected errorCode %v, got %v", tt.paparaResp.Error.Code, result.ErrorCode)
				}

				if result.Message != tt.paparaResp.Error.Message {
					t.Errorf("Expected message %v, got %v", tt.paparaResp.Error.Message, result.Message)
				}
			}
		})
	}
}

func TestPaparaProvider_generateWebhookSignature(t *testing.T) {
	p := &PaparaProvider{
		apiKey: "test-api-key",
	}

	payload := `{"paymentId":"123","status":"COMPLETED"}`
	signature := p.generateWebhookSignature(payload)

	if signature == "" {
		t.Error("Expected non-empty signature")
	}

	// Test that the same payload generates the same signature
	signature2 := p.generateWebhookSignature(payload)
	if signature != signature2 {
		t.Error("Expected same signature for same payload")
	}

	// Test that different payload generates different signature
	differentPayload := `{"paymentId":"456","status":"FAILED"}`
	differentSignature := p.generateWebhookSignature(differentPayload)
	if signature == differentSignature {
		t.Error("Expected different signature for different payload")
	}
}

func TestPaparaProvider_GetRequiredConfig(t *testing.T) {
	provider := NewProvider().(*PaparaProvider)

	tests := []struct {
		name        string
		environment string
		expected    int
	}{
		{"sandbox environment", "sandbox", 2},
		{"production environment", "production", 2},
		{"test environment", "test", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetRequiredConfig(tt.environment)
			if len(result) != tt.expected {
				t.Errorf("GetRequiredConfig() returned %d fields, want %d", len(result), tt.expected)
			}

			// Check required fields
			expectedFields := []string{"apiKey", "environment"}
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

func TestPaparaProvider_ValidateConfig(t *testing.T) {
	provider := NewProvider().(*PaparaProvider)

	tests := []struct {
		name        string
		config      map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid sandbox config",
			config: map[string]string{
				"apiKey":      "12345678-1234-1234-1234-123456789012",
				"environment": "sandbox",
			},
			expectError: false,
		},
		{
			name: "valid production config",
			config: map[string]string{
				"apiKey":      "87654321-4321-4321-4321-210987654321",
				"environment": "production",
			},
			expectError: false,
		},
		{
			name: "missing apiKey",
			config: map[string]string{
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'apiKey' is missing",
		},
		{
			name: "missing environment",
			config: map[string]string{
				"apiKey": "12345678-1234-1234-1234-123456789012",
			},
			expectError: true,
			errorMsg:    "required field 'environment' is missing",
		},
		{
			name: "empty apiKey",
			config: map[string]string{
				"apiKey":      "",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "required field 'apiKey' cannot be empty",
		},
		{
			name: "invalid environment",
			config: map[string]string{
				"apiKey":      "12345678-1234-1234-1234-123456789012",
				"environment": "invalid_env",
			},
			expectError: true,
			errorMsg:    "environment must be one of",
		},
		{
			name: "apiKey too short",
			config: map[string]string{
				"apiKey":      "short-api-key",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must be at least 32 characters",
		},
		{
			name: "apiKey too long",
			config: map[string]string{
				"apiKey":      "12345678-1234-1234-1234-123456789012-extra-long-part",
				"environment": "sandbox",
			},
			expectError: true,
			errorMsg:    "must not exceed 50 characters",
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
