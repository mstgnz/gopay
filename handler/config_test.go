package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

func TestNewConfigHandler(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()

	handler := NewConfigHandler(providerConfig, paymentService, validate)

	if handler == nil {
		t.Fatal("NewConfigHandler should not return nil")
	}

	if handler.providerConfig == nil {
		t.Error("Handler should store provider config")
	}

	if handler.paymentService == nil {
		t.Error("Handler should store payment service")
	}

	if handler.validate == nil {
		t.Error("Handler should store validator")
	}
}

func TestConfigHandler_SetEnv_MissingTenantID(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	requestBody := map[string]string{
		"IYZICO_API_KEY":    "test-api-key",
		"IYZICO_SECRET_KEY": "test-secret-key",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/config/env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Not setting X-Tenant-ID header

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if success, ok := response["success"].(bool); !ok || success {
		t.Error("Response should have success=false")
	}

	expectedMessage := "X-Tenant-ID header is required"
	if message, ok := response["message"].(string); !ok || message != expectedMessage {
		t.Errorf("Expected error message '%s', got '%s'", expectedMessage, message)
	}
}

func TestConfigHandler_SetEnv_InvalidJSON(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("POST", "/config/env", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "TEST")

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if success, ok := response["success"].(bool); !ok || success {
		t.Error("Response should have success=false")
	}
}

func TestConfigHandler_SetEnv_EmptyConfig(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	requestBody := map[string]string{}
	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/config/env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "TEST")

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if success, ok := response["success"].(bool); !ok || success {
		t.Error("Response should have success=false")
	}
}

