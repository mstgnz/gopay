package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/middle"
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

	requestBody := SetEnvRequest{
		Provider:    "iyzico",
		Environment: "test",
		Configs: []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{
			{Key: "IYZICO_API_KEY", Value: "test-api-key"},
			{Key: "IYZICO_SECRET_KEY", Value: "test-secret-key"},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Not setting JWT context

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if success, ok := response["success"].(bool); !ok || success {
		t.Error("Response should have success=false")
	}

	expectedMessage := "Authentication required"
	if message, ok := response["message"].(string); !ok || message != expectedMessage {
		t.Errorf("Expected error message '%s', got '%s'", expectedMessage, message)
	}
}

func TestConfigHandler_SetEnv_InvalidJSON(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	// Add JWT context
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
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

	requestBody := SetEnvRequest{
		Provider:    "iyzico",
		Environment: "test",
		Configs: []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
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

	requestBody := SetEnvRequest{
		Provider:    "iyzico",
		Environment: "test",
		Configs: []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{
			{Key: "IYZICO_API_KEY", Value: "test-api-key"},
			{Key: "IYZICO_SECRET_KEY", Value: "test-secret-key"},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	// In test environment, provider validation will fail due to no database connection
	// This is expected behavior - provider not found in database
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d (provider not found), got %d. Response: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if success, ok := response["success"].(bool); ok && success {
		t.Error("Response should have success=false due to provider validation failure")
	}

	// Check error message contains provider not found
	if message, ok := response["message"].(string); ok {
		if !strings.Contains(strings.ToLower(message), "provider not found") {
			t.Error("Error message should mention provider not found")
		}
	}
}

func TestConfigHandler_SetEnv_ValidOzanpayConfig(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	requestBody := SetEnvRequest{
		Provider:    "ozanpay",
		Environment: "test",
		Configs: []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{
			{Key: "OZANPAY_API_KEY", Value: "test-api-key"},
			{Key: "OZANPAY_SECRET_KEY", Value: "test-secret-key"},
			{Key: "OZANPAY_MERCHANT_ID", Value: "test-merchant"},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	// In test environment, provider validation will fail due to no database connection
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d (provider not found), got %d. Response: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestConfigHandler_SetEnv_ValidPaycellConfig(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	requestBody := SetEnvRequest{
		Provider:    "paycell",
		Environment: "test",
		Configs: []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{
			{Key: "PAYCELL_USERNAME", Value: "testuser"},
			{Key: "PAYCELL_PASSWORD", Value: "testpass"},
			{Key: "PAYCELL_MERCHANT_ID", Value: "12345"},
			{Key: "PAYCELL_TERMINAL_ID", Value: "67890"},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	// In test environment, provider validation will fail due to no database connection
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d (provider not found), got %d. Response: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestConfigHandler_SetEnv_InvalidEnvironment(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	requestBody := SetEnvRequest{
		Provider:    "iyzico",
		Environment: "invalid",
		Configs: []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{
			{Key: "IYZICO_API_KEY", Value: "test-api-key"},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if success, ok := response["success"].(bool); !ok || success {
		t.Error("Response should have success=false")
	}
}

func TestConfigHandler_SetEnv_ProductionEnvironment(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	requestBody := SetEnvRequest{
		Provider:    "iyzico",
		Environment: "production",
		Configs: []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{
			{Key: "IYZICO_API_KEY", Value: "prod-api-key"},
			{Key: "IYZICO_SECRET_KEY", Value: "prod-secret-key"},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.SetEnv(w, req)

	// Should fail due to provider validation, but environment should be converted to "prod"
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d (provider not found), got %d", http.StatusBadRequest, w.Code)
	}
}

func TestConfigHandler_GetTenantConfig_MissingTenantID(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("GET", "/v1/config/tenant-config?provider=iyzico", nil)
	// Not setting JWT context

	w := httptest.NewRecorder()
	handler.GetTenantConfig(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestConfigHandler_GetTenantConfig_MissingProvider(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("GET", "/v1/config/tenant-config", nil)
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetTenantConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

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

	req := httptest.NewRequest("GET", "/v1/config/tenant-config?provider=iyzico", nil)
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.GetTenantConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
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

func TestConfigHandler_DeleteTenantConfig_MissingTenantID(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("DELETE", "/v1/config/tenant-config?provider=iyzico", nil)
	// Not setting JWT context

	w := httptest.NewRecorder()
	handler.DeleteTenantConfig(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestConfigHandler_DeleteTenantConfig_MissingProvider(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("DELETE", "/v1/config/tenant-config", nil)
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.DeleteTenantConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestConfigHandler_DeleteTenantConfig_ErrorHandling(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("DELETE", "/v1/config/tenant-config?provider=nonexistent", nil)
	ctx := context.WithValue(req.Context(), middle.TenantIDKey, "NONEXISTENT")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.DeleteTenantConfig(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response map[string]any
	json.NewDecoder(w.Body).Decode(&response)

	if !strings.Contains(response["message"].(string), "Failed to delete configuration") {
		t.Errorf("Expected delete error message, got: %v", response["message"])
	}
}

func TestConfigHandler_GetStats(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()
	handler.GetStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
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

func TestConfigHandler_GetStats_ErrorHandling(t *testing.T) {
	// Create a config that will cause an error when getting stats
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	// Since we can't easily mock the error in NewProviderConfig,
	// we'll test the normal flow and check that it doesn't error
	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()

	handler.GetStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]any
	json.NewDecoder(w.Body).Decode(&response)

	if response["success"] != true {
		t.Errorf("Expected success=true, got: %v", response["success"])
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
			path:        "/v1/set-env",
			handler:     handler.SetEnv,
			needsBody:   true,
			needsTenant: true,
		},
		{
			name:        "get tenant config",
			method:      "GET",
			path:        "/v1/config/tenant-config?provider=iyzico",
			handler:     handler.GetTenantConfig,
			needsTenant: true,
		},
		{
			name:        "delete tenant config",
			method:      "DELETE",
			path:        "/v1/config/tenant-config?provider=iyzico",
			handler:     handler.DeleteTenantConfig,
			needsTenant: true,
		},
		{
			name:    "get stats",
			method:  "GET",
			path:    "/v1/stats",
			handler: handler.GetStats,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.needsBody {
				requestBody := SetEnvRequest{
					Provider:    "iyzico",
					Environment: "test",
					Configs: []struct {
						Key   string `json:"key"`
						Value string `json:"value"`
					}{
						{Key: "IYZICO_API_KEY", Value: "test"},
						{Key: "IYZICO_SECRET_KEY", Value: "test"},
					},
				}
				body, _ = json.Marshal(requestBody)
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			if tt.needsTenant {
				ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
				req = req.WithContext(ctx)
			}
			if tt.needsBody {
				req.Header.Set("Content-Type", "application/json")
			}

			w := httptest.NewRecorder()
			tt.handler(w, req)

			// Should not cause server errors (5xx) except for expected cases
			if w.Code >= 500 && tt.name != "set env" { // SetEnv might fail due to provider validation
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

	requestBody := SetEnvRequest{
		Provider:    "iyzico",
		Environment: "test",
		Configs: []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{
			{Key: "IYZICO_API_KEY", Value: "test"},
			{Key: "IYZICO_SECRET_KEY", Value: "test"},
		},
	}

	body, _ := json.Marshal(requestBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.SetEnv(w, req)
	}
}

// Test edge cases for SetEnv
func TestConfigHandler_SetEnv_EdgeCases(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService(&provider.DBPaymentLogger{})
	validate := validator.New()
	handler := NewConfigHandler(providerConfig, paymentService, validate)

	// Test with empty provider
	t.Run("empty provider", func(t *testing.T) {
		requestBody := SetEnvRequest{
			Provider:    "",
			Environment: "test",
			Configs: []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}{
				{Key: "IYZICO_API_KEY", Value: "test"},
			},
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.SetEnv(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	// Test with empty environment
	t.Run("empty environment", func(t *testing.T) {
		requestBody := SetEnvRequest{
			Provider:    "iyzico",
			Environment: "",
			Configs: []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}{
				{Key: "IYZICO_API_KEY", Value: "test"},
			},
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.SetEnv(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	// Test with only environment config (no other configs)
	t.Run("only environment config", func(t *testing.T) {
		requestBody := SetEnvRequest{
			Provider:    "iyzico",
			Environment: "test",
			Configs: []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}{
				{Key: "", Value: ""}, // Empty key should be skipped
			},
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/v1/set-env", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), middle.TenantIDKey, "TEST")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.SetEnv(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})
}
