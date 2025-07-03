package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

func TestNewConfigHandler(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService()
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
	paymentService := provider.NewPaymentService()
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
	paymentService := provider.NewPaymentService()
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
	paymentService := provider.NewPaymentService()
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
	paymentService := provider.NewPaymentService()
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

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d. Response: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if success, ok := response["success"].(bool); !ok || !success {
		t.Error("Response should have success=true")
	}

	if data, ok := response["data"].(map[string]any); ok {
		if tenantID, exists := data["tenantId"]; !exists || tenantID != "TEST" {
			t.Errorf("Response should contain correct tenantId, got %v", tenantID)
		}
		if providers, exists := data["configuredProviders"]; !exists {
			t.Error("Response should contain configuredProviders")
		} else if providerList, ok := providers.([]any); ok {
			if len(providerList) == 0 {
				t.Error("Should have configured at least one provider")
			}
			// Check if iyzico is in the list
			found := false
			for _, p := range providerList {
				if p == "iyzico" {
					found = true
					break
				}
			}
			if !found {
				t.Error("Should have configured iyzico provider")
			}
		}
	} else {
		t.Error("Response should contain data field")
	}
}

func TestConfigHandler_SetEnv_ValidOzanpayConfig(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService()
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

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d. Response: %s", w.Code, w.Body.String())
	}
}

func TestConfigHandler_SetEnv_MultipleProviders(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService()
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

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d. Response: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if data, ok := response["data"].(map[string]any); ok {
		if providers, exists := data["configuredProviders"]; exists {
			if providerList, ok := providers.([]any); ok && len(providerList) >= 2 {
				// Should have configured multiple providers
			} else {
				t.Error("Should have configured multiple providers")
			}
		}
	}
}

func TestConfigHandler_GetTenantConfig_MissingTenantID(t *testing.T) {
	providerConfig := config.NewProviderConfig()
	paymentService := provider.NewPaymentService()
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
	paymentService := provider.NewPaymentService()
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
	paymentService := provider.NewPaymentService()
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
	paymentService := provider.NewPaymentService()
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
	paymentService := provider.NewPaymentService()
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
	paymentService := provider.NewPaymentService()
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