func TestConfigHandler_SetEnv_ValidIyzicoConfig(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	requestBody := map[string]string{
		"IYZICO_API_KEY":     "test-api-key",
		"IYZICO_SECRET_KEY":  "test-secret-key",
		"IYZICO_ENVIRONMENT": "sandbox",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/config/env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "TEST")

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	// Provider registration will fail in test environment since providers aren't registered
	// This is expected behavior - config is saved but provider registration fails
	if w.Code != 500 {
		t.Errorf("Expected status 500 (provider not registered), got %d. Response: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if success, ok := response["success"].(bool); ok && success {
		t.Error("Response should have success=false due to provider registration failure")
	}

	// Since provider registration fails, we don't expect data field in error response
	if _, ok := response["data"]; ok {
		t.Error("Error response should not contain data field")
	}

	// Check error message contains provider registration failure
	if message, ok := response["message"].(string); ok {
		if !strings.Contains(strings.ToLower(message), "provider") {
			t.Error("Error message should mention provider registration failure")
		}
	}
}

func TestConfigHandler_SetEnv_ValidOzanpayConfig(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	requestBody := map[string]string{
		"OZANPAY_API_KEY":     "test-api-key",
		"OZANPAY_SECRET_KEY":  "test-secret-key",
		"OZANPAY_MERCHANT_ID": "test-merchant",
		"OZANPAY_ENVIRONMENT": "sandbox",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/config/env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "TEST")

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	if w.Code != 500 {
		t.Errorf("Expected status 500 (provider not registered), got %d. Response: %s", w.Code, w.Body.String())
	}
}

func TestConfigHandler_SetEnv_MultipleProviders(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	requestBody := map[string]string{
		"IYZICO_API_KEY":      "test-api-key",
		"IYZICO_SECRET_KEY":   "test-secret-key",
		"OZANPAY_API_KEY":     "test-api-key2",
		"OZANPAY_SECRET_KEY":  "test-secret-key2",
		"OZANPAY_MERCHANT_ID": "test-merchant2",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/config/env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "TEST")

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	if w.Code != 500 {
		t.Errorf("Expected status 500 (provider not registered), got %d. Response: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	// Check that it's a provider registration error
	if message, ok := response["message"].(string); ok {
		if !strings.Contains(strings.ToLower(message), "provider") {
			t.Error("Error message should mention provider registration failure")
		}
	}
}

func TestConfigHandler_GetTenantConfig_MissingTenantID(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("GET", "/config/tenant?provider=iyzico", nil)
	// Not setting X-Tenant-ID header

	w := httptest.NewRecorder()
	handler.GetTenantConfig(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestConfigHandler_GetTenantConfig_MissingProvider(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("GET", "/config/tenant", nil)
	req.Header.Set("X-Tenant-ID", "TEST")

	w := httptest.NewRecorder()
	handler.GetTenantConfig(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestConfigHandler_DeleteTenantConfig_MissingTenantID(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("DELETE", "/config/tenant?provider=iyzico", nil)
	// Not setting X-Tenant-ID header

	w := httptest.NewRecorder()
	handler.DeleteTenantConfig(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestConfigHandler_GetStats(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("GET", "/config/stats", nil)
	w := httptest.NewRecorder()
	handler.GetStats(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if success, ok := response["success"].(bool); !ok || !success {
		t.Error("Response should have success=true")
	}

	if data, ok := response["data"].(map[string]any); ok {
		// Check for basic stats fields
		if _, exists := data["memory_configs"]; !exists {
			t.Log("Response doesn't contain memory_configs, but that's okay")
		}
		if _, exists := data["base_url"]; !exists {
			t.Log("Response doesn't contain base_url, but that's okay")
		}
	} else {
		t.Error("Response should contain data field")
	}
}

func TestConfigHandler_HTTPMethods(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	tests := []struct {
		name        string
		method      string
		path        string
		handler     func(w http.ResponseWriter, r *http.Request)
		needsBody   bool
		needsTenant bool
	}{
		{
			name:        "set env",
			method:      "POST",
			path:        "/config/env",
			handler:     handler.SetEnv,
			needsBody:   true,
			needsTenant: true,
		},
		{
			name:        "get tenant config",
			method:      "GET",
			path:        "/config/tenant?provider=iyzico",
			handler:     handler.GetTenantConfig,
			needsTenant: true,
		},
		{
			name:        "delete tenant config",
			method:      "DELETE",
			path:        "/config/tenant?provider=iyzico",
			handler:     handler.DeleteTenantConfig,
			needsTenant: true,
		},
		{
			name:    "get stats",
			method:  "GET",
			path:    "/config/stats",
			handler: handler.GetStats,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.needsBody {
				body = []byte(`{"IYZICO_API_KEY":"test","IYZICO_SECRET_KEY":"test"}`)
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			if tt.needsTenant {
				req.Header.Set("X-Tenant-ID", "TEST")
			}
			if tt.needsBody {
				req.Header.Set("Content-Type", "application/json")
			}

			w := httptest.NewRecorder()
			tt.handler(w, req)

			// Should not cause server errors (5xx) except for expected cases
			if w.Code >= 500 && tt.name != "set env" { // SetEnv might fail due to provider registration
				t.Errorf("Handler %s should not cause server error, got %d", tt.name, w.Code)
			}
		})
	}
}

func BenchmarkConfigHandler_SetEnv(b *testing.B) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	body := []byte(`{"IYZICO_API_KEY":"test","IYZICO_SECRET_KEY":"test"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/config/env", bytes.NewReader(body))
		req.Header.Set("X-Tenant-ID", "TEST")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.SetEnv(w, req)
	}
}

// Test Paycell configuration
func TestConfigHandler_SetEnv_ValidPaycellConfig(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	body := `{
		"PAYCELL_USERNAME": "testuser",
		"PAYCELL_PASSWORD": "testpass",
		"PAYCELL_MERCHANT_ID": "12345",
		"PAYCELL_TERMINAL_ID": "67890",
		"PAYCELL_ENVIRONMENT": "sandbox"
	}`

	req := httptest.NewRequest("POST", "/config/set", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "TEST")
	w := httptest.NewRecorder()

	handler.SetEnv(w, req)

	if w.Code != 500 { // Expects 500 due to provider registration failure
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	var response map[string]any
	json.NewDecoder(w.Body).Decode(&response)

	if !strings.Contains(response["message"].(string), "Failed to register Paycell provider") {
		t.Errorf("Expected Paycell provider registration error, got: %v", response["message"])
	}
}

// Test multiple providers in one request
func TestConfigHandler_SetEnv_MultipleProvidersWithPaycell(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	body := `{
		"IYZICO_API_KEY": "test_iyzico_key",
		"IYZICO_SECRET_KEY": "test_iyzico_secret",
		"PAYCELL_USERNAME": "testuser",
		"PAYCELL_PASSWORD": "testpass"
	}`

	req := httptest.NewRequest("POST", "/config/set", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "TEST")
	w := httptest.NewRecorder()

	handler.SetEnv(w, req)

	if w.Code != 500 { // Expects 500 due to provider registration failure
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// Test Paycell with partial configuration
func TestConfigHandler_SetEnv_PaycellPartialConfig(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	body := `{
		"PAYCELL_USERNAME": "testuser",
		"PAYCELL_PASSWORD": "testpass"
	}`

	req := httptest.NewRequest("POST", "/config/set", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "TEST")
	w := httptest.NewRecorder()

	handler.SetEnv(w, req)

	// With username and password, it should trigger Paycell configuration but then fail at provider registration
	if w.Code != 500 { // Expects 500 due to provider registration failure
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// Test GetTenantConfig with sensitive value masking
func TestConfigHandler_GetTenantConfig_SensitiveValueMasking(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	// First set a valid iyzico config with sensitive values
	testConfig := map[string]string{
		"apiKey":      "verylongapikey12345", // > 8 chars, should be masked
		"secretKey":   "short",               // <= 8 chars, should be ****
		"password":    "mypassword123",       // > 8 chars, should be masked (last 4: d123)
		"environment": "sandbox",             // not sensitive, should be visible
	}

	// Manually set config to test masking using a valid provider
	err := providerConfig.SetTenantConfig("TEST", "iyzico", testConfig)
	if err != nil {
		t.Fatalf("Failed to set test config: %v", err)
	}

	req := httptest.NewRequest("GET", "/config/tenant?provider=iyzico", nil)
	req.Header.Set("X-Tenant-ID", "TEST")
	w := httptest.NewRecorder()

	handler.GetTenantConfig(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]any
	json.NewDecoder(w.Body).Decode(&response)

	data := response["data"].(map[string]any)
	config := data["config"].(map[string]any)

	// Check masking for long sensitive values
	if config["apiKey"] != "very****2345" {
		t.Errorf("Expected masked apiKey 'very****2345', got '%s'", config["apiKey"])
	}

	// Check masking for short sensitive values
	if config["secretKey"] != "****" {
		t.Errorf("Expected masked secretKey '****', got '%s'", config["secretKey"])
	}

	// Check masking for password (implementation takes last 4 chars)
	if config["password"] != "mypa****d123" {
		t.Errorf("Expected masked password 'mypa****d123', got '%s'", config["password"])
	}

	// Check non-sensitive value is not masked
	if config["environment"] != "sandbox" {
		t.Errorf("Expected environment 'sandbox', got '%s'", config["environment"])
	}
}

// Test GetStats error handling
func TestConfigHandler_GetStats_ErrorHandling(t *testing.T) {
	// Create a config that will cause an error when getting stats
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	// Since we can't easily mock the error in NewProviderConfig,
	// we'll test the normal flow and check that it doesn't error
	req := httptest.NewRequest("GET", "/config/stats", nil)
	w := httptest.NewRecorder()

	handler.GetStats(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]any
	json.NewDecoder(w.Body).Decode(&response)

	if response["success"] != true {
		t.Errorf("Expected success=true, got: %v", response["success"])
	}
}

// Test DeleteTenantConfig error handling
func TestConfigHandler_DeleteTenantConfig_ErrorHandling(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("DELETE", "/config/tenant?provider=nonexistent", nil)
	req.Header.Set("X-Tenant-ID", "NONEXISTENT")
	w := httptest.NewRecorder()

	handler.DeleteTenantConfig(w, req)

	if w.Code != 404 {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	var response map[string]any
	json.NewDecoder(w.Body).Decode(&response)

	if !strings.Contains(response["message"].(string), "Failed to delete configuration") {
		t.Errorf("Expected delete error message, got: %v", response["message"])
	}
}

// Test edge cases for SetEnv
func TestConfigHandler_SetEnv_EdgeCases(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	// Test with environment variables set but empty values
	t.Run("empty environment values", func(t *testing.T) {
		body := `{
			"IYZICO_API_KEY": "test_key",
			"IYZICO_SECRET_KEY": "test_secret",
			"IYZICO_ENVIRONMENT": ""
		}`

		req := httptest.NewRequest("POST", "/config/set", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "TEST")
		w := httptest.NewRecorder()

		handler.SetEnv(w, req)

		if w.Code != 500 { // Expects 500 due to provider registration failure
			t.Errorf("Expected status 500, got %d", w.Code)
		}
	})

	// Test with OzanPay environment variable
	t.Run("ozanpay with environment", func(t *testing.T) {
		body := `{
			"OZANPAY_API_KEY": "test_key",
			"OZANPAY_SECRET_KEY": "test_secret",
			"OZANPAY_MERCHANT_ID": "test_merchant",
			"OZANPAY_ENVIRONMENT": "production"
		}`

		req := httptest.NewRequest("POST", "/config/set", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "TEST")
		w := httptest.NewRecorder()

		handler.SetEnv(w, req)

		if w.Code != 500 { // Expects 500 due to provider registration failure
			t.Errorf("Expected status 500, got %d", w.Code)
		}
	})

	// Test with Paycell environment variable
	t.Run("paycell with environment", func(t *testing.T) {
		body := `{
			"PAYCELL_USERNAME": "user",
			"PAYCELL_PASSWORD": "pass",
			"PAYCELL_MERCHANT_ID": "merchant",
			"PAYCELL_TERMINAL_ID": "terminal",
			"PAYCELL_ENVIRONMENT": "production"
		}`

		req := httptest.NewRequest("POST", "/config/set", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "TEST")
		w := httptest.NewRecorder()

		handler.SetEnv(w, req)

		if w.Code != 500 { // Expects 500 due to provider registration failure
			t.Errorf("Expected status 500, got %d", w.Code)
		}
	})
}
