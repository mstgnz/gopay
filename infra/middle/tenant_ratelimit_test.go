package middle

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestNewTenantRateLimiter(t *testing.T) {
	// Set test environment variables
	os.Setenv("TENANT_GLOBAL_RATE_LIMIT", "10")
	os.Setenv("TENANT_PAYMENT_RATE_LIMIT", "5")
	os.Setenv("PREMIUM_TENANTS", "premium1,premium2")
	defer func() {
		os.Unsetenv("TENANT_GLOBAL_RATE_LIMIT")
		os.Unsetenv("TENANT_PAYMENT_RATE_LIMIT")
		os.Unsetenv("PREMIUM_TENANTS")
	}()

	rl := NewTenantRateLimiter()

	if rl == nil {
		t.Fatal("NewTenantRateLimiter should not return nil")
	}

	if rl.config.DefaultGlobalRate != 10 {
		t.Errorf("Expected global rate 10, got %d", rl.config.DefaultGlobalRate)
	}

	if rl.config.DefaultPaymentRate != 5 {
		t.Errorf("Expected payment rate 5, got %d", rl.config.DefaultPaymentRate)
	}

	if !rl.config.PremiumTenants["premium1"] {
		t.Error("premium1 should be in premium tenants")
	}

	if !rl.config.PremiumTenants["premium2"] {
		t.Error("premium2 should be in premium tenants")
	}
}

func TestTenantRateLimiter_Allow(t *testing.T) {
	rl := &TenantRateLimiter{
		tenants: make(map[string]*tenantBucket),
		ips:     make(map[string]*visitor),
		config: &TenantRateLimitConfig{
			DefaultGlobalRate:   2,
			DefaultPaymentRate:  1,
			DefaultRefundRate:   1,
			DefaultStatusRate:   3,
			DefaultWindow:       time.Second,
			UnauthenticatedRate: 1,
			TenantOverrides:     make(map[string]*TenantLimits),
			PremiumTenants:      make(map[string]bool),
			PremiumMultiplier:   2.0,
			BurstAllowance:      1,
		},
	}

	tenantID := "test-tenant"
	clientIP := "192.168.1.1"

	// Test first payment request - should be allowed
	allowed, info := rl.Allow(tenantID, ActionPayment, clientIP)
	if !allowed {
		t.Error("First payment should be allowed")
	}
	if info.Remaining != 0 { // 1 limit, 1 used = 0 remaining
		t.Errorf("Expected 0 remaining, got %d", info.Remaining)
	}

	// Test second payment request - should be blocked (burst allows 1 extra)
	allowed, _ = rl.Allow(tenantID, ActionPayment, clientIP)
	if !allowed {
		t.Error("Second payment should be allowed due to burst")
	}

	// Test third payment request - should be blocked
	allowed, info = rl.Allow(tenantID, ActionPayment, clientIP)
	if allowed {
		t.Error("Third payment should be blocked")
	}
	if info.RetryAfter < 0 {
		t.Error("RetryAfter should not be negative")
	}

	// Test different action type - should be allowed
	allowed, _ = rl.Allow(tenantID, ActionStatus, clientIP)
	if !allowed {
		t.Error("Status check should be allowed (different action bucket)")
	}
}

func TestTenantRateLimiter_UnauthenticatedRequests(t *testing.T) {
	rl := &TenantRateLimiter{
		tenants: make(map[string]*tenantBucket),
		ips:     make(map[string]*visitor),
		config: &TenantRateLimitConfig{
			DefaultGlobalRate:   100,
			DefaultPaymentRate:  50,
			DefaultWindow:       time.Second,
			UnauthenticatedRate: 1,
			TenantOverrides:     make(map[string]*TenantLimits),
			PremiumTenants:      make(map[string]bool),
			BurstAllowance:      0,
		},
	}

	clientIP := "192.168.1.1"

	// Test unauthenticated request (no tenant ID)
	allowed, info := rl.Allow("", ActionGlobal, clientIP)
	if !allowed {
		t.Error("First unauthenticated request should be allowed")
	}
	if info.ActionType != "unauthenticated" {
		t.Errorf("Expected action type 'unauthenticated', got %s", info.ActionType)
	}

	// Test second unauthenticated request - should be blocked
	allowed, _ = rl.Allow("", ActionGlobal, clientIP)
	if allowed {
		t.Error("Second unauthenticated request should be blocked")
	}
}

