package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mstgnz/gopay/infra/opensearch"
)

func TestNewAnalyticsHandler(t *testing.T) {
	// Test with nil logger
	handler := NewAnalyticsHandler(nil)
	if handler == nil {
		t.Fatal("NewAnalyticsHandler should not return nil")
	}

	if handler.logger != nil {
		t.Error("Handler should store nil logger")
	}

	// Test with mock logger (using opensearch.NewLogger with nil client)
	client := &opensearch.Client{}
	logger := opensearch.NewLogger(client)
	handler2 := NewAnalyticsHandler(logger)

	if handler2 == nil {
		t.Fatal("NewAnalyticsHandler should not return nil")
	}

	if handler2.logger != logger {
		t.Error("Handler should store the logger")
	}
}

func TestAnalyticsHandler_GetDashboardStats(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		hoursParam     string
		expectedStatus int
		logger         *opensearch.Logger
	}{
		{
			name:           "successful stats with nil logger (fallback to demo data)",
			hoursParam:     "24",
			expectedStatus: 200,
			logger:         nil,
		},
		{
			name:           "successful stats with tenant and nil logger",
			tenantID:       "APP1",
			hoursParam:     "48",
			expectedStatus: 200,
			logger:         nil,
		},
		{
			name:           "invalid hours parameter - fallback to 24",
			hoursParam:     "abc",
			expectedStatus: 200,
			logger:         nil,
		},
		{
			name:           "hours over limit - fallback to 24",
			hoursParam:     "200",
			expectedStatus: 200,
			logger:         nil,
		},
		{
			name:           "negative hours - fallback to 24",
			hoursParam:     "-5",
			expectedStatus: 200,
			logger:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAnalyticsHandler(tt.logger)

			url := "/analytics/dashboard"
			if tt.hoursParam != "" {
				url += "?hours=" + tt.hoursParam
			}

			req := httptest.NewRequest("GET", url, nil)
			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()
			handler.GetDashboardStats(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Verify response structure
			var response map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
			}

			// Check if response has success field
			if success, ok := response["success"].(bool); !ok || !success {
				t.Error("Response should have success=true")
			}

			// Check if response has data field with expected structure
			if data, ok := response["data"].(map[string]any); ok {
				// Verify all required fields are present
				requiredFields := []string{"totalPayments", "successRate", "totalVolume", "avgResponseTime",
					"totalPaymentsChange", "successRateChange", "totalVolumeChange", "avgResponseChange"}

				for _, field := range requiredFields {
					if _, exists := data[field]; !exists {
						t.Errorf("Response should contain %s field", field)
					}
				}
			} else {
				t.Error("Response should contain data field with proper structure")
			}
		})
	}
}

func TestAnalyticsHandler_GetProviderStats(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		expectedStatus int
		logger         *opensearch.Logger
	}{
		{
			name:           "successful provider stats with nil logger",
			tenantID:       "APP1",
			expectedStatus: 200,
			logger:         nil,
		},
		{
			name:           "successful provider stats without tenant",
			expectedStatus: 200,
			logger:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAnalyticsHandler(tt.logger)

			req := httptest.NewRequest("GET", "/analytics/providers", nil)
			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()
			handler.GetProviderStats(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
			}

			// Check if response has provider stats array
			if data, ok := response["data"].([]any); ok {
				if len(data) == 0 {
					t.Error("Response should contain provider stats")
				}

				// Check first provider structure
				if len(data) > 0 {
					if provider, ok := data[0].(map[string]any); ok {
						requiredFields := []string{"name", "status", "responseTime", "transactions", "successRate"}
						for _, field := range requiredFields {
							if _, exists := provider[field]; !exists {
								t.Errorf("Provider stats should contain %s field", field)
							}
						}
					}
				}
			} else {
				t.Error("Response should contain data field with array")
			}
		})
	}
}

