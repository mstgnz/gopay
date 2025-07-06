package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mstgnz/gopay/infra/logger"
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
	logger           LoggerInterface
	openSearchLogger *opensearch.Logger
}

// NewLogsHandler creates a new logs handler
func NewLogsHandler(logger LoggerInterface, openSearchLogger *opensearch.Logger) *LogsHandler {
	return &LogsHandler{
		logger:           logger,
		openSearchLogger: openSearchLogger,
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
	if h.openSearchLogger == nil {
		response.Error(w, http.StatusServiceUnavailable, "Logging service not available", nil)
		return
	}

	// Get parameters
	tenantID := r.Header.Get("X-Tenant-ID")
	provider := r.URL.Query().Get("provider")
	hours := r.URL.Query().Get("hours")
	paymentID := r.URL.Query().Get("payment_id")

	if provider == "" {
		response.Error(w, http.StatusBadRequest, "provider parameter is required", nil)
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

	var logs []opensearch.PaymentLog
	var err error

	if paymentID != "" {
		// Get logs for specific payment
		logs, err = h.openSearchLogger.GetPaymentLogs(ctx, tenantID, provider, paymentID)
	} else {
		// Get recent logs
		query := map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{
					{
						"range": map[string]any{
							"timestamp": map[string]any{
								"gte": "now-" + strconv.Itoa(hoursInt) + "h",
							},
						},
					},
				},
			},
		}

		// Add tenant filter if provided
		if tenantID != "" {
			query["bool"].(map[string]any)["must"] = append(
				query["bool"].(map[string]any)["must"].([]map[string]any),
				map[string]any{
					"term": map[string]any{
						"tenant_id": tenantID,
					},
				},
			)
		}

		logs, err = h.openSearchLogger.SearchLogs(ctx, tenantID, provider, query)
	}

	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve logs", err)
		return
	}

	responseData := map[string]any{
		"logs":      logs,
		"count":     len(logs),
		"tenant_id": tenantID,
		"provider":  provider,
		"hours":     hoursInt,
	}

	if paymentID != "" {
		responseData["payment_id"] = paymentID
	}

	response.Success(w, http.StatusOK, "Logs retrieved successfully", responseData)
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

// GetSystemLogs retrieves system logs with optional filtering
func (h *LogsHandler) GetSystemLogs(w http.ResponseWriter, r *http.Request) {
	if h.openSearchLogger == nil {
		response.Error(w, http.StatusServiceUnavailable, "Logging service not available", nil)
		return
	}

	// Get parameters
	tenantID := r.Header.Get("X-Tenant-ID")
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

	// Build search query
	query := map[string]any{
		"bool": map[string]any{
			"must": []map[string]any{
				{
					"range": map[string]any{
						"timestamp": map[string]any{
							"gte": "now-" + strconv.Itoa(hoursInt) + "h",
						},
					},
				},
			},
		},
	}

	// Add tenant filter if provided
	if tenantID != "" {
		query["bool"].(map[string]any)["must"] = append(
			query["bool"].(map[string]any)["must"].([]map[string]any),
			map[string]any{
				"term": map[string]any{
					"tenant_id": tenantID,
				},
			},
		)
	}

	// Add level filter if provided
	if level != "" {
		query["bool"].(map[string]any)["must"] = append(
			query["bool"].(map[string]any)["must"].([]map[string]any),
			map[string]any{
				"term": map[string]any{
					"level": level,
				},
			},
		)
	}

	// Add component filter if provided
	if component != "" {
		query["bool"].(map[string]any)["must"] = append(
			query["bool"].(map[string]any)["must"].([]map[string]any),
			map[string]any{
				"term": map[string]any{
					"component": component,
				},
			},
		)
	}

	// Search system logs
	searchQuery := map[string]any{
		"query": query,
		"sort": []map[string]any{
			{"timestamp": map[string]string{"order": "desc"}},
		},
		"size": limitInt,
	}

	logs, err := h.searchSystemLogs(ctx, searchQuery)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve system logs", err)
		return
	}

	responseData := map[string]any{
		"logs":      logs,
		"count":     len(logs),
		"tenant_id": tenantID,
		"level":     level,
		"component": component,
		"hours":     hoursInt,
		"limit":     limitInt,
	}

	response.Success(w, http.StatusOK, "System logs retrieved successfully", responseData)
}

// searchSystemLogs performs the actual search in OpenSearch
func (h *LogsHandler) searchSystemLogs(ctx context.Context, searchQuery map[string]any) ([]logger.SystemLog, error) {
	// This is a simplified implementation
	// In a real implementation, you would use the OpenSearch client to search the system logs index

	// For now, return empty array
	// TODO: Implement actual OpenSearch query for system logs
	return []logger.SystemLog{}, nil
}

// GetLogStats retrieves logging statistics
func (h *LogsHandler) GetLogStats(w http.ResponseWriter, r *http.Request) {
	if h.openSearchLogger == nil {
		response.Error(w, http.StatusServiceUnavailable, "Logging service not available", nil)
		return
	}

	tenantID := r.Header.Get("X-Tenant-ID")
	hours := r.URL.Query().Get("hours")

	hoursInt := 24 // Default to 24 hours
	if hours != "" {
		if h, err := strconv.Atoi(hours); err == nil && h > 0 {
			hoursInt = h
		}
	}

	// Build aggregation query for statistics
	statsQuery := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{
					{
						"range": map[string]any{
							"timestamp": map[string]any{
								"gte": "now-" + strconv.Itoa(hoursInt) + "h",
							},
						},
					},
				},
			},
		},
		"aggs": map[string]any{
			"by_level": map[string]any{
				"terms": map[string]any{
					"field": "level",
				},
			},
			"by_component": map[string]any{
				"terms": map[string]any{
					"field": "component",
				},
			},
			"by_provider": map[string]any{
				"terms": map[string]any{
					"field": "provider",
				},
			},
		},
		"size": 0, // Only aggregations, no documents
	}

	// Add tenant filter if provided
	if tenantID != "" {
		statsQuery["query"].(map[string]any)["bool"].(map[string]any)["must"] = append(
			statsQuery["query"].(map[string]any)["bool"].(map[string]any)["must"].([]map[string]any),
			map[string]any{
				"term": map[string]any{
					"tenant_id": tenantID,
				},
			},
		)
	}

	// For now, return mock statistics
	// TODO: Implement actual OpenSearch aggregation query
	stats := map[string]any{
		"total_logs": 0,
		"by_level": map[string]int{
			"error": 0,
			"warn":  0,
			"info":  0,
			"debug": 0,
		},
		"by_component": map[string]int{},
		"by_provider":  map[string]int{},
		"time_range": map[string]any{
			"hours": hoursInt,
			"from":  time.Now().Add(-time.Duration(hoursInt) * time.Hour).Format(time.RFC3339),
			"to":    time.Now().Format(time.RFC3339),
		},
	}

	responseData := map[string]any{
		"stats":     stats,
		"tenant_id": tenantID,
		"hours":     hoursInt,
	}

	response.Success(w, http.StatusOK, "Log statistics retrieved successfully", responseData)
}
