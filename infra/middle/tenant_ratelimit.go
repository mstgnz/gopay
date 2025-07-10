package middle

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/response"
)

// TenantRateLimiter represents a tenant-aware rate limiter
type TenantRateLimiter struct {
	tenants map[string]*tenantBucket // tenant_id -> bucket
	ips     map[string]*visitor      // ip -> bucket (for unauthenticated requests)
	mu      sync.RWMutex
	config  *TenantRateLimitConfig
}

// tenantBucket holds rate limiting information for a specific tenant
type tenantBucket struct {
	actions    map[string]*actionBucket // action -> bucket
	globalRate *visitor                 // global tenant rate limit
	lastSeen   time.Time
}

// actionBucket holds rate limiting for specific actions (payment, refund, etc.)
type actionBucket struct {
	count     int
	lastReset time.Time
}

// TenantRateLimitConfig holds configuration for different rate limits
type TenantRateLimitConfig struct {
	// Global defaults
	DefaultGlobalRate   int           `json:"default_global_rate"`  // requests per minute per tenant
	DefaultPaymentRate  int           `json:"default_payment_rate"` // payments per minute per tenant
	DefaultRefundRate   int           `json:"default_refund_rate"`  // refunds per minute per tenant
	DefaultStatusRate   int           `json:"default_status_rate"`  // status checks per minute per tenant
	DefaultWindow       time.Duration `json:"default_window"`       // time window
	UnauthenticatedRate int           `json:"unauthenticated_rate"` // rate for non-authenticated requests (per IP)

	// Tenant-specific overrides
	TenantOverrides map[string]*TenantLimits `json:"tenant_overrides"`

	// Premium tenant benefits
	PremiumTenants    map[string]bool `json:"premium_tenants"`
	PremiumMultiplier float64         `json:"premium_multiplier"` // multiply rates for premium tenants

	// Burst allowance
	BurstAllowance int `json:"burst_allowance"` // allow burst above normal rate
}

// TenantLimits holds specific limits for a tenant
type TenantLimits struct {
	GlobalRate  int `json:"global_rate"`
	PaymentRate int `json:"payment_rate"`
	RefundRate  int `json:"refund_rate"`
	StatusRate  int `json:"status_rate"`
}

// ActionType represents different types of actions that can be rate limited
type ActionType string

const (
	ActionGlobal  ActionType = "global"
	ActionPayment ActionType = "payment"
	ActionRefund  ActionType = "refund"
	ActionStatus  ActionType = "status"
	ActionAuth    ActionType = "auth"
	ActionConfig  ActionType = "config"
)

// NewTenantRateLimiter creates a new tenant-aware rate limiter
func NewTenantRateLimiter() *TenantRateLimiter {
	config := loadTenantRateLimitConfig()

	rl := &TenantRateLimiter{
		tenants: make(map[string]*tenantBucket),
		ips:     make(map[string]*visitor),
		config:  config,
	}

	// Start cleanup routine
	go rl.cleanup()

	return rl
}

// loadTenantRateLimitConfig loads rate limit configuration from environment
func loadTenantRateLimitConfig() *TenantRateLimitConfig {
	cfg := &TenantRateLimitConfig{
		DefaultWindow:     time.Minute,
		TenantOverrides:   make(map[string]*TenantLimits),
		PremiumTenants:    make(map[string]bool),
		PremiumMultiplier: 2.0, // Premium tenants get 2x the rate
		BurstAllowance:    10,  // Allow 10 extra requests in burst
	}

	// Load from environment with defaults
	cfg.DefaultGlobalRate = config.GetIntEnv("TENANT_GLOBAL_RATE_LIMIT", 100)    // 100/min per tenant
	cfg.DefaultPaymentRate = config.GetIntEnv("TENANT_PAYMENT_RATE_LIMIT", 50)   // 50/min payments per tenant
	cfg.DefaultRefundRate = config.GetIntEnv("TENANT_REFUND_RATE_LIMIT", 20)     // 20/min refunds per tenant
	cfg.DefaultStatusRate = config.GetIntEnv("TENANT_STATUS_RATE_LIMIT", 200)    // 200/min status checks per tenant
	cfg.UnauthenticatedRate = config.GetIntEnv("UNAUTHENTICATED_RATE_LIMIT", 10) // 10/min per IP for unauthenticated

	return cfg
}

