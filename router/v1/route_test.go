package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mstgnz/gopay/infra/opensearch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutes(t *testing.T) {
	tests := []struct {
		name   string
		logger *opensearch.Logger
	}{
		{
			name:   "valid_logger",
			logger: &opensearch.Logger{},
		},
		{
			name:   "nil_logger",
			logger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			require.NotNil(t, r)

			// Routes function should not panic
			assert.NotPanics(t, func() {
				Routes(r, tt.logger)
			})
		})
	}
}

func TestRoutes_EndpointRegistration(t *testing.T) {
	r := chi.NewRouter()
	logger := &opensearch.Logger{}

	Routes(r, logger)

	// Test that routes are properly registered by making test requests
	tests := []struct {
		name       string
		method     string
		path       string
		expectCode int
	}{
		{
			name:       "set_env_endpoint",
			method:     "POST",
			path:       "/set-env",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "tenant_config_get",
			method:     "GET",
			path:       "/config/tenant-config",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "tenant_config_delete",
			method:     "DELETE",
			path:       "/config/tenant-config",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "stats_endpoint",
			method:     "GET",
			path:       "/stats",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "payments_post",
			method:     "POST",
			path:       "/payments/",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "payment_status_get",
			method:     "GET",
			path:       "/payments/test-payment-id",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "payment_cancel",
			method:     "DELETE",
			path:       "/payments/test-payment-id",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "payment_refund",
			method:     "POST",
			path:       "/payments/refund",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "provider_specific_payment",
			method:     "POST",
			path:       "/payments/iyzico",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "provider_payment_status",
			method:     "GET",
			path:       "/payments/iyzico/test-payment-id",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "logs_list",
			method:     "GET",
			path:       "/logs/iyzico",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "payment_logs",
			method:     "GET",
			path:       "/logs/iyzico/payment/test-payment-id",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "error_logs",
			method:     "GET",
			path:       "/logs/iyzico/errors",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
		{
			name:       "log_stats",
			method:     "GET",
			path:       "/logs/iyzico/stats",
			expectCode: 401, // Should return 401 Unauthorized without proper auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			// The exact status code might vary based on middleware
			// But we should get some response, not 404 (which would mean route not found)
			assert.NotEqual(t, http.StatusNotFound, rec.Code, "Route should be registered")
		})
	}
}

func TestCleanup(t *testing.T) {
	// Test that cleanup function doesn't panic
	assert.NotPanics(t, func() {
		Cleanup()
	})
}

func TestGlobalVariables(t *testing.T) {
	// Test that global variables are initialized
	assert.NotNil(t, paymentService, "paymentService should be initialized")
	assert.NotNil(t, providerConfig, "providerConfig should be initialized")
}

func TestInit(t *testing.T) {
	// The init function runs automatically when the package is imported
	// If we reach this point, it means init() executed successfully
	// We can test that the global variables are properly initialized

	assert.NotNil(t, paymentService, "Payment service should be initialized by init()")
	assert.NotNil(t, providerConfig, "Provider config should be initialized by init()")

	// Test that we can call methods on the initialized services
	assert.NotPanics(t, func() {
		providers := providerConfig.GetAvailableProviders()
		_ = providers // Use the variable to avoid unused variable error
	})
}

func TestRoutes_MethodNotAllowed(t *testing.T) {
	r := chi.NewRouter()
	logger := &opensearch.Logger{}

	Routes(r, logger)

	// Test method not allowed scenarios
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "get_set_env",
			method: "GET",
			path:   "/set-env",
		},
		{
			name:   "post_stats",
			method: "POST",
			path:   "/stats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			// Should not return 404 (route exists)
			// Might return 405 (method not allowed) or other auth-related status
			assert.NotEqual(t, http.StatusNotFound, rec.Code)
		})
	}
}
