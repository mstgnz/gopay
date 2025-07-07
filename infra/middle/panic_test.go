package middle

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mstgnz/gopay/infra/response"
)

func TestPanicRecoveryMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectedStatus int
		shouldPanic    bool
	}{
		{
			name: "Normal request - no panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			},
			expectedStatus: http.StatusOK,
			shouldPanic:    false,
		},
		{
			name: "Handler panics with string",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			},
			expectedStatus: http.StatusInternalServerError,
			shouldPanic:    true,
		},
		{
			name: "Handler panics with error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic(http.ErrAbortHandler)
			},
			expectedStatus: http.StatusInternalServerError,
			shouldPanic:    true,
		},
		{
			name: "Handler panics with nil",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("runtime error: invalid memory address or nil pointer dereference")
			},
			expectedStatus: http.StatusInternalServerError,
			shouldPanic:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the middleware
			middleware := PanicRecoveryMiddleware()

			// Wrap the test handler with middleware
			wrappedHandler := middleware(tt.handler)

			// Create test request
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Request-ID", "test-request-123")

			// Add tenant context
			ctx := context.WithValue(req.Context(), TenantIDKey, "test-tenant")
			req = req.WithContext(ctx)

			// Create response recorder
			w := httptest.NewRecorder()

			// Execute the handler
			wrappedHandler.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// For panic cases, check response structure
			if tt.shouldPanic {
				// Check content type
				contentType := w.Header().Get("Content-Type")
				if !strings.Contains(contentType, "application/json") {
					t.Error("Expected JSON content type for panic response")
				}

				// Check cache headers
				cacheControl := w.Header().Get("Cache-Control")
				if cacheControl != "no-cache, no-store, must-revalidate" {
					t.Error("Expected no-cache header for panic response")
				}

				// Parse response body
				var resp response.Response
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Errorf("Failed to decode panic response: %v", err)
				}

				// Check response structure
				if resp.Success {
					t.Error("Expected success=false for panic response")
				}

				if resp.Code != http.StatusInternalServerError {
					t.Errorf("Expected code %d, got %d", http.StatusInternalServerError, resp.Code)
				}

				if resp.Message != "Internal server error" {
					t.Errorf("Expected message 'Internal server error', got '%s'", resp.Message)
				}

				if resp.Error != "an unexpected error occurred" {
					t.Errorf("Expected error 'an unexpected error occurred', got '%s'", resp.Error)
				}
			}
		})
	}
}

func TestPanicRecoveryWithCustomHandler(t *testing.T) {
	customHandlerCalled := false
	var capturedPanic any

	customHandler := func(w http.ResponseWriter, r *http.Request, panicValue any) {
		customHandlerCalled = true
		capturedPanic = panicValue
		w.WriteHeader(http.StatusTeapot) // Unique status for testing
		w.Write([]byte("custom panic handler"))
	}

	middleware := PanicRecoveryWithCustomHandler(customHandler)

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("custom test panic")
	})

	wrappedHandler := middleware(panicHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if !customHandlerCalled {
		t.Error("Custom panic handler was not called")
	}

	if capturedPanic != "custom test panic" {
		t.Errorf("Expected panic value 'custom test panic', got %v", capturedPanic)
	}

	if w.Code != http.StatusTeapot {
		t.Errorf("Expected status %d, got %d", http.StatusTeapot, w.Code)
	}

	if w.Body.String() != "custom panic handler" {
		t.Errorf("Expected body 'custom panic handler', got '%s'", w.Body.String())
	}
}

func TestPanicRecoveryMiddleware_WithoutTenantContext(t *testing.T) {
	middleware := PanicRecoveryMiddleware()

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test without tenant")
	})

	wrappedHandler := middleware(panicHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	// No tenant context added

	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	// Should still work without tenant context
	var resp response.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("Expected success=false")
	}
}