// Allow checks if the request is allowed for a specific tenant and action
func (trl *TenantRateLimiter) Allow(tenantID string, action ActionType, clientIP string) (bool, *RateLimitInfo) {
	trl.mu.Lock()
	defer trl.mu.Unlock()

	now := time.Now()

	// Handle unauthenticated requests (no tenant ID)
	if tenantID == "" {
		return trl.allowUnauthenticated(clientIP, now)
	}

	// Get or create tenant bucket
	bucket, exists := trl.tenants[tenantID]
	if !exists || now.Sub(bucket.lastSeen) > trl.config.DefaultWindow*2 {
		bucket = &tenantBucket{
			actions:    make(map[string]*actionBucket),
			globalRate: &visitor{count: 0, lastReset: now},
			lastSeen:   now,
		}
		trl.tenants[tenantID] = bucket
	}

	bucket.lastSeen = now

	// Get rate limits for this tenant
	limits := trl.getTenantLimits(tenantID)

	// Check global tenant rate limit first
	if !trl.checkLimit(bucket.globalRate, limits.GlobalRate, now) {
		return false, &RateLimitInfo{
			Allowed:    false,
			Limit:      limits.GlobalRate,
			Remaining:  0,
			ResetTime:  bucket.globalRate.lastReset.Add(trl.config.DefaultWindow),
			RetryAfter: int(time.Until(bucket.globalRate.lastReset.Add(trl.config.DefaultWindow)).Seconds()),
			ActionType: string(action),
			TenantID:   tenantID,
		}
	}

	// Check action-specific rate limit
	actionKey := string(action)
	actionBucketPtr, exists := bucket.actions[actionKey]
	if !exists || now.Sub(actionBucketPtr.lastReset) > trl.config.DefaultWindow {
		actionBucketPtr = &actionBucket{
			count:     0,
			lastReset: now,
		}
		bucket.actions[actionKey] = actionBucketPtr
	}

	actionLimit := trl.getActionLimit(action, limits)
	if !trl.checkActionLimit(actionBucketPtr, actionLimit, now) {
		return false, &RateLimitInfo{
			Allowed:    false,
			Limit:      actionLimit,
			Remaining:  0,
			ResetTime:  actionBucketPtr.lastReset.Add(trl.config.DefaultWindow),
			RetryAfter: int(time.Until(actionBucketPtr.lastReset.Add(trl.config.DefaultWindow)).Seconds()),
			ActionType: string(action),
			TenantID:   tenantID,
		}
	}

	// Increment counters
	bucket.globalRate.count++
	actionBucketPtr.count++

	return true, &RateLimitInfo{
		Allowed:    true,
		Limit:      actionLimit,
		Remaining:  max(0, actionLimit-actionBucketPtr.count),
		ResetTime:  actionBucketPtr.lastReset.Add(trl.config.DefaultWindow),
		RetryAfter: 0,
		ActionType: string(action),
		TenantID:   tenantID,
	}
}

// allowUnauthenticated handles rate limiting for requests without authentication
func (trl *TenantRateLimiter) allowUnauthenticated(clientIP string, now time.Time) (bool, *RateLimitInfo) {
	v, exists := trl.ips[clientIP]
	if !exists || now.Sub(v.lastReset) > trl.config.DefaultWindow {
		trl.ips[clientIP] = &visitor{
			count:     1,
			lastReset: now,
		}
		return true, &RateLimitInfo{
			Allowed:    true,
			Limit:      trl.config.UnauthenticatedRate,
			Remaining:  trl.config.UnauthenticatedRate - 1,
			ResetTime:  now.Add(trl.config.DefaultWindow),
			RetryAfter: 0,
			ActionType: "unauthenticated",
		}
	}

	if v.count >= trl.config.UnauthenticatedRate {
		return false, &RateLimitInfo{
			Allowed:    false,
			Limit:      trl.config.UnauthenticatedRate,
			Remaining:  0,
			ResetTime:  v.lastReset.Add(trl.config.DefaultWindow),
			RetryAfter: int(trl.config.DefaultWindow.Seconds()),
			ActionType: "unauthenticated",
		}
	}

	v.count++
	return true, &RateLimitInfo{
		Allowed:    true,
		Limit:      trl.config.UnauthenticatedRate,
		Remaining:  trl.config.UnauthenticatedRate - v.count,
		ResetTime:  v.lastReset.Add(trl.config.DefaultWindow),
		RetryAfter: 0,
		ActionType: "unauthenticated",
	}
}

