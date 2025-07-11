package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mstgnz/gopay/infra/middle"
	"github.com/mstgnz/gopay/infra/postgres"
	"github.com/mstgnz/gopay/infra/response"
)

// LoggerInterface defines the interface for logging operations
type LoggerInterface interface {
	SearchLogs(ctx context.Context, tenantID, provider string, query map[string]any) ([]postgres.PaymentLog, error)
	GetPaymentLogs(ctx context.Context, tenantID, provider, paymentID string) ([]postgres.PaymentLog, error)
	GetRecentErrorLogs(ctx context.Context, tenantID, provider string, hours int) ([]postgres.PaymentLog, error)
	GetProviderStats(ctx context.Context, tenantID, provider string, hours int) (map[string]any, error)
}

// LogsHandler handles logs related HTTP requests
type LogsHandler struct {
	logger         LoggerInterface
	postgresLogger *postgres.Logger
}

// NewLogsHandler creates a new logs handler
func NewLogsHandler(logger LoggerInterface, postgresLogger *postgres.Logger) *LogsHandler {
	return &LogsHandler{
		logger:         logger,
		postgresLogger: postgresLogger,
	}
}

// ListLogs lists payment logs with filtering by tenant and provider
func (h *LogsHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get tenant ID from JWT token context (automatically set by auth middleware)
	tenantID := middle.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		response.Error(w, http.StatusUnauthorized, "Invalid or missing authentication", nil)
		return
	}

	// Get provider from URL path parameter (required)
	provider := chi.URLParam(r, "provider")
	if provider == "" {
		response.Error(w, http.StatusBadRequest, "Provider parameter is required", nil)
		return
	}

	// Parse query parameters
	var query map[string]any = make(map[string]any)

	// Payment ID filter
	if paymentID := r.URL.Query().Get("paymentId"); paymentID != "" {
		query = map[string]any{
			"match": map[string]any{
				"payment_info.payment_id": paymentID,
			},
		}
	}

	// Status filter
	if status := r.URL.Query().Get("status"); status != "" {
		if len(query) == 0 {
			query = map[string]any{
				"match": map[string]any{
					"payment_info.status": status,
				},
			}
		} else {
			// Combine with bool query if payment ID is also present
			existing := query
			query = map[string]any{
				"bool": map[string]any{
					"must": []map[string]any{
						existing,
						{
							"match": map[string]any{
								"payment_info.status": status,
							},
						},
					},
				},
			}
		}
	}

	// Error filter (only errors)
	if errorsOnly := r.URL.Query().Get("errorsOnly"); errorsOnly == "true" {
		errorFilter := map[string]any{
			"exists": map[string]any{
				"field": "error.code",
			},
		}

		if len(query) == 0 {
			query = errorFilter
		} else {
			// Combine with existing query
			existing := query
			query = map[string]any{
				"bool": map[string]any{
					"must": []map[string]any{
						existing,
						errorFilter,
					},
				},
			}
		}
	}

	// Time range filter
	hoursStr := r.URL.Query().Get("hours")
	hours := 24 // Default to 24 hours
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 && h <= 168 { // Max 7 days
			hours = h
		}
	}

	timeFilter := map[string]any{
		"range": map[string]any{
			"timestamp": map[string]any{
				"gte": fmt.Sprintf("now-%dh", hours),
			},
		},
	}

	if len(query) == 0 {
		query = timeFilter
	} else {
		// Combine with existing query
		existing := query
		query = map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{
					existing,
					timeFilter,
				},
			},
		}
	}

	// Search logs
	logs, err := h.logger.SearchLogs(ctx, tenantID, provider, query)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to search logs", err)
		return
	}

	// Prepare response data
	responseData := map[string]any{
		"tenantId": tenantID,
		"provider": provider,
		"filters": map[string]any{
			"hours":      hours,
			"paymentId":  r.URL.Query().Get("paymentId"),
			"status":     r.URL.Query().Get("status"),
			"errorsOnly": r.URL.Query().Get("errorsOnly") == "true",
		},
		"count": len(logs),
		"logs":  logs,
	}

	response.Success(w, http.StatusOK, "Logs retrieved successfully", responseData)
}