func TestAnalyticsHandler_GetRecentActivity(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		limitParam     string
		expectedStatus int
		logger         *opensearch.Logger
	}{
		{
			name:           "successful recent activity",
			tenantID:       "APP1",
			limitParam:     "10",
			expectedStatus: 200,
			logger:         nil,
		},
		{
			name:           "successful recent activity without tenant",
			limitParam:     "5",
			expectedStatus: 200,
			logger:         nil,
		},
		{
			name:           "invalid limit parameter - fallback to 10",
			limitParam:     "abc",
			expectedStatus: 200,
			logger:         nil,
		},
		{
			name:           "limit over maximum - fallback to 50",
			limitParam:     "100",
			expectedStatus: 200,
			logger:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAnalyticsHandler(tt.logger)

			url := "/analytics/recent"
			if tt.limitParam != "" {
				url += "?limit=" + tt.limitParam
			}

			req := httptest.NewRequest("GET", url, nil)
			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()
			handler.GetRecentActivity(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
			}

			// Check if response has recent activity data
			if data, ok := response["data"].([]any); ok {
				if len(data) == 0 {
					t.Error("Response should contain recent activity data")
				}

				// Check first activity structure
				if len(data) > 0 {
					if activity, ok := data[0].(map[string]any); ok {
						requiredFields := []string{"type", "provider", "amount", "status", "time", "id"}
						for _, field := range requiredFields {
							if _, exists := activity[field]; !exists {
								t.Errorf("Recent activity should contain %s field", field)
							}
						}
					}
				}
			} else {
				t.Error("Response should contain data field with array")
			}
		})
	}
}

func TestAnalyticsHandler_GetPaymentTrends(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		hoursParam     string
		expectedStatus int
		logger         *opensearch.Logger
	}{
		{
			name:           "successful payment trends",
			tenantID:       "APP1",
			hoursParam:     "24",
			expectedStatus: 200,
			logger:         nil,
		},
		{
			name:           "successful payment trends without tenant",
			hoursParam:     "48",
			expectedStatus: 200,
			logger:         nil,
		},
		{
			name:           "invalid hours parameter",
			hoursParam:     "abc",
			expectedStatus: 200,
			logger:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAnalyticsHandler(tt.logger)

			url := "/analytics/trends"
			if tt.hoursParam != "" {
				url += "?hours=" + tt.hoursParam
			}

			req := httptest.NewRequest("GET", url, nil)
			if tt.tenantID != "" {
				req.Header.Set("X-Tenant-ID", tt.tenantID)
			}

			w := httptest.NewRecorder()
			handler.GetPaymentTrends(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
			}

			// Check if response has trends data
			if data, ok := response["data"].(map[string]any); ok {
				if len(data) == 0 {
					t.Error("Response should contain trends data")
				}
			} else {
				t.Error("Response should contain data field")
			}
		})
	}
}

func TestAnalyticsHandler_HTTPMethods(t *testing.T) {
	handler := NewAnalyticsHandler(nil)

	tests := []struct {
		name    string
		method  string
		path    string
		handler func(w http.ResponseWriter, r *http.Request)
	}{
		{"dashboard stats", "GET", "/analytics/dashboard", handler.GetDashboardStats},
		{"provider stats", "GET", "/analytics/providers", handler.GetProviderStats},
		{"recent activity", "GET", "/analytics/recent", handler.GetRecentActivity},
		{"payment trends", "GET", "/analytics/trends", handler.GetPaymentTrends},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test GET method (should work)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			tt.handler(w, req)

			if w.Code != 200 {
				t.Errorf("Expected status 200 for GET, got %d", w.Code)
			}

			// Test POST method (should still work but might return different response)
			req = httptest.NewRequest("POST", tt.path, nil)
			w = httptest.NewRecorder()
			tt.handler(w, req)

			// Analytics endpoints typically accept any method and process the request
			if w.Code >= 500 {
				t.Errorf("POST method should not cause server error, got %d", w.Code)
			}
		})
	}
}

func BenchmarkAnalyticsHandler_GetDashboardStats(b *testing.B) {
	handler := NewAnalyticsHandler(nil)
	req := httptest.NewRequest("GET", "/analytics/dashboard", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.GetDashboardStats(w, req)
	}
}

func BenchmarkAnalyticsHandler_GetProviderStats(b *testing.B) {
	handler := NewAnalyticsHandler(nil)
	req := httptest.NewRequest("GET", "/analytics/providers", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.GetProviderStats(w, req)
	}
}

func BenchmarkAnalyticsHandler_GetRecentActivity(b *testing.B) {
	handler := NewAnalyticsHandler(nil)
	req := httptest.NewRequest("GET", "/analytics/recent", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.GetRecentActivity(w, req)
	}
}