// getTenantLimits returns the rate limits for a specific tenant
func (trl *TenantRateLimiter) getTenantLimits(tenantID string) *TenantLimits {
	// Check for tenant-specific overrides
	if override, exists := trl.config.TenantOverrides[tenantID]; exists {
		limits := *override // copy
		// Apply premium multiplier if tenant is premium
		if trl.config.PremiumTenants[tenantID] {
			limits.GlobalRate = int(float64(limits.GlobalRate) * trl.config.PremiumMultiplier)
			limits.PaymentRate = int(float64(limits.PaymentRate) * trl.config.PremiumMultiplier)
			limits.RefundRate = int(float64(limits.RefundRate) * trl.config.PremiumMultiplier)
			limits.StatusRate = int(float64(limits.StatusRate) * trl.config.PremiumMultiplier)
		}
		return &limits
	}

	// Use defaults
	limits := &TenantLimits{
		GlobalRate:  trl.config.DefaultGlobalRate,
		PaymentRate: trl.config.DefaultPaymentRate,
		RefundRate:  trl.config.DefaultRefundRate,
		StatusRate:  trl.config.DefaultStatusRate,
	}

	// Apply premium multiplier if tenant is premium
	if trl.config.PremiumTenants[tenantID] {
		limits.GlobalRate = int(float64(limits.GlobalRate) * trl.config.PremiumMultiplier)
		limits.PaymentRate = int(float64(limits.PaymentRate) * trl.config.PremiumMultiplier)
		limits.RefundRate = int(float64(limits.RefundRate) * trl.config.PremiumMultiplier)
		limits.StatusRate = int(float64(limits.StatusRate) * trl.config.PremiumMultiplier)
	}

	return limits
}

// getActionLimit returns the rate limit for a specific action
func (trl *TenantRateLimiter) getActionLimit(action ActionType, limits *TenantLimits) int {
	switch action {
	case ActionPayment:
		return limits.PaymentRate
	case ActionRefund:
		return limits.RefundRate
	case ActionStatus:
		return limits.StatusRate
	case ActionAuth:
		return limits.GlobalRate / 2 // Auth requests get half the global rate
	case ActionConfig:
		return limits.GlobalRate / 4 // Config requests get quarter of global rate
	default:
		return limits.GlobalRate
	}
}

// checkLimit checks if the request is within the rate limit
func (trl *TenantRateLimiter) checkLimit(v *visitor, limit int, now time.Time) bool {
	if now.Sub(v.lastReset) > trl.config.DefaultWindow {
		v.count = 0
		v.lastReset = now
	}

	// Allow burst above normal rate
	effectiveLimit := limit + trl.config.BurstAllowance
	return v.count < effectiveLimit
}

// checkActionLimit checks action-specific rate limit
func (trl *TenantRateLimiter) checkActionLimit(ab *actionBucket, limit int, now time.Time) bool {
	if now.Sub(ab.lastReset) > trl.config.DefaultWindow {
		ab.count = 0
		ab.lastReset = now
	}

	// Allow burst above normal rate
	effectiveLimit := limit + trl.config.BurstAllowance
	return ab.count < effectiveLimit
}

// cleanup removes old entries
func (trl *TenantRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		trl.mu.Lock()
		now := time.Now()

		// Clean up tenant buckets
		for tenantID, bucket := range trl.tenants {
			if now.Sub(bucket.lastSeen) > trl.config.DefaultWindow*3 {
				delete(trl.tenants, tenantID)
				continue
			}

			// Clean up action buckets within tenant
			for action, actionBucket := range bucket.actions {
				if now.Sub(actionBucket.lastReset) > trl.config.DefaultWindow*2 {
					delete(bucket.actions, action)
				}
			}
		}

		// Clean up IP buckets
		for ip, v := range trl.ips {
			if now.Sub(v.lastReset) > trl.config.DefaultWindow*2 {
				delete(trl.ips, ip)
			}
		}

		trl.mu.Unlock()
	}
}

// RateLimitInfo contains information about rate limiting status
type RateLimitInfo struct {
	Allowed    bool      `json:"allowed"`
	Limit      int       `json:"limit"`
	Remaining  int       `json:"remaining"`
	ResetTime  time.Time `json:"reset_time"`
	RetryAfter int       `json:"retry_after"` // seconds
	ActionType string    `json:"action_type"`
	TenantID   string    `json:"tenant_id"`
}