func TestTenantRateLimiter_PremiumTenants(t *testing.T) {
	rl := &TenantRateLimiter{
		tenants: make(map[string]*tenantBucket),
		ips:     make(map[string]*visitor),
		config: &TenantRateLimitConfig{
			DefaultGlobalRate:   2,
			DefaultPaymentRate:  1,
			DefaultWindow:       time.Second,
			UnauthenticatedRate: 1,
			TenantOverrides:     make(map[string]*TenantLimits),
			PremiumTenants: map[string]bool{
				"premium-tenant": true,
			},
			PremiumMultiplier: 2.0,
			BurstAllowance:    0,
		},
	}

	regularTenant := "regular-tenant"
	premiumTenant := "premium-tenant"
	clientIP := "192.168.1.1"

	// Regular tenant - should be blocked after 1 payment
	rl.Allow(regularTenant, ActionPayment, clientIP)
	allowed, _ := rl.Allow(regularTenant, ActionPayment, clientIP)
	if allowed {
		t.Error("Regular tenant should be blocked after 1 payment")
	}

	// Premium tenant - should allow 2 payments (multiplier effect)
	rl.Allow(premiumTenant, ActionPayment, clientIP)
	allowed, _ = rl.Allow(premiumTenant, ActionPayment, clientIP)
	if !allowed {
		t.Error("Premium tenant should allow 2 payments")
	}

	// Premium tenant - should be blocked after 2 payments
	allowed, _ = rl.Allow(premiumTenant, ActionPayment, clientIP)
	if allowed {
		t.Error("Premium tenant should be blocked after 2 payments")
	}
}

func TestDetermineActionType(t *testing.T) {
	tests := []struct {
		path     string
		method   string
		expected ActionType
	}{
		{"/v1/auth/login", "POST", ActionAuth},
		{"/v1/auth/register", "POST", ActionAuth},
		{"/v1/set-env", "POST", ActionConfig},
		{"/v1/config/tenant-config", "GET", ActionConfig},
		{"/v1/payments/iyzico", "POST", ActionPayment},
		{"/v1/payments/iyzico/pay123", "GET", ActionStatus},
		{"/v1/payments/iyzico/refund", "POST", ActionRefund},
		{"/v1/refund", "POST", ActionRefund},
		{"/v1/status/check", "GET", ActionStatus},
		{"/v1/other", "GET", ActionGlobal},
	}

	for _, tt := range tests {
		t.Run(tt.path+"_"+tt.method, func(t *testing.T) {
			result := determineActionType(tt.path, tt.method)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s for %s %s", tt.expected, result, tt.method, tt.path)
			}
		})
	}
}

func TestTenantRateLimitMiddleware(t *testing.T) {
	rl := &TenantRateLimiter{
		tenants: make(map[string]*tenantBucket),
		ips:     make(map[string]*visitor),
		config: &TenantRateLimitConfig{
			DefaultGlobalRate:   1,
			DefaultPaymentRate:  1,
			DefaultWindow:       time.Second,
			UnauthenticatedRate: 1,
			TenantOverrides:     make(map[string]*TenantLimits),
			PremiumTenants:      make(map[string]bool),
			BurstAllowance:      0,
		},
	}

	middleware := TenantRateLimitMiddleware(rl)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	// Test with tenant context
	req1 := httptest.NewRequest("POST", "/v1/payments/iyzico", nil)
	req1.RemoteAddr = "192.168.1.1:12345"

	// Add tenant to context
	ctx := context.WithValue(req1.Context(), TenantIDKey, "test-tenant")
	req1 = req1.WithContext(ctx)

	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First request should succeed, got status %d", rr1.Code)
	}

	// Check rate limit headers
	if rr1.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("X-RateLimit-Limit header should be set")
	}
	if rr1.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("X-RateLimit-Remaining header should be set")
	}
	if rr1.Header().Get("X-RateLimit-Tenant") != "test-tenant" {
		t.Error("X-RateLimit-Tenant header should be set to tenant ID")
	}

	// Test second request - should be rate limited
	req2 := httptest.NewRequest("POST", "/v1/payments/iyzico", nil)
	req2.RemoteAddr = "192.168.1.1:12346"
	ctx2 := context.WithValue(req2.Context(), TenantIDKey, "test-tenant")
	req2 = req2.WithContext(ctx2)

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request should be rate limited, got status %d", rr2.Code)
	}

	// Check Retry-After header
	if rr2.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header should be set when rate limited")
	}
}

