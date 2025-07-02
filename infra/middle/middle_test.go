package middle

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAuthMiddleware(t *testing.T) {
	// Set test API key
	os.Setenv("API_KEY", "test-api-key")
	defer os.Unsetenv("API_KEY")

	middleware := AuthMiddleware()

	// Test handler
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "Valid API key",
			authHeader:     "Bearer test-api-key",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid API key",
			authHeader:     "Bearer wrong-key",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Missing Authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid format",
			authHeader:     "Basic test-api-key",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Empty Bearer token",
			authHeader:     "Bearer ",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestRateLimiter(t *testing.T) {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     2, // 2 requests per window
		window:   time.Second,
	}

	// Test rate limiting
	clientIP := "192.168.1.1"

	// First request should be allowed
	if !rl.Allow(clientIP) {
		t.Error("First request should be allowed")
	}

	// Second request should be allowed
	if !rl.Allow(clientIP) {
		t.Error("Second request should be allowed")
	}

	// Third request should be blocked
	if rl.Allow(clientIP) {
		t.Error("Third request should be blocked")
	}

	// After waiting for the window, requests should be allowed again
	time.Sleep(time.Second + 100*time.Millisecond)
	if !rl.Allow(clientIP) {
		t.Error("Request after window should be allowed")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     1, // 1 request per window
		window:   time.Second,
	}

	middleware := RateLimitMiddleware(rl)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// First request should succeed
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First request should succeed, got status %d", rr1.Code)
	}

	// Second request from same IP should be rate limited
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12346"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request should be rate limited, got status %d", rr2.Code)
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	middleware := SecurityHeadersMiddleware()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"X-XSS-Protection":        "1; mode=block",
		"Content-Security-Policy": "default-src 'self'",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
	}

	for header, expectedValue := range expectedHeaders {
		if rr.Header().Get(header) != expectedValue {
			t.Errorf("Expected %s: %s, got: %s", header, expectedValue, rr.Header().Get(header))
		}
	}
}

func TestIPWhitelistMiddleware(t *testing.T) {
	// Test with whitelist enabled
	os.Setenv("IP_WHITELIST", "127.0.0.1,192.168.1.100")
	defer os.Unsetenv("IP_WHITELIST")

	middleware := IPWhitelistMiddleware()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	tests := []struct {
		name           string
		clientIP       string
		expectedStatus int
	}{
		{
			name:           "Whitelisted IP",
			clientIP:       "127.0.0.1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Another whitelisted IP",
			clientIP:       "192.168.1.100",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Non-whitelisted IP",
			clientIP:       "192.168.1.999",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.clientIP + ":12345"

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestRequestValidationMiddleware(t *testing.T) {
	middleware := RequestValidationMiddleware()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	tests := []struct {
		name           string
		method         string
		contentType    string
		contentLength  int64
		expectedStatus int
	}{
		{
			name:           "Valid JSON POST",
			method:         "POST",
			contentType:    "application/json",
			contentLength:  100,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid form POST",
			method:         "POST",
			contentType:    "application/x-www-form-urlencoded",
			contentLength:  100,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET request without content type",
			method:         "GET",
			contentType:    "",
			contentLength:  0,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with unsupported content type",
			method:         "POST",
			contentType:    "text/plain",
			contentLength:  100,
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "Request too large",
			method:         "POST",
			contentType:    "application/json",
			contentLength:  11 * 1024 * 1024, // 11MB
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", strings.NewReader("test body"))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			req.ContentLength = tt.contentLength

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}
