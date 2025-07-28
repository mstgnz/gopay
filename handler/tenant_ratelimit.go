package handler

import (
	"net/http"

	"github.com/mstgnz/gopay/infra/middle"
	"github.com/mstgnz/gopay/infra/response"
)

// TenantRateLimitHandler handles tenant rate limiting operations
type TenantRateLimitHandler struct {
	rateLimiter *middle.TenantRateLimiter
}

// NewTenantRateLimitHandler creates a new tenant rate limit handler
func NewTenantRateLimitHandler(rateLimiter *middle.TenantRateLimiter) *TenantRateLimitHandler {
	return &TenantRateLimitHandler{
		rateLimiter: rateLimiter,
	}
}

// GetTenantStats returns rate limiting statistics for the authenticated tenant
func (h *TenantRateLimitHandler) GetTenantStats(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from JWT context
	tenantID := middle.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		response.Error(w, http.StatusUnauthorized, "Tenant ID not found in token", nil)
		return
	}

	// Get rate limiting statistics for this tenant
	stats := h.rateLimiter.GetTenantRateLimitStats(tenantID)

	_ = response.WriteJSON(w, http.StatusOK, response.Response{
		Success: true,
		Message: "Tenant rate limiting statistics retrieved successfully",
		Data:    stats,
	})
}