func TestTenantRateLimitMiddleware_UnauthenticatedRequests(t *testing.T) {
	rl := &TenantRateLimiter{
		tenants: make(map[string]*tenantBucket),
		ips:     make(map[string]*visitor),
		config: &TenantRateLimitConfig{
			DefaultGlobalRate:   100,
			DefaultPaymentRate:  50,
			DefaultWindow:       time.Second,
			UnauthenticatedRate: 1,
			TenantOverrides:     make(map[string]*TenantLimits),
			PremiumTenants:      make(map[string]bool),
			BurstAllowance:      0,
		},
	}

	middleware := TenantRateLimitMiddleware(rl)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	// Test unauthenticated request (no tenant context)
	req1 := httptest.NewRequest("GET", "/health", nil)
	req1.RemoteAddr = "192.168.1.1:12345"

	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First unauthenticated request should succeed, got status %d", rr1.Code)
	}

	if rr1.Header().Get("X-RateLimit-Tenant") != "" {
		t.Error("X-RateLimit-Tenant header should not be set for unauthenticated requests")
	}

	// Test second unauthenticated request from same IP - should be rate limited
	req2 := httptest.NewRequest("GET", "/health", nil)
	req2.RemoteAddr = "192.168.1.1:12346"

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Second unauthenticated request should be rate limited, got status %d", rr2.Code)
	}
}

func TestGetTenantRateLimitStats(t *testing.T) {
	rl := &TenantRateLimiter{
		tenants: make(map[string]*tenantBucket),
		ips:     make(map[string]*visitor),
		config: &TenantRateLimitConfig{
			DefaultGlobalRate:  10,
			DefaultPaymentRate: 5,
			DefaultWindow:      time.Minute,
			TenantOverrides:    make(map[string]*TenantLimits),
			PremiumTenants:     make(map[string]bool),
			BurstAllowance:     2,
		},
	}

	tenantID := "test-tenant"
	clientIP := "192.168.1.1"

	// Make some requests to generate stats
	rl.Allow(tenantID, ActionPayment, clientIP)
	rl.Allow(tenantID, ActionPayment, clientIP)
	rl.Allow(tenantID, ActionStatus, clientIP)

	stats := rl.GetTenantRateLimitStats(tenantID)

	if stats["tenant_id"] != tenantID {
		t.Errorf("Expected tenant_id %s, got %v", tenantID, stats["tenant_id"])
	}

	if stats["global_used"] != 3 {
		t.Errorf("Expected global_used 3, got %v", stats["global_used"])
	}

	if stats["global_remaining"] != 7 {
		t.Errorf("Expected global_remaining 7, got %v", stats["global_remaining"])
	}

	// Check actions stats
	actions, ok := stats["actions"].(map[string]map[string]any)
	if !ok {
		t.Error("Actions should be a map")
		return
	}

	if paymentStats, exists := actions["payment"]; exists {
		if paymentStats["used"] != 2 {
			t.Errorf("Expected payment used 2, got %v", paymentStats["used"])
		}
		if paymentStats["remaining"] != 3 { // 5 limit - 2 used = 3
			t.Errorf("Expected payment remaining 3, got %v", paymentStats["remaining"])
		}
	} else {
		t.Error("Payment action stats should exist")
	}
}

func TestGetTenantRateLimitStats_NoActivity(t *testing.T) {
	rl := &TenantRateLimiter{
		tenants: make(map[string]*tenantBucket),
		ips:     make(map[string]*visitor),
		config: &TenantRateLimitConfig{
			DefaultGlobalRate:  10,
			DefaultPaymentRate: 5,
			TenantOverrides:    make(map[string]*TenantLimits),
			PremiumTenants:     make(map[string]bool),
		},
	}

	stats := rl.GetTenantRateLimitStats("non-existent-tenant")

	if stats["tenant_id"] != "non-existent-tenant" {
		t.Errorf("Expected tenant_id non-existent-tenant, got %v", stats["tenant_id"])
	}

	if stats["status"] != "no_activity" {
		t.Errorf("Expected status no_activity, got %v", stats["status"])
	}
}
