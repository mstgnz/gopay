package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mstgnz/gopay/infra/opensearch"
)

// Mock OpenSearch Logger for testing
type mockOpenSearchLogger struct {
	searchLogsFunc         func(ctx context.Context, tenantID, provider string, query map[string]any) ([]opensearch.PaymentLog, error)
	getPaymentLogsFunc     func(ctx context.Context, tenantID, provider, paymentID string) ([]opensearch.PaymentLog, error)
	getRecentErrorLogsFunc func(ctx context.Context, tenantID, provider string, hours int) ([]opensearch.PaymentLog, error)
	getProviderStatsFunc   func(ctx context.Context, tenantID, provider string, hours int) (map[string]any, error)
}

func (m *mockOpenSearchLogger) SearchLogs(ctx context.Context, tenantID, provider string, query map[string]any) ([]opensearch.PaymentLog, error) {
	if m.searchLogsFunc != nil {
		return m.searchLogsFunc(ctx, tenantID, provider, query)
	}
	return []opensearch.PaymentLog{
		{
			TenantID: tenantID,
			Provider: provider,
			PaymentInfo: opensearch.PaymentInfo{
				PaymentID: "test-payment-123",
				Status:    "successful",
				Amount:    100.50,
			},
		},
	}, nil
}

func (m *mockOpenSearchLogger) GetPaymentLogs(ctx context.Context, tenantID, provider, paymentID string) ([]opensearch.PaymentLog, error) {
	if m.getPaymentLogsFunc != nil {
		return m.getPaymentLogsFunc(ctx, tenantID, provider, paymentID)
	}
	return []opensearch.PaymentLog{
		{
			TenantID: tenantID,
			Provider: provider,
			PaymentInfo: opensearch.PaymentInfo{
				PaymentID: paymentID,
				Status:    "successful",
				Amount:    100.50,
			},
		},
	}, nil
}

func (m *mockOpenSearchLogger) GetRecentErrorLogs(ctx context.Context, tenantID, provider string, hours int) ([]opensearch.PaymentLog, error) {
	if m.getRecentErrorLogsFunc != nil {
		return m.getRecentErrorLogsFunc(ctx, tenantID, provider, hours)
	}
	return []opensearch.PaymentLog{
		{
			TenantID: tenantID,
			Provider: provider,
			Error: opensearch.ErrorInfo{
				Code:    "PAYMENT_FAILED",
				Message: "Payment processing failed",
			},
		},
	}, nil
}

func (m *mockOpenSearchLogger) GetProviderStats(ctx context.Context, tenantID, provider string, hours int) (map[string]any, error) {
	if m.getProviderStatsFunc != nil {
		return m.getProviderStatsFunc(ctx, tenantID, provider, hours)
	}
	return map[string]any{
		"total_logs":        100,
		"error_count":       5,
		"success_count":     95,
		"avg_response_time": 250.5,
		"period_hours":      hours,
	}, nil
}

// Helper function to create test logger
func newMockLogger() *mockOpenSearchLogger {
	return &mockOpenSearchLogger{}
}

func TestNewLogsHandler(t *testing.T) {
	logger := newMockLogger()
	handler := NewLogsHandler(logger)

	if handler == nil {
		t.Fatal("NewLogsHandler should not return nil")
	}

	if handler.logger != logger {
		t.Error("Handler should store the logger")
	}
}

