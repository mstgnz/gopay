package handler

import (
	"net/http/httptest"
	"testing"
)

func TestNewLogsHandler(t *testing.T) {
	handler := NewLogsHandler(nil, nil)
	if handler == nil {
		t.Error("NewLogsHandler should not return nil")
	}
}

func TestLogsHandler_GetPaymentLogs_WithNilLogger(t *testing.T) {
	handler := NewLogsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/logs/payments?provider=iyzico", nil)
	w := httptest.NewRecorder()
	handler.GetPaymentLogs(w, req)

	if w.Code != 503 {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestLogsHandler_GetSystemLogs_WithNilLogger(t *testing.T) {
	handler := NewLogsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/logs/system", nil)
	w := httptest.NewRecorder()
	handler.GetSystemLogs(w, req)

	if w.Code != 503 {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestLogsHandler_GetLogStats_WithNilLogger(t *testing.T) {
	handler := NewLogsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/logs/stats", nil)
	w := httptest.NewRecorder()
	handler.GetLogStats(w, req)

	if w.Code != 503 {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestLogsHandler_GetErrorLogs_MissingProvider(t *testing.T) {
	handler := NewLogsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/logs/errors", nil)
	w := httptest.NewRecorder()
	handler.GetErrorLogs(w, req)

	if w.Code != 401 {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestLogsHandler_GetPaymentLogs_MissingProvider(t *testing.T) {
	handler := NewLogsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/logs/payments", nil)
	w := httptest.NewRecorder()
	handler.GetPaymentLogs(w, req)

	if w.Code != 503 {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestLogsHandler_ListLogs_MissingTenant(t *testing.T) {
	handler := NewLogsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/logs", nil)
	w := httptest.NewRecorder()
	handler.ListLogs(w, req)

	if w.Code != 401 {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestLogsHandler_EdgeCases(t *testing.T) {
	handler := NewLogsHandler(nil, nil)

	// Test various parameter combinations
	tests := []struct {
		name           string
		method         string
		path           string
		headers        map[string]string
		expectedStatus int
	}{
		{
			name:           "payment logs with parameters",
			method:         "GET",
			path:           "/logs/payments?provider=iyzico&hours=24&payment_id=test",
			expectedStatus: 503,
		},
		{
			name:           "system logs with filters",
			method:         "GET",
			path:           "/logs/system?level=ERROR&component=payment&limit=10",
			expectedStatus: 503,
		},
		{
			name:           "error logs with tenant",
			method:         "GET",
			path:           "/logs/errors?provider=iyzico&hours=48",
			headers:        map[string]string{"X-Tenant-ID": "APP1"},
			expectedStatus: 401, // Changed to 401 because JWT authentication is required
		},
		{
			name:           "log stats with invalid hours",
			method:         "GET",
			path:           "/logs/stats?hours=invalid",
			expectedStatus: 503,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			w := httptest.NewRecorder()

			switch {
			case tt.path == "/logs/payments" || (tt.path != "" && tt.path[0:14] == "/logs/payments"):
				handler.GetPaymentLogs(w, req)
			case tt.path == "/logs/system" || (tt.path != "" && tt.path[0:12] == "/logs/system"):
				handler.GetSystemLogs(w, req)
			case tt.path == "/logs/errors" || (tt.path != "" && tt.path[0:12] == "/logs/errors"):
				handler.GetErrorLogs(w, req)
			case tt.path == "/logs/stats" || (tt.path != "" && tt.path[0:11] == "/logs/stats"):
				handler.GetLogStats(w, req)
			default:
				handler.ListLogs(w, req)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