// TenantRateLimitMiddleware creates a tenant-aware rate limiting middleware
func TenantRateLimitMiddleware(trl *TenantRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for certain paths
			if shouldSkipRateLimit(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Get tenant ID from JWT context (if authenticated)
			tenantID := GetTenantIDFromContext(r.Context())

			// Get client IP for fallback
			clientIP := GetClientIP(r)

			// Determine action type based on URL path and method
			action := determineActionType(r.URL.Path, r.Method)

			// Check rate limit
			allowed, info := trl.Allow(tenantID, action, clientIP)

			// Add rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(info.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(info.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(info.ResetTime.Unix(), 10))
			w.Header().Set("X-RateLimit-Action", info.ActionType)

			if tenantID != "" {
				w.Header().Set("X-RateLimit-Tenant", tenantID)
			}

			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(info.RetryAfter))

				errorMsg := fmt.Sprintf("Rate limit exceeded for %s. Limit: %d/%s",
					info.ActionType, info.Limit, "minute")

				if tenantID != "" {
					errorMsg = fmt.Sprintf("Rate limit exceeded for tenant %s, action %s. Limit: %d/%s",
						tenantID, info.ActionType, info.Limit, "minute")
				}

				response.Error(w, http.StatusTooManyRequests, errorMsg, nil)
				return
			}

			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// shouldSkipRateLimit determines if a request should skip rate limiting
func shouldSkipRateLimit(path string) bool {
	path = strings.ToLower(path)

	// Static assets - no rate limiting
	staticPaths := []string{
		"/public/",
		"/favicon.ico",
		"/robots.txt",
		"/sitemap.xml",
	}

	for _, staticPath := range staticPaths {
		if strings.HasPrefix(path, staticPath) {
			return true
		}
	}

	// Public endpoints - no rate limiting
	publicEndpoints := []string{
		"/health",
		"/docs",
		"/scalar.yaml",
		"/login",                  // Login page
		"/",                       // Dashboard main page
		"/v1/auth/login",          // Login endpoint
		"/v1/auth/register",       // Register endpoint
		"/v1/auth/refresh",        // Token refresh endpoint
		"/v1/auth/validate",       // Token validation endpoint
		"/v1/analytics/dashboard", // Public analytics
		"/v1/analytics/providers", // Public analytics
		"/v1/analytics/activity",  // Public analytics
		"/v1/analytics/trends",    // Public analytics
	}

	for _, endpoint := range publicEndpoints {
		if path == endpoint {
			return true
		}
	}

	return false
}

// determineActionType determines the action type based on URL path and method
func determineActionType(path, method string) ActionType {
	path = strings.ToLower(path)

	// Authentication endpoints
	if strings.Contains(path, "/auth/") {
		return ActionAuth
	}

	// Configuration endpoints
	if strings.Contains(path, "/config/") || strings.Contains(path, "/set-env") {
		return ActionConfig
	}

	// Payment-related endpoints
	if strings.Contains(path, "/payments") {
		if method == "POST" {
			return ActionPayment
		}
		if method == "GET" {
			return ActionStatus
		}
		if strings.Contains(path, "/refund") {
			return ActionRefund
		}
	}

	// Refund endpoints
	if strings.Contains(path, "/refund") {
		return ActionRefund
	}

	// Status check endpoints
	if strings.Contains(path, "/status") || (strings.Contains(path, "/payments") && method == "GET") {
		return ActionStatus
	}

	// Default to global
	return ActionGlobal
}

// GetTenantRateLimitStats returns rate limiting statistics for a tenant
func (trl *TenantRateLimiter) GetTenantRateLimitStats(tenantID string) map[string]any {
	trl.mu.RLock()
	defer trl.mu.RUnlock()

	stats := make(map[string]any)

	if bucket, exists := trl.tenants[tenantID]; exists {
		limits := trl.getTenantLimits(tenantID)

		stats["tenant_id"] = tenantID
		stats["is_premium"] = trl.config.PremiumTenants[tenantID]
		stats["global_limit"] = limits.GlobalRate
		stats["global_used"] = bucket.globalRate.count
		stats["global_remaining"] = max(0, limits.GlobalRate-bucket.globalRate.count)
		stats["last_reset"] = bucket.globalRate.lastReset
		stats["next_reset"] = bucket.globalRate.lastReset.Add(trl.config.DefaultWindow)

		actions := make(map[string]map[string]any)
		for actionName, actionBucket := range bucket.actions {
			actionLimit := trl.getActionLimit(ActionType(actionName), limits)
			actions[actionName] = map[string]any{
				"limit":      actionLimit,
				"used":       actionBucket.count,
				"remaining":  max(0, actionLimit-actionBucket.count),
				"last_reset": actionBucket.lastReset,
				"next_reset": actionBucket.lastReset.Add(trl.config.DefaultWindow),
			}
		}
		stats["actions"] = actions
	} else {
		stats["tenant_id"] = tenantID
		stats["status"] = "no_activity"
	}

	return stats
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
