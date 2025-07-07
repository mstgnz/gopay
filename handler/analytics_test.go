package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mstgnz/gopay/infra/postgres"
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

	// Test with mock logger (using postgres.Logger)
	logger := &postgres.Logger{}
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
		logger         *postgres.Logger
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
		logger         *postgres.Logger
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
		logger         *postgres.Logger
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
		logger         *postgres.Logger
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

func TestAnalyticsHandler_CalculationFunctions(t *testing.T) {
	// Test with nil logger (fallback behavior)
	handler := NewAnalyticsHandler(nil)

	tests := []struct {
		name     string
		hours    int
		expected string
	}{
		{
			name:     "payment change calculation",
			hours:    24,
			expected: "+12.5% from yesterday", // Default fallback value
		},
		{
			name:     "success rate change calculation",
			hours:    24,
			expected: "+0.8% from yesterday", // Default fallback value
		},
		{
			name:     "volume change calculation",
			hours:    24,
			expected: "+18.2% from yesterday", // Default fallback value
		},
		{
			name:     "response time change calculation",
			hours:    24,
			expected: "-15ms from yesterday", // Default fallback value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "payment change calculation":
				result := handler.calculatePaymentChange(0, tt.hours)
				if result != tt.expected {
					t.Errorf("calculatePaymentChange: expected %s, got %s", tt.expected, result)
				}
			case "success rate change calculation":
				result := handler.calculateSuccessRateChange(0, tt.hours)
				if result != tt.expected {
					t.Errorf("calculateSuccessRateChange: expected %s, got %s", tt.expected, result)
				}
			case "volume change calculation":
				result := handler.calculateVolumeChange(0, tt.hours)
				if result != tt.expected {
					t.Errorf("calculateVolumeChange: expected %s, got %s", tt.expected, result)
				}
			case "response time change calculation":
				result := handler.calculateResponseTimeChange(0, tt.hours)
				if result != tt.expected {
					t.Errorf("calculateResponseTimeChange: expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

// Test edge cases and error handling for better coverage
func TestAnalyticsHandler_EdgeCases(t *testing.T) {
	handler := NewAnalyticsHandler(nil)

	// Test with extreme parameter values
	t.Run("dashboard stats with zero hours", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/analytics/dashboard?hours=0", nil)
		w := httptest.NewRecorder()
		handler.GetDashboardStats(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("recent activity with zero limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/analytics/activity?limit=0", nil)
		w := httptest.NewRecorder()
		handler.GetRecentActivity(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("payment trends with very large hours", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/analytics/trends?hours=999999", nil)
		w := httptest.NewRecorder()
		handler.GetPaymentTrends(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

// Test real analytics functions
func TestAnalyticsHandler_RealDataFunctions(t *testing.T) {
	// Test getRealDashboardStats
	t.Run("getRealDashboardStats with mock data", func(t *testing.T) {
		// Create a real handler but we'll need to mock the logger methods
		handler := NewAnalyticsHandler(nil)

		// We'll test the logic by calling the function directly with our mock
		// Since we can't easily mock the private methods, let's test the public methods instead

		// Test that the handler can handle nil logger gracefully
		req := httptest.NewRequest("GET", "/analytics/dashboard", nil)
		w := httptest.NewRecorder()
		handler.GetDashboardStats(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	// Test getProviderVolume logic by testing the overall flow
	t.Run("provider volume calculation logic", func(t *testing.T) {
		handler := NewAnalyticsHandler(nil)

		// Test with nil logger - should return generated data
		req := httptest.NewRequest("GET", "/analytics/providers", nil)
		w := httptest.NewRecorder()
		handler.GetProviderStats(w, req)

		if w.Code != 200 {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]any
		json.NewDecoder(w.Body).Decode(&response)

		if data, ok := response["data"]; ok {
			if providers, ok := data.([]any); ok && len(providers) > 0 {
				t.Logf("Generated %d provider stats", len(providers))
			}
		}
	})

	// Test getRealRecentActivity logic
	t.Run("recent activity with various conditions", func(t *testing.T) {
		handler := NewAnalyticsHandler(nil)

		// Test with different limit values
		testCases := []struct {
			limit string
			name  string
		}{
			{"5", "small limit"},
			{"25", "medium limit"},
			{"100", "large limit"},
			{"invalid", "invalid limit"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/analytics/activity?limit="+tc.limit, nil)
				w := httptest.NewRecorder()
				handler.GetRecentActivity(w, req)

				if w.Code != 200 {
					t.Errorf("Expected status 200, got %d", w.Code)
				}
			})
		}
	})

	// Test getRealPaymentTrends
	t.Run("payment trends with various hours", func(t *testing.T) {
		handler := NewAnalyticsHandler(nil)

		testCases := []struct {
			hours string
			name  string
		}{
			{"24", "24 hours"},
			{"48", "48 hours"},
			{"168", "1 week"},
			{"200", "over limit"},
			{"invalid", "invalid hours"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/analytics/trends?hours="+tc.hours, nil)
				w := httptest.NewRecorder()
				handler.GetPaymentTrends(w, req)

				if w.Code != 200 {
					t.Errorf("Expected status 200, got %d", w.Code)
				}

				var response map[string]any
				json.NewDecoder(w.Body).Decode(&response)

				if data, ok := response["data"]; ok {
					if trends, ok := data.(map[string]any); ok {
						if _, hasLabels := trends["labels"]; !hasLabels {
							t.Error("Expected labels in trends data")
						}
						if _, hasDatasets := trends["datasets"]; !hasDatasets {
							t.Error("Expected datasets in trends data")
						}
					}
				}
			})
		}
	})
}

// Test calculation functions with comprehensive edge cases
func TestAnalyticsHandler_CalculationFunctionsEdgeCases(t *testing.T) {
	// Test with nil logger (fallback values)
	t.Run("calculation functions with nil logger", func(t *testing.T) {
		handler := NewAnalyticsHandler(nil)

		paymentChange := handler.calculatePaymentChange(0, 24)
		successRateChange := handler.calculateSuccessRateChange(0, 24)
		volumeChange := handler.calculateVolumeChange(0, 24)
		responseTimeChange := handler.calculateResponseTimeChange(0, 24)

		// With nil logger, these should return fixed fallback values
		if paymentChange != "+12.5% from yesterday" {
			t.Errorf("Expected '+12.5%% from yesterday', got '%s'", paymentChange)
		}
		if successRateChange != "+0.8% from yesterday" {
			t.Errorf("Expected '+0.8%% from yesterday', got '%s'", successRateChange)
		}
		if volumeChange != "+18.2% from yesterday" {
			t.Errorf("Expected '+18.2%% from yesterday', got '%s'", volumeChange)
		}
		if responseTimeChange != "-15ms from yesterday" {
			t.Errorf("Expected '-15ms from yesterday', got '%s'", responseTimeChange)
		}
	})

	// Test with mock logger (random values)
	t.Run("calculation functions with mock logger", func(t *testing.T) {
		// Create a mock logger to trigger the random calculation path
		mockLogger := &postgres.Logger{}
		handler := NewAnalyticsHandler(mockLogger)

		// Test multiple times to ensure they return valid formats
		for i := 0; i < 5; i++ {
			paymentChange := handler.calculatePaymentChange(0, 24)
			successRateChange := handler.calculateSuccessRateChange(0, 24)
			volumeChange := handler.calculateVolumeChange(0, 24)
			responseTimeChange := handler.calculateResponseTimeChange(0, 24)

			// Check format consistency - with mock logger (no DB), should return "No data" messages
			if !strings.Contains(paymentChange, "from previous") && !strings.Contains(paymentChange, "No previous data") {
				t.Errorf("Payment change should contain 'from previous' or 'No previous data', got '%s'", paymentChange)
			}
			if !strings.Contains(successRateChange, "from previous") && !strings.Contains(successRateChange, "No data available") {
				t.Errorf("Success rate change should contain 'from previous' or 'No data available', got '%s'", successRateChange)
			}
			if !strings.Contains(volumeChange, "from previous") && !strings.Contains(volumeChange, "No previous data") {
				t.Errorf("Volume change should contain 'from previous' or 'No previous data', got '%s'", volumeChange)
			}
			if !strings.Contains(responseTimeChange, "from previous") && !strings.Contains(responseTimeChange, "No data available") {
				t.Errorf("Response time change should contain 'from previous' or 'No data available', got '%s'", responseTimeChange)
			}

			// Check that percentage changes contain % or no data message
			if !strings.Contains(paymentChange, "%") && !strings.Contains(paymentChange, "No previous data") {
				t.Errorf("Payment change should contain %% or 'No previous data', got '%s'", paymentChange)
			}
			if !strings.Contains(successRateChange, "%") && !strings.Contains(successRateChange, "No data available") {
				t.Errorf("Success rate change should contain %% or 'No data available', got '%s'", successRateChange)
			}
			if !strings.Contains(volumeChange, "%") && !strings.Contains(volumeChange, "No previous data") {
				t.Errorf("Volume change should contain %% or 'No previous data', got '%s'", volumeChange)
			}

			// Response time change contains ms or no data message
			if !strings.Contains(responseTimeChange, "ms") && !strings.Contains(responseTimeChange, "No data available") {
				t.Errorf("Response time change should contain 'ms' or 'No data available', got '%s'", responseTimeChange)
			}
		}
	})

	// Test calculation functions with different hours values (should not affect output with nil logger)
	t.Run("calculation functions with different hours (nil logger)", func(t *testing.T) {
		handler := NewAnalyticsHandler(nil)

		testCases := []int{0, 1, 6, 12, 24, 48, 72, 168, 1000, -5}

		for _, hours := range testCases {
			paymentChange := handler.calculatePaymentChange(0, hours)
			successRateChange := handler.calculateSuccessRateChange(0, hours)
			volumeChange := handler.calculateVolumeChange(0, hours)
			responseTimeChange := handler.calculateResponseTimeChange(0, hours)

			// With nil logger, hours parameter should not affect the output
			if paymentChange != "+12.5% from yesterday" {
				t.Errorf("For hours=%d, expected '+12.5%% from yesterday', got '%s'", hours, paymentChange)
			}
			if successRateChange != "+0.8% from yesterday" {
				t.Errorf("For hours=%d, expected '+0.8%% from yesterday', got '%s'", hours, successRateChange)
			}
			if volumeChange != "+18.2% from yesterday" {
				t.Errorf("For hours=%d, expected '+18.2%% from yesterday', got '%s'", hours, volumeChange)
			}
			if responseTimeChange != "-15ms from yesterday" {
				t.Errorf("For hours=%d, expected '-15ms from yesterday', got '%s'", hours, responseTimeChange)
			}
		}
	})

	// Test calculation functions with boundary values
	t.Run("calculation functions with boundary values", func(t *testing.T) {
		handler := NewAnalyticsHandler(nil)

		// Test with minimum and maximum int values
		extremeValues := []int{-2147483648, 2147483647}

		for _, hours := range extremeValues {
			paymentChange := handler.calculatePaymentChange(0, hours)
			successRateChange := handler.calculateSuccessRateChange(0, hours)
			volumeChange := handler.calculateVolumeChange(0, hours)
			responseTimeChange := handler.calculateResponseTimeChange(0, hours)

			// Should still return valid strings
			if paymentChange == "" {
				t.Errorf("Payment change should not be empty for hours=%d", hours)
			}
			if successRateChange == "" {
				t.Errorf("Success rate change should not be empty for hours=%d", hours)
			}
			if volumeChange == "" {
				t.Errorf("Volume change should not be empty for hours=%d", hours)
			}
			if responseTimeChange == "" {
				t.Errorf("Response time change should not be empty for hours=%d", hours)
			}
		}
	})

	// Test that calculation functions are deterministic with nil logger
	t.Run("calculation functions deterministic with nil logger", func(t *testing.T) {
		handler := NewAnalyticsHandler(nil)

		// Call multiple times and ensure consistent results
		for i := 0; i < 10; i++ {
			paymentChange := handler.calculatePaymentChange(0, 24)
			successRateChange := handler.calculateSuccessRateChange(0, 24)
			volumeChange := handler.calculateVolumeChange(0, 24)
			responseTimeChange := handler.calculateResponseTimeChange(0, 24)

			if paymentChange != "+12.5% from yesterday" {
				t.Errorf("Payment change should be consistent, got '%s'", paymentChange)
			}
			if successRateChange != "+0.8% from yesterday" {
				t.Errorf("Success rate change should be consistent, got '%s'", successRateChange)
			}
			if volumeChange != "+18.2% from yesterday" {
				t.Errorf("Volume change should be consistent, got '%s'", volumeChange)
			}
			if responseTimeChange != "-15ms from yesterday" {
				t.Errorf("Response time change should be consistent, got '%s'", responseTimeChange)
			}
		}
	})
}