func TestLogsHandler_ListLogs(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		tenantID       string
		provider       string
		queryParams    string
		expectedStatus int
		mockFunc       func(ctx context.Context, tenantID, provider string, query map[string]any) ([]opensearch.PaymentLog, error)
	}{
		{
			name:           "successful logs listing",
			method:         "GET",
			path:           "/logs/iyzico",
			tenantID:       "APP1",
			provider:       "iyzico",
			expectedStatus: 200,
		},
		{
			name:           "logs with payment ID filter",
			method:         "GET",
			path:           "/logs/iyzico",
			tenantID:       "APP1",
			provider:       "iyzico",
			queryParams:    "paymentId=test-123",
			expectedStatus: 200,
		},
		{
			name:           "logs with status filter",
			method:         "GET",
			path:           "/logs/iyzico",
			tenantID:       "APP1",
			provider:       "iyzico",
			queryParams:    "status=failed",
			expectedStatus: 200,
		},
		{
			name:           "logs with errors only filter",
			method:         "GET",
			path:           "/logs/iyzico",
			tenantID:       "APP1",
			provider:       "iyzico",
			queryParams:    "errorsOnly=true",
			expectedStatus: 200,
		},
		{
			name:           "logs with hours filter",
			method:         "GET",
			path:           "/logs/iyzico",
			tenantID:       "APP1",
			provider:       "iyzico",
			queryParams:    "hours=12",
			expectedStatus: 200,
		},
		{
			name:           "logs with multiple filters",
			method:         "GET",
			path:           "/logs/iyzico",
			tenantID:       "APP1",
			provider:       "iyzico",
			queryParams:    "paymentId=test-123&status=failed&errorsOnly=true&hours=6",
			expectedStatus: 200,
		},
		{
			name:           "missing tenant ID",
			method:         "GET",
			path:           "/logs/iyzico",
			provider:       "iyzico",
			expectedStatus: 400,
		},
		{
			name:           "missing provider",
			method:         "GET",
			path:           "/logs/",
			tenantID:       "APP1",
			expectedStatus: 400,
		},
		{
			name:           "invalid hours parameter",
			method:         "GET",
			path:           "/logs/iyzico",
			tenantID:       "APP1",
			provider:       "iyzico",
			queryParams:    "hours=invalid",
			expectedStatus: 200, // Should fallback to default
		},
		{
			name:           "hours over limit",
			method:         "GET",
			path:           "/logs/iyzico",
			tenantID:       "APP1",
			provider:       "iyzico",
			queryParams:    "hours=200",
			expectedStatus: 200, // Should fallback to default
		},
		{
			name:           "logger error",
			method:         "GET",
			path:           "/logs/iyzico",
			tenantID:       "APP1",
			provider:       "iyzico",
			expectedStatus: 500,
			mockFunc: func(ctx context.Context, tenantID, provider string, query map[string]any) ([]opensearch.PaymentLog, error) {
				return nil, errors.New("opensearch connection failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := &mockOpenSearchLogger{
				searchLogsFunc: tt.mockFunc,
			}
			handler := NewLogsHandler(mockLogger)

			var req *http.Request
			if tt.queryParams != "" {
				req = httptest.NewRequest(tt.method, tt.path+"?"+tt.queryParams, nil)
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()

			// Set up chi context
			rctx := chi.NewRouteContext()
			if tt.provider != "" {
				rctx.URLParams.Add("provider", tt.provider)
			}
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.ListLogs(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Response: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedStatus == 200 {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if !response["success"].(bool) {
					t.Error("Expected success to be true")
				}

				data, ok := response["data"].(map[string]interface{})
				if !ok {
					t.Fatal("Response should contain data field")
				}

				if data["tenantId"] != tt.tenantID {
					t.Errorf("Expected tenantId %s, got %v", tt.tenantID, data["tenantId"])
				}

				if data["provider"] != tt.provider {
					t.Errorf("Expected provider %s, got %v", tt.provider, data["provider"])
				}
			}
		})
	}
}

func TestLogsHandler_GetPaymentLogs(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		tenantID       string
		provider       string
		paymentID      string
		expectedStatus int
		mockFunc       func(ctx context.Context, tenantID, provider, paymentID string) ([]opensearch.PaymentLog, error)
	}{
		{
			name:           "successful payment logs retrieval",
			method:         "GET",
			path:           "/logs/iyzico/payment-123",
			tenantID:       "APP1",
			provider:       "iyzico",
			paymentID:      "payment-123",
			expectedStatus: 200,
		},
		{
			name:           "missing tenant ID",
			method:         "GET",
			path:           "/logs/iyzico/payment-123",
			provider:       "iyzico",
			paymentID:      "payment-123",
			expectedStatus: 400,
		},
		{
			name:           "missing provider",
			method:         "GET",
			path:           "/logs//payment-123",
			tenantID:       "APP1",
			paymentID:      "payment-123",
			expectedStatus: 400,
		},
		{
			name:           "missing payment ID",
			method:         "GET",
			path:           "/logs/iyzico/",
			tenantID:       "APP1",
			provider:       "iyzico",
			expectedStatus: 400,
		},
		{
			name:           "logger error",
			method:         "GET",
			path:           "/logs/iyzico/payment-123",
			tenantID:       "APP1",
			provider:       "iyzico",
			paymentID:      "payment-123",
			expectedStatus: 500,
			mockFunc: func(ctx context.Context, tenantID, provider, paymentID string) ([]opensearch.PaymentLog, error) {
				return nil, errors.New("opensearch query failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := &mockOpenSearchLogger{
				getPaymentLogsFunc: tt.mockFunc,
			}
			handler := NewLogsHandler(mockLogger)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()

			// Set up chi context
			rctx := chi.NewRouteContext()
			if tt.provider != "" {
				rctx.URLParams.Add("provider", tt.provider)
			}
			if tt.paymentID != "" {
				rctx.URLParams.Add("paymentID", tt.paymentID)
			}
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.GetPaymentLogs(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == 200 {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if !response["success"].(bool) {
					t.Error("Expected success to be true")
				}

				data, ok := response["data"].(map[string]interface{})
				if !ok {
					t.Fatal("Response should contain data field")
				}

				if data["paymentId"] != tt.paymentID {
					t.Errorf("Expected paymentId %s, got %v", tt.paymentID, data["paymentId"])
				}
			}
		})
	}
}

func TestLogsHandler_GetErrorLogs(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		tenantID       string
		provider       string
		queryParams    string
		expectedStatus int
		mockFunc       func(ctx context.Context, tenantID, provider string, hours int) ([]opensearch.PaymentLog, error)
	}{
		{
			name:           "successful error logs retrieval",
			method:         "GET",
			path:           "/logs/iyzico/errors",
			tenantID:       "APP1",
			provider:       "iyzico",
			expectedStatus: 200,
		},
		{
			name:           "error logs with custom hours",
			method:         "GET",
			path:           "/logs/iyzico/errors",
			tenantID:       "APP1",
			provider:       "iyzico",
			queryParams:    "hours=48",
			expectedStatus: 200,
		},
		{
			name:           "missing tenant ID",
			method:         "GET",
			path:           "/logs/iyzico/errors",
			provider:       "iyzico",
			expectedStatus: 400,
		},
		{
			name:           "missing provider",
			method:         "GET",
			path:           "/logs//errors",
			tenantID:       "APP1",
			expectedStatus: 400,
		},
		{
			name:           "invalid hours parameter",
			method:         "GET",
			path:           "/logs/iyzico/errors",
			tenantID:       "APP1",
			provider:       "iyzico",
			queryParams:    "hours=invalid",
			expectedStatus: 200, // Should fallback to default
		},
		{
			name:           "logger error",
			method:         "GET",
			path:           "/logs/iyzico/errors",
			tenantID:       "APP1",
			provider:       "iyzico",
			expectedStatus: 500,
			mockFunc: func(ctx context.Context, tenantID, provider string, hours int) ([]opensearch.PaymentLog, error) {
				return nil, errors.New("opensearch connection failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := &mockOpenSearchLogger{
				getRecentErrorLogsFunc: tt.mockFunc,
			}
			handler := NewLogsHandler(mockLogger)

			var req *http.Request
			if tt.queryParams != "" {
				req = httptest.NewRequest(tt.method, tt.path+"?"+tt.queryParams, nil)
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()

			// Set up chi context
			rctx := chi.NewRouteContext()
			if tt.provider != "" {
				rctx.URLParams.Add("provider", tt.provider)
			}
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.GetErrorLogs(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == 200 {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if !response["success"].(bool) {
					t.Error("Expected success to be true")
				}
			}
		})
	}
}

func TestLogsHandler_GetLogStats(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		tenantID       string
		provider       string
		queryParams    string
		expectedStatus int
		mockFunc       func(ctx context.Context, tenantID, provider string, hours int) (map[string]any, error)
	}{
		{
			name:           "successful stats retrieval",
			method:         "GET",
			path:           "/logs/iyzico/stats",
			tenantID:       "APP1",
			provider:       "iyzico",
			expectedStatus: 200,
		},
		{
			name:           "stats with custom hours",
			method:         "GET",
			path:           "/logs/iyzico/stats",
			tenantID:       "APP1",
			provider:       "iyzico",
			queryParams:    "hours=72",
			expectedStatus: 200,
		},
		{
			name:           "missing tenant ID",
			method:         "GET",
			path:           "/logs/iyzico/stats",
			provider:       "iyzico",
			expectedStatus: 400,
		},
		{
			name:           "missing provider",
			method:         "GET",
			path:           "/logs//stats",
			tenantID:       "APP1",
			expectedStatus: 400,
		},
		{
			name:           "logger error",
			method:         "GET",
			path:           "/logs/iyzico/stats",
			tenantID:       "APP1",
			provider:       "iyzico",
			expectedStatus: 500,
			mockFunc: func(ctx context.Context, tenantID, provider string, hours int) (map[string]any, error) {
				return nil, errors.New("stats calculation failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := &mockOpenSearchLogger{
				getProviderStatsFunc: tt.mockFunc,
			}
			handler := NewLogsHandler(mockLogger)

			var req *http.Request
			if tt.queryParams != "" {
				req = httptest.NewRequest(tt.method, tt.path+"?"+tt.queryParams, nil)
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()

			// Set up chi context
			rctx := chi.NewRouteContext()
			if tt.provider != "" {
				rctx.URLParams.Add("provider", tt.provider)
			}
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.GetLogStats(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == 200 {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if !response["success"].(bool) {
					t.Error("Expected success to be true")
				}

				data, ok := response["data"].(map[string]interface{})
				if !ok {
					t.Fatal("Response should contain data field")
				}

				stats, ok := data["stats"].(map[string]interface{})
				if !ok {
					t.Fatal("Response should contain stats field")
				}

				if stats["total_logs"] == nil {
					t.Error("Stats should contain total_logs")
				}
			}
		})
	}
}

func TestLogsHandler_HTTPMethods(t *testing.T) {
	mockLogger := newMockLogger()
	handler := NewLogsHandler(mockLogger)

	tests := []struct {
		name     string
		method   string
		path     string
		handler  func(w http.ResponseWriter, r *http.Request)
		tenantID string
		provider string
	}{
		{
			name:     "list logs",
			method:   "POST", // Testing wrong method
			path:     "/logs/iyzico",
			handler:  handler.ListLogs,
			tenantID: "APP1",
			provider: "iyzico",
		},
		{
			name:     "get payment logs",
			method:   "POST", // Testing wrong method
			path:     "/logs/iyzico/payment-123",
			handler:  handler.GetPaymentLogs,
			tenantID: "APP1",
			provider: "iyzico",
		},
		{
			name:     "get error logs",
			method:   "POST", // Testing wrong method
			path:     "/logs/iyzico/errors",
			handler:  handler.GetErrorLogs,
			tenantID: "APP1",
			provider: "iyzico",
		},
		{
			name:     "get log stats",
			method:   "POST", // Testing wrong method
			path:     "/logs/iyzico/stats",
			handler:  handler.GetLogStats,
			tenantID: "APP1",
			provider: "iyzico",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("X-Tenant-ID", tt.tenantID)

			w := httptest.NewRecorder()

			// Set up chi context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("provider", tt.provider)
			if strings.Contains(tt.path, "payment-123") {
				rctx.URLParams.Add("paymentID", "payment-123")
			}
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// These handlers don't specifically check HTTP methods,
			// they should work with any method
			tt.handler(w, req)

			// Should return 200 since handlers don't validate HTTP methods
			if w.Code != 200 {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

// Benchmark tests
func BenchmarkLogsHandler_ListLogs(b *testing.B) {
	mockLogger := newMockLogger()
	handler := NewLogsHandler(mockLogger)

	req := httptest.NewRequest("GET", "/logs/iyzico", nil)
	req.Header.Set("X-Tenant-ID", "APP1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()

		// Set up chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("provider", "iyzico")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handler.ListLogs(w, req)
	}
}

func BenchmarkLogsHandler_GetPaymentLogs(b *testing.B) {
	mockLogger := newMockLogger()
	handler := NewLogsHandler(mockLogger)

	req := httptest.NewRequest("GET", "/logs/iyzico/payment-123", nil)
	req.Header.Set("X-Tenant-ID", "APP1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()

		// Set up chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("provider", "iyzico")
		rctx.URLParams.Add("paymentID", "payment-123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handler.GetPaymentLogs(w, req)
	}
}
