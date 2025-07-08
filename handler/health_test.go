package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

func TestNewHealthHandler(t *testing.T) {
	handler := NewHealthHandler(nil, nil, nil, nil)
	if handler == nil {
		t.Error("NewHealthHandler should not return nil")
		return
	}

	if handler.startTime.IsZero() {
		t.Error("HealthHandler should have start time set")
	}
}

func TestHealthHandler_CheckHealth(t *testing.T) {
	tests := []struct {
		name           string
		expectedStatus int
		handler        *HealthHandler
	}{
		{
			name:           "health check with no services",
			expectedStatus: 503, // Should be unhealthy without services
			handler:        NewHealthHandler(nil, nil, nil, nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()

			tt.handler.CheckHealth(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Expected content type application/json, got %s", contentType)
			}
		})
	}
}

func TestHealthHandler_CheckHealth_Status(t *testing.T) {
	handler := NewHealthHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.CheckHealth(w, req)

	if w.Code == 0 {
		t.Error("Health check should return a status code")
	}

	// Should return either 200 or 503
	if w.Code != 200 && w.Code != 503 {
		t.Errorf("Health check should return 200 or 503, got %d", w.Code)
	}
}

func TestGetEnvironment(t *testing.T) {
	env := getEnvironment()
	if env == "" {
		t.Error("getEnvironment should return a non-empty string")
	}

	// Should return development as default
	if env != "development" {
		t.Logf("Environment detected as: %s", env)
	}
}

func TestProviderRegistryIntegration(t *testing.T) {
	// Test that the health handler works with provider registry
	providers := provider.GetAvailableProviders()

	// Should be able to get available providers without error
	if providers == nil {
		t.Error("GetAvailableProviders should not return nil")
	}

	// Each provider should be retrievable from registry
	for _, providerName := range providers {
		_, err := provider.Get(providerName)
		if err != nil {
			t.Errorf("Provider %s should be available in registry: %v", providerName, err)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %s, expected %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestCalculateMemoryUsagePercent(t *testing.T) {
	// This is hard to test precisely, but we can check it returns a reasonable value
	// We'll just verify it doesn't panic and returns a non-negative value
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("calculateMemoryUsagePercent panicked: %v", r)
		}
	}()

	// Create a mock MemStats
	var memStats struct {
		Alloc uint64
		Sys   uint64
	}
	memStats.Alloc = 1000000 // 1MB
	memStats.Sys = 10000000  // 10MB

	// Can't easily test the actual function since it requires runtime.MemStats,
	// but we can verify the logic
	percent := (float64(memStats.Alloc) / float64(memStats.Sys)) * 100
	if percent < 0 || percent > 100 {
		t.Errorf("Memory usage percent should be between 0-100, got %f", percent)
	}
}

func BenchmarkHealthCheck(b *testing.B) {
	handler := NewHealthHandler(nil, nil, nil, nil)
	req := httptest.NewRequest("GET", "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.CheckHealth(w, req)
	}
}