// GetPaymentLogs retrieves logs for a specific payment ID
func (h *LogsHandler) GetPaymentLogs(w http.ResponseWriter, r *http.Request) {
	if h.logger == nil {
		response.Error(w, http.StatusServiceUnavailable, "Logging service not available", nil)
		return
	}

	// Get parameters
	tenantID := middle.GetTenantIDFromContext(r.Context())
	provider := chi.URLParam(r, "provider")
	paymentID := chi.URLParam(r, "paymentID")

	if provider == "" {
		response.Error(w, http.StatusBadRequest, "provider parameter is required", nil)
		return
	}

	if paymentID == "" {
		response.Error(w, http.StatusBadRequest, "paymentID parameter is required", nil)
		return
	}

	if tenantID == "" {
		response.Error(w, http.StatusUnauthorized, "Invalid or missing authentication", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get logs for specific payment
	logs, err := h.logger.GetPaymentLogs(ctx, tenantID, provider, paymentID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve logs", err)
		return
	}

	responseData := map[string]any{
		"logs":       logs,
		"count":      len(logs),
		"tenant_id":  tenantID,
		"provider":   provider,
		"payment_id": paymentID,
	}

	response.Success(w, http.StatusOK, "Logs retrieved successfully", responseData)
}

// GetErrorLogs retrieves recent error logs for a provider
func (h *LogsHandler) GetErrorLogs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get tenant ID from JWT token context (automatically set by auth middleware)
	tenantID := middle.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		response.Error(w, http.StatusUnauthorized, "Invalid or missing authentication", nil)
		return
	}

	// Get provider from URL path parameter
	provider := chi.URLParam(r, "provider")
	if provider == "" {
		response.Error(w, http.StatusBadRequest, "Provider parameter is required", nil)
		return
	}

	// Parse hours parameter
	hoursStr := r.URL.Query().Get("hours")
	hours := 24 // Default to 24 hours
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 && h <= 168 { // Max 7 days
			hours = h
		}
	}

	// Get error logs
	logs, err := h.logger.GetRecentErrorLogs(ctx, tenantID, provider, hours)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to get error logs", err)
		return
	}

	// Prepare response data
	responseData := map[string]any{
		"tenantId": tenantID,
		"provider": provider,
		"hours":    hours,
		"count":    len(logs),
		"logs":     logs,
	}

	response.Success(w, http.StatusOK, "Error logs retrieved successfully", responseData)
}

// GetSystemLogs retrieves system logs with optional filtering
func (h *LogsHandler) GetSystemLogs(w http.ResponseWriter, r *http.Request) {
	if h.postgresLogger == nil {
		response.Error(w, http.StatusServiceUnavailable, "Logging service not available", nil)
		return
	}

	// Get parameters
	tenantID := middle.GetTenantIDFromContext(r.Context())
	level := r.URL.Query().Get("level")
	component := r.URL.Query().Get("component")
	hours := r.URL.Query().Get("hours")
	limit := r.URL.Query().Get("limit")

	hoursInt := 24 // Default to 24 hours
	if hours != "" {
		if h, err := strconv.Atoi(hours); err == nil && h > 0 {
			hoursInt = h
		}
	}

	limitInt := 100 // Default limit
	if limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 && l <= 1000 {
			limitInt = l
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get system logs from PostgreSQL
	filters := map[string]any{
		"start_date": time.Now().Add(-time.Duration(hoursInt) * time.Hour),
		"end_date":   time.Now(),
		"limit":      limitInt,
	}

	if level != "" {
		filters["level"] = level
	}

	if component != "" {
		filters["component"] = component
	}

	if tenantID != "" {
		tenantIDInt, convErr := strconv.Atoi(tenantID)
		if convErr == nil {
			filters["tenant_id"] = tenantIDInt
		}
	}

	systemLogs, err := h.postgresLogger.SearchSystemLogs(ctx, filters)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve system logs", err)
		return
	}

	responseData := map[string]any{
		"logs":      systemLogs,
		"count":     len(systemLogs),
		"tenant_id": tenantID,
		"level":     level,
		"component": component,
		"hours":     hoursInt,
		"limit":     limitInt,
	}

	response.Success(w, http.StatusOK, "System logs retrieved successfully", responseData)
}

// GetLogStats retrieves log statistics
func (h *LogsHandler) GetLogStats(w http.ResponseWriter, r *http.Request) {
	if h.logger == nil {
		response.Error(w, http.StatusServiceUnavailable, "Logging service not available", nil)
		return
	}

	// Get parameters
	tenantID := middle.GetTenantIDFromContext(r.Context())
	provider := chi.URLParam(r, "provider")
	hours := r.URL.Query().Get("hours")

	if provider == "" {
		response.Error(w, http.StatusBadRequest, "provider parameter is required", nil)
		return
	}

	if tenantID == "" {
		response.Error(w, http.StatusUnauthorized, "Invalid or missing authentication", nil)
		return
	}

	hoursInt := 24 // Default to 24 hours
	if hours != "" {
		if h, err := strconv.Atoi(hours); err == nil && h > 0 {
			hoursInt = h
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get stats from provider-specific logger
	stats, err := h.logger.GetProviderStats(ctx, tenantID, provider, hoursInt)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve log statistics", err)
		return
	}

	responseData := map[string]any{
		"stats":     stats,
		"tenant_id": tenantID,
		"provider":  provider,
		"hours":     hoursInt,
	}

	response.Success(w, http.StatusOK, "Log statistics retrieved successfully", responseData)
}
