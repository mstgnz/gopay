package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mstgnz/gopay/infra/opensearch"
	"github.com/mstgnz/gopay/infra/response"
)

// LoggerInterface defines the interface for logging operations
type LoggerInterface interface {
	SearchLogs(ctx context.Context, tenantID, provider string, query map[string]any) ([]opensearch.PaymentLog, error)
	GetPaymentLogs(ctx context.Context, tenantID, provider, paymentID string) ([]opensearch.PaymentLog, error)
	GetRecentErrorLogs(ctx context.Context, tenantID, provider string, hours int) ([]opensearch.PaymentLog, error)
	GetProviderStats(ctx context.Context, tenantID, provider string, hours int) (map[string]any, error)
}

// LogsHandler handles logs related HTTP requests
type LogsHandler struct {
	logger LoggerInterface
}

// NewLogsHandler creates a new logs handler
func NewLogsHandler(logger LoggerInterface) *LogsHandler {
	return &LogsHandler{
		logger: logger,
	}
}

// ListLogs lists payment logs with filtering by tenant and provider
func (h *LogsHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get tenant ID from header (required)
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		response.Error(w, http.StatusBadRequest, "X-Tenant-ID header is required", nil)
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
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get tenant ID from header (required)
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		response.Error(w, http.StatusBadRequest, "X-Tenant-ID header is required", nil)
		return
	}

	// Get provider and payment ID from URL path parameters
	provider := chi.URLParam(r, "provider")
	paymentID := chi.URLParam(r, "paymentID")

	if provider == "" {
		response.Error(w, http.StatusBadRequest, "Provider parameter is required", nil)
		return
	}

	if paymentID == "" {
		response.Error(w, http.StatusBadRequest, "Payment ID parameter is required", nil)
		return
	}

	// Get payment logs
	logs, err := h.logger.GetPaymentLogs(ctx, tenantID, provider, paymentID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to get payment logs", err)
		return
	}

	// Prepare response data
	responseData := map[string]any{
		"tenantId":  tenantID,
		"provider":  provider,
		"paymentId": paymentID,
		"count":     len(logs),
		"logs":      logs,
	}

	response.Success(w, http.StatusOK, "Payment logs retrieved successfully", responseData)
}

// GetErrorLogs retrieves recent error logs for a provider
func (h *LogsHandler) GetErrorLogs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get tenant ID from header (required)
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		response.Error(w, http.StatusBadRequest, "X-Tenant-ID header is required", nil)
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

// GetLogStats retrieves logging statistics for a provider
func (h *LogsHandler) GetLogStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get tenant ID from header (required)
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		response.Error(w, http.StatusBadRequest, "X-Tenant-ID header is required", nil)
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

	// Get statistics
	stats, err := h.logger.GetProviderStats(ctx, tenantID, provider, hours)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to get statistics", err)
		return
	}

	// Prepare response data
	responseData := map[string]any{
		"tenantId": tenantID,
		"provider": provider,
		"hours":    hours,
		"stats":    stats,
	}

	response.Success(w, http.StatusOK, "Statistics retrieved successfully", responseData)
}
