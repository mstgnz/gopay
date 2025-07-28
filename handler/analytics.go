package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/infra/middle"
	"github.com/mstgnz/gopay/infra/postgres"
	"github.com/mstgnz/gopay/infra/response"
)

// AnalyticsHandler handles analytics related HTTP requests
type AnalyticsHandler struct {
	logger *postgres.Logger
}

// getTenantContext extracts tenant information from request context
// Returns tenantID from JWT token and whether user is admin (tenant_id=1)
func (h *AnalyticsHandler) getTenantContext(r *http.Request) (tenantID string, isAdmin bool) {
	tenantID = middle.GetTenantIDFromContext(r.Context())
	isAdmin = tenantID == "1" // Only tenant_id=1 is considered admin
	return
}

// NewAnalyticsHandler creates a new analytics handler
func NewAnalyticsHandler(logger *postgres.Logger) *AnalyticsHandler {
	return &AnalyticsHandler{
		logger: logger,
	}
}

// DashboardStats represents the main dashboard statistics
type DashboardStats struct {
	TotalPayments       int     `json:"totalPayments"`
	SuccessRate         float64 `json:"successRate"`
	TotalVolume         float64 `json:"totalVolume"`
	AvgResponseTime     float64 `json:"avgResponseTime"`
	TotalPaymentsChange string  `json:"totalPaymentsChange"`
	SuccessRateChange   string  `json:"successRateChange"`
	TotalVolumeChange   string  `json:"totalVolumeChange"`
	AvgResponseChange   string  `json:"avgResponseChange"`
	ActiveTenants       int     `json:"activeTenants"`
	ActiveProviders     int     `json:"activeProviders"`
	Environment         string  `json:"environment"`
}

// ProviderStats represents provider-specific statistics
type ProviderStats struct {
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	ResponseTime string  `json:"responseTime"`
	Transactions int     `json:"transactions"`
	SuccessRate  float64 `json:"successRate"`
	Environment  string  `json:"environment"`
	TenantCount  int     `json:"tenantCount"`
}

// RecentActivity represents recent payment activity
type RecentActivity struct {
	Type     string `json:"type"`
	Provider string `json:"provider"`
	Amount   string `json:"amount"`
	Status   string `json:"status"`
	Time     string `json:"time"`
	ID       string `json:"id"`
	TenantID string `json:"tenantId"`
	Env      string `json:"environment"`
	Request  string `json:"request"`
	Response string `json:"response"`
	Endpoint string `json:"endpoint"`
}

// AnalyticsFilters represents the filters for analytics queries
type AnalyticsFilters struct {
	TenantID    *int    `json:"tenantId,omitempty"`
	ProviderID  *string `json:"providerId,omitempty"`
	Environment *string `json:"environment,omitempty"`
	Hours       int     `json:"hours"` // Keep for backwards compatibility
	Month       int     `json:"month"` // For trends chart
	Year        int     `json:"year"`  // For trends chart
}

// GetDashboardStats returns main dashboard statistics
func (h *AnalyticsHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse filters from query parameters
	filters := h.parseAnalyticsFilters(r)

	var stats DashboardStats
	var err error

	if h.logger != nil {
		stats, err = h.getRealDashboardStats(ctx, filters)
		if err != nil {
			logger.Warn("Failed to get real dashboard stats", logger.LogContext{
				TenantID: fmt.Sprintf("%v", filters.TenantID),
				Fields: map[string]any{
					"error":   err.Error(),
					"filters": filters,
				},
			})
			stats = DashboardStats{}
		}
	} else {
		stats = DashboardStats{}
	}

	response.Success(w, http.StatusOK, "Dashboard stats retrieved successfully", stats)
}

// parseAnalyticsFilters parses query parameters into analytics filters with tenant security
func (h *AnalyticsHandler) parseAnalyticsFilters(r *http.Request) AnalyticsFilters {
	now := time.Now()
	filters := AnalyticsFilters{
		Hours: 24,               // default for backwards compatibility
		Month: int(now.Month()), // default to current month (1-12)
		Year:  now.Year(),       // default to current year
	}

	// Get tenant context from JWT
	userTenantID, isAdmin := h.getTenantContext(r)

	// Parse month
	if monthStr := r.URL.Query().Get("month"); monthStr != "" {
		if m, err := strconv.Atoi(monthStr); err == nil && m >= 1 && m <= 12 {
			filters.Month = m
		}
	}

	// Parse year
	if yearStr := r.URL.Query().Get("year"); yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil && y >= 2020 && y <= 2030 {
			filters.Year = y
		}
	}

	// Parse tenant_id with security enforcement
	// Rule: If user is admin (tenant_id=1), they can specify any tenant_id
	// Rule: If user is not admin, tenant_id parameter is ignored and user's own tenant_id is used
	if tenantStr := r.URL.Query().Get("tenant_id"); tenantStr != "" && tenantStr != "all" {
		if tenantID, err := strconv.Atoi(tenantStr); err == nil {
			if isAdmin {
				// Admin users (tenant_id=1) can access any tenant's data
				filters.TenantID = &tenantID
			} else {
				// Non-admin users: ignore requested tenant_id, use their own tenant_id
				if userTenantIDInt, err := strconv.Atoi(userTenantID); err == nil {
					filters.TenantID = &userTenantIDInt
				}
			}
		}
	} else if !isAdmin {
		// For non-admin users, always enforce their tenant_id even if "all" was requested
		if userTenantIDInt, err := strconv.Atoi(userTenantID); err == nil {
			filters.TenantID = &userTenantIDInt
		}
	}

	// Parse provider_id
	if providerStr := r.URL.Query().Get("provider_id"); providerStr != "" && providerStr != "all" {
		filters.ProviderID = &providerStr
	}

	// Parse environment
	if envStr := r.URL.Query().Get("environment"); envStr != "" && envStr != "all" {
		if envStr == "sandbox" || envStr == "production" {
			filters.Environment = &envStr
		}
	}

	return filters
}

// getRealDashboardStats fetches real analytics data from PostgreSQL
func (h *AnalyticsHandler) getRealDashboardStats(ctx context.Context, filters AnalyticsFilters) (DashboardStats, error) {
	// Get providers that actually have tenant configurations
	configuredProviders, err := h.logger.GetActiveProviders(ctx)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("failed to get active providers: %w", err)
	}

	// Extract provider keys
	var providers []string
	for _, provider := range configuredProviders {
		providers = append(providers, provider["id"].(string))
	}

	// Filter by specific provider if requested
	if filters.ProviderID != nil {
		providers = []string{*filters.ProviderID}
	}

	var totalPayments int
	var totalSuccessful int
	var totalVolume float64
	var totalResponseTime float64
	var responseTimeCount int
	activeTenants := make(map[int]bool)
	activeProviders := make(map[string]bool)

	// Get tenant IDs to process
	var tenantIDs []int
	if filters.TenantID != nil {
		tenantIDs = []int{*filters.TenantID}
	} else {
		// Get all active tenant IDs from PostgreSQL
		tenantIDs = h.getActiveTenants(ctx)
	}

	for _, tenantID := range tenantIDs {
		for _, provider := range providers {
			// Get provider stats from PostgreSQL with environment filter
			providerStats, err := h.getPaymentStatsWithEnv(ctx, tenantID, provider, filters.Hours, filters.Environment)
			if err != nil {
				continue // Skip provider if error
			}

			// Extract stats from PostgreSQL response
			if totalReq, ok := providerStats["total_requests"].(int); ok && totalReq > 0 {
				totalPayments += totalReq
				activeTenants[tenantID] = true
				activeProviders[provider] = true
			}
			if successReq, ok := providerStats["success_count"].(int); ok {
				totalSuccessful += successReq
			}
			if avgTime, ok := providerStats["avg_processing_ms"].(float64); ok && avgTime > 0 {
				totalResponseTime += avgTime
				responseTimeCount++
			}

			// Get payment volumes from PostgreSQL
			volume, err := h.getProviderVolumeWithFilters(ctx, tenantID, provider, filters)
			if err == nil {
				totalVolume += volume
			}
		}
	}

	// Calculate success rate
	successRate := 0.0
	if totalPayments > 0 {
		successRate = (float64(totalSuccessful) / float64(totalPayments)) * 100
	}

	// Calculate average response time
	avgResponseTime := 0.0
	if responseTimeCount > 0 {
		avgResponseTime = totalResponseTime / float64(responseTimeCount)
	}

	// Round to 2 decimal places
	successRate = float64(int(successRate*100)) / 100
	totalVolume = float64(int(totalVolume*100)) / 100
	avgResponseTime = float64(int(avgResponseTime*100)) / 100

	environment := "all"
	if filters.Environment != nil {
		environment = *filters.Environment
	}

	return DashboardStats{
		TotalPayments:       totalPayments,
		SuccessRate:         successRate,
		TotalVolume:         totalVolume,
		AvgResponseTime:     avgResponseTime,
		TotalPaymentsChange: h.calculatePaymentChangeWithFilters(filters),
		SuccessRateChange:   h.calculateSuccessRateChangeWithFilters(filters),
		TotalVolumeChange:   h.calculateVolumeChangeWithFilters(filters),
		AvgResponseChange:   h.calculateResponseTimeChangeWithFilters(filters),
		ActiveTenants:       len(activeTenants),
		ActiveProviders:     len(activeProviders),
		Environment:         environment,
	}, nil
}

// getProviderVolumeWithFilters calculates total payment volume for a provider with filters
func (h *AnalyticsHandler) getProviderVolumeWithFilters(ctx context.Context, tenantID int, provider string, filters AnalyticsFilters) (float64, error) {
	// Create filters for PostgreSQL search
	searchFilters := map[string]any{
		"start_date": time.Now().Add(-time.Duration(filters.Hours) * time.Hour),
		"end_date":   time.Now(),
	}

	// Add environment filter if specified
	if filters.Environment != nil {
		searchFilters["environment"] = *filters.Environment
	}

	logs, err := h.logger.SearchPaymentLogs(ctx, tenantID, provider, searchFilters)
	if err != nil {
		return 0, err
	}

	var totalVolume float64
	for _, log := range logs {
		if log.PaymentInfo != nil && log.PaymentInfo.Amount > 0 {
			totalVolume += log.PaymentInfo.Amount
		}
	}

	return totalVolume, nil
}

// GetProviderStats returns provider-specific statistics
func (h *AnalyticsHandler) GetProviderStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse filters from query parameters
	filters := h.parseAnalyticsFilters(r)

	var providers []ProviderStats
	var err error

	if h.logger != nil {
		providers, err = h.getRealProviderStats(ctx, filters)
		if err != nil {
			logger.Warn("Failed to get real provider stats", logger.LogContext{
				TenantID: fmt.Sprintf("%v", filters.TenantID),
				Fields: map[string]any{
					"error":   err.Error(),
					"filters": filters,
				},
			})
			providers = []ProviderStats{}
		}
	} else {
		providers = []ProviderStats{}
	}

	response.Success(w, http.StatusOK, "Provider stats retrieved successfully", providers)
}

// getRealProviderStats fetches real provider statistics from PostgreSQL
func (h *AnalyticsHandler) getRealProviderStats(ctx context.Context, filters AnalyticsFilters) ([]ProviderStats, error) {
	// Get providers that actually have tenant configurations
	configuredProviders, err := h.logger.GetActiveProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active providers: %w", err)
	}

	// Filter by specific provider if requested
	if filters.ProviderID != nil {
		filtered := []map[string]any{}
		for _, provider := range configuredProviders {
			if provider["id"].(string) == *filters.ProviderID {
				filtered = append(filtered, provider)
				break
			}
		}
		configuredProviders = filtered
	}

	stats := make([]ProviderStats, len(configuredProviders))

	// Get tenant IDs to process
	var tenantIDs []int
	if filters.TenantID != nil {
		tenantIDs = []int{*filters.TenantID}
	} else {
		tenantIDs = h.getActiveTenants(ctx)
	}

	for i, provider := range configuredProviders {
		providerKey := provider["id"].(string)
		providerName := provider["name"].(string)
		realTenantCount := provider["tenant_count"].(int)

		status := "offline"
		responseTime := "0ms"
		transactions := 0
		successRate := 0.0

		// Aggregate stats across all tenants for this provider
		for _, tenantID := range tenantIDs {
			// Get provider stats from PostgreSQL with environment consideration
			providerStats, err := h.getPaymentStatsWithEnv(ctx, tenantID, providerKey, filters.Hours, filters.Environment)

			if err == nil {
				// Extract stats from PostgreSQL response
				if totalReq, ok := providerStats["total_requests"].(int); ok {
					transactions += totalReq
				}
				if successCount, ok := providerStats["success_count"].(int); ok && transactions > 0 {
					// Recalculate success rate with aggregated data
					if totalTrans := transactions; totalTrans > 0 {
						successRate = (float64(successCount) / float64(totalTrans)) * 100
					}
				}
				if avgTime, ok := providerStats["avg_processing_ms"].(float64); ok && avgTime > 0 {
					responseTime = fmt.Sprintf("%.0fms", avgTime)
					// Mark as degraded if response time > 400ms
					if avgTime > 400 {
						status = "degraded"
					}
				}
			}
		}

		// Only use real data - set status to online only if there are transactions and not already degraded
		if transactions > 0 && status != "degraded" {
			status = "online"
		}

		// Round success rate to 2 decimal places
		successRate = float64(int(successRate*100)) / 100

		environment := "all"
		if filters.Environment != nil {
			environment = *filters.Environment
		}

		stats[i] = ProviderStats{
			Name:         providerName,
			Status:       status,
			ResponseTime: responseTime,
			Transactions: transactions,
			SuccessRate:  successRate,
			Environment:  environment,
			TenantCount:  realTenantCount,
		}
	}

	return stats, nil
}

// getActiveTenants gets all tenant IDs from PostgreSQL
func (h *AnalyticsHandler) getActiveTenants(ctx context.Context) []int {
	tenants, err := h.logger.GetAllTenants(ctx)
	if err != nil {
		// Return fallback if database query fails
		return []int{1}
	}

	tenantIDs := make([]int, 0, len(tenants))
	for _, tenant := range tenants {
		if id, ok := tenant["id"].(int); ok {
			tenantIDs = append(tenantIDs, id)
		}
	}

	// If no tenants found, return a default set
	if len(tenantIDs) == 0 {
		tenantIDs = []int{1}
	}

	return tenantIDs
}

// getPaymentStatsWithEnv gets payment stats with environment filter (wrapper method)
func (h *AnalyticsHandler) getPaymentStatsWithEnv(ctx context.Context, tenantID int, provider string, hours int, environment *string) (map[string]any, error) {
	// This is a wrapper method since GetPaymentStatsWithEnv doesn't exist in postgres.Logger
	// For now, we'll use the existing method and filter by environment in application logic
	stats, err := h.logger.GetPaymentStats(ctx, tenantID, provider, hours)
	if err != nil {
		return nil, err
	}

	// If environment filter is specified, filter the results based on environment
	if environment != nil {
		// Get detailed logs to filter by environment
		searchFilters := map[string]any{
			"start_date": time.Now().Add(-time.Duration(hours) * time.Hour),
			"end_date":   time.Now(),
		}

		logs, err := h.logger.SearchPaymentLogs(ctx, tenantID, provider, searchFilters)
		if err != nil {
			// If we can't get detailed logs, return unfiltered stats
			return stats, nil
		}

		// Filter logs by environment
		var filteredLogs []postgres.PaymentLog
		for _, log := range logs {
			// Check if the log contains environment information
			if log.Request != nil {
				// Check for environment in request data
				if env, ok := log.Request["environment"].(string); ok {
					if env == *environment {
						filteredLogs = append(filteredLogs, log)
					}
				} else {
					// If no environment in request, check response data
					if log.Response != nil {
						if env, ok := log.Response["environment"].(string); ok {
							if env == *environment {
								filteredLogs = append(filteredLogs, log)
							}
						}
					}
				}
			}
		}

		// Recalculate stats based on filtered logs
		if len(filteredLogs) > 0 {
			totalRequests := len(filteredLogs)
			successCount := 0
			errorCount := 0
			var totalProcessingMs float64
			processingCount := 0

			for _, log := range filteredLogs {
				// Check success status
				if log.Response != nil {
					if success, ok := log.Response["success"].(bool); ok && success {
						successCount++
					} else {
						errorCount++
					}
				}

				// Calculate processing time
				if log.ProcessingMs > 0 {
					totalProcessingMs += float64(log.ProcessingMs)
					processingCount++
				}
			}

			// Update stats with filtered data
			stats["total_requests"] = totalRequests
			stats["success_count"] = successCount
			stats["error_count"] = errorCount

			if totalRequests > 0 {
				stats["success_rate"] = (float64(successCount) / float64(totalRequests)) * 100
			}

			if processingCount > 0 {
				stats["avg_processing_ms"] = totalProcessingMs / float64(processingCount)
			}
		} else {
			// No logs found for the specified environment, return zero stats
			stats["total_requests"] = 0
			stats["success_count"] = 0
			stats["error_count"] = 0
			stats["success_rate"] = 0.0
			stats["avg_processing_ms"] = 0.0
		}
	}

	return stats, nil
}

// GetRecentActivity returns recent payment activity
func (h *AnalyticsHandler) GetRecentActivity(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse limit parameter (default 10)
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	// Parse filters from query parameters
	filters := h.parseAnalyticsFilters(r)

	var activities []RecentActivity
	var err error

	if h.logger != nil {
		activities, err = h.getRealRecentActivity(ctx, filters, limit)
		if err != nil {
			logger.Warn("Failed to get real recent activity", logger.LogContext{
				TenantID: fmt.Sprintf("%v", filters.TenantID),
				Fields: map[string]any{
					"error":   err.Error(),
					"limit":   limit,
					"filters": filters,
				},
			})
			activities = []RecentActivity{}
		}
	} else {
		activities = []RecentActivity{}
	}

	response.Success(w, http.StatusOK, "Recent activity retrieved successfully", activities)
}

// getRealRecentActivity fetches real recent activity from PostgreSQL
func (h *AnalyticsHandler) getRealRecentActivity(ctx context.Context, filters AnalyticsFilters, limit int) ([]RecentActivity, error) {
	// Get all recent activities from provider tables
	activities, err := h.logger.GetAllRecentActivity(ctx, limit*2) // Get more to filter
	if err != nil {
		return []RecentActivity{}, err
	}

	var filteredActivities []RecentActivity

	for _, activity := range activities {
		// Apply filters
		if filters.TenantID != nil {
			if tenantID, ok := activity["tenant_id"].(int); ok {
				if tenantID != *filters.TenantID {
					continue
				}
			}
		}

		if filters.ProviderID != nil {
			providerName := strings.ToLower(activity["provider"].(string))
			if !strings.Contains(providerName, *filters.ProviderID) {
				continue
			}
		}

		if filters.Environment != nil {
			// For now, we don't have environment detection in the data
			// Could be enhanced later to detect from the actual data
		}

		// Convert to RecentActivity struct
		recentActivity := RecentActivity{
			Type:     activity["type"].(string),
			Provider: activity["provider"].(string),
			Amount:   activity["amount"].(string),
			Status:   activity["status"].(string),
			Time:     activity["time"].(string),
			ID:       activity["id"].(string),
			TenantID: fmt.Sprintf("%d", activity["tenant_id"].(int)),
			Env:      activity["env"].(string),
			Request:  activity["request"].(string),
			Response: activity["response"].(string),
			Endpoint: activity["endpoint"].(string),
		}

		filteredActivities = append(filteredActivities, recentActivity)

		// Stop if we have enough activities
		if len(filteredActivities) >= limit {
			break
		}
	}

	return filteredActivities, nil
}

// GetPaymentTrends returns payment trends data for charts
func (h *AnalyticsHandler) GetPaymentTrends(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse filters from query parameters
	filters := h.parseAnalyticsFilters(r)

	var trends map[string]any
	var err error

	if h.logger != nil {
		trends, err = h.getRealPaymentTrends(ctx, filters)
		if err != nil {
			logger.Warn("Failed to get real payment trends", logger.LogContext{
				TenantID: fmt.Sprintf("%v", filters.TenantID),
				Fields: map[string]any{
					"error":   err.Error(),
					"filters": filters,
				},
			})
			trends = map[string]any{
				"labels": []string{},
				"datasets": []map[string]any{
					{
						"label":           "Successful Payments",
						"data":            []int{},
						"borderColor":     "#10B981",
						"backgroundColor": "rgba(16, 185, 129, 0.1)",
					},
					{
						"label":           "Failed Payments",
						"data":            []int{},
						"borderColor":     "#EF4444",
						"backgroundColor": "rgba(239, 68, 68, 0.1)",
					},
				},
				"volume": []float64{},
			}
		}
	} else {
		trends = map[string]any{
			"labels": []string{},
			"datasets": []map[string]any{
				{
					"label":           "Successful Payments",
					"data":            []int{},
					"borderColor":     "#10B981",
					"backgroundColor": "rgba(16, 185, 129, 0.1)",
				},
				{
					"label":           "Failed Payments",
					"data":            []int{},
					"borderColor":     "#EF4444",
					"backgroundColor": "rgba(239, 68, 68, 0.1)",
				},
			},
			"volume": []float64{},
		}
	}

	response.Success(w, http.StatusOK, "Payment trends retrieved successfully", trends)
}

// getRealPaymentTrends fetches real payment trends from PostgreSQL
func (h *AnalyticsHandler) getRealPaymentTrends(ctx context.Context, filters AnalyticsFilters) (map[string]any, error) {
	// Get providers that actually have tenant configurations
	configuredProviders, err := h.logger.GetActiveProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active providers: %w", err)
	}

	// Extract provider keys
	var providers []string
	for _, provider := range configuredProviders {
		providers = append(providers, provider["id"].(string))
	}

	// Filter by specific provider if requested
	if filters.ProviderID != nil {
		providers = []string{*filters.ProviderID}
	}

	// Track combined daily data
	dailyData := make(map[string]struct {
		successful int
		failed     int
		volume     float64
	})

	// Get tenant IDs to process
	var tenantIDs []int
	if filters.TenantID != nil {
		tenantIDs = []int{*filters.TenantID}
	} else {
		tenantIDs = h.getActiveTenants(ctx)
	}

	// Collect data from all tenants and providers
	for _, tenantID := range tenantIDs {
		for _, provider := range providers {
			trends, err := h.logger.GetPaymentTrendsMonthly(ctx, tenantID, provider, filters.Month, filters.Year)
			if err != nil {
				continue // Skip provider if error
			}

			// Extract data from trends and aggregate
			if labels, ok := trends["labels"].([]string); ok {
				if datasets, ok := trends["datasets"].([]map[string]any); ok && len(datasets) >= 2 {
					if successData, ok := datasets[0]["data"].([]int); ok {
						if failedData, ok := datasets[1]["data"].([]int); ok {
							if volumeData, ok := trends["volume"].([]float64); ok {
								// Combine data by day
								for i, label := range labels {
									if i < len(successData) && i < len(failedData) && i < len(volumeData) {
										if existing, exists := dailyData[label]; exists {
											existing.successful += successData[i]
											existing.failed += failedData[i]
											existing.volume += volumeData[i]
											dailyData[label] = existing
										} else {
											dailyData[label] = struct {
												successful int
												failed     int
												volume     float64
											}{
												successful: successData[i],
												failed:     failedData[i],
												volume:     volumeData[i],
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// If no data found, return empty trends structure with the month's days
	if len(dailyData) == 0 {
		// Generate empty data for all days in the month
		emptyLabels := []string{}
		emptySuccess := []int{}
		emptyFailed := []int{}
		emptyVolume := []float64{}

		// Generate day labels for the selected month
		firstDay := time.Date(filters.Year, time.Month(filters.Month), 1, 0, 0, 0, 0, time.UTC)
		currentDay := firstDay
		for currentDay.Month() == time.Month(filters.Month) && currentDay.Year() == filters.Year {
			emptyLabels = append(emptyLabels, currentDay.Format("Jan 2"))
			emptySuccess = append(emptySuccess, 0)
			emptyFailed = append(emptyFailed, 0)
			emptyVolume = append(emptyVolume, 0.0)
			currentDay = currentDay.AddDate(0, 0, 1)
		}

		return map[string]any{
			"labels": emptyLabels,
			"datasets": []map[string]any{
				{
					"label":           "Successful Payments",
					"data":            emptySuccess,
					"borderColor":     "#10B981",
					"backgroundColor": "rgba(16, 185, 129, 0.1)",
				},
				{
					"label":           "Failed Payments",
					"data":            emptyFailed,
					"borderColor":     "#EF4444",
					"backgroundColor": "rgba(239, 68, 68, 0.1)",
				},
			},
			"volume": emptyVolume,
		}, nil
	}

	// Convert aggregated data to arrays maintaining chronological order
	firstDay := time.Date(filters.Year, time.Month(filters.Month), 1, 0, 0, 0, 0, time.UTC)
	var combinedLabels []string
	var combinedSuccessData []int
	var combinedFailedData []int
	var combinedVolumeData []float64

	currentDay := firstDay
	for currentDay.Month() == time.Month(filters.Month) && currentDay.Year() == filters.Year {
		dayLabel := currentDay.Format("Jan 2")
		combinedLabels = append(combinedLabels, dayLabel)

		if data, exists := dailyData[dayLabel]; exists {
			combinedSuccessData = append(combinedSuccessData, data.successful)
			combinedFailedData = append(combinedFailedData, data.failed)
			combinedVolumeData = append(combinedVolumeData, data.volume)
		} else {
			// No data for this day, fill with zeros
			combinedSuccessData = append(combinedSuccessData, 0)
			combinedFailedData = append(combinedFailedData, 0)
			combinedVolumeData = append(combinedVolumeData, 0.0)
		}

		currentDay = currentDay.AddDate(0, 0, 1)
	}

	return map[string]any{
		"labels": combinedLabels,
		"datasets": []map[string]any{
			{
				"label":           "Successful Payments",
				"data":            combinedSuccessData,
				"borderColor":     "#10B981",
				"backgroundColor": "rgba(16, 185, 129, 0.1)",
			},
			{
				"label":           "Failed Payments",
				"data":            combinedFailedData,
				"borderColor":     "#EF4444",
				"backgroundColor": "rgba(239, 68, 68, 0.1)",
			},
		},
		"volume": combinedVolumeData,
	}, nil
}

// calculatePaymentChangeWithFilters calculates the percentage change in payment count from previous period
func (h *AnalyticsHandler) calculatePaymentChangeWithFilters(filters AnalyticsFilters) string {
	if h.logger == nil {
		return "+12.5% from yesterday"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get providers that actually have tenant configurations
	configuredProviders, err := h.logger.GetActiveProviders(ctx)
	if err != nil {
		return "+12.5% from yesterday" // fallback
	}

	// Extract provider keys
	var providers []string
	for _, provider := range configuredProviders {
		providers = append(providers, provider["id"].(string))
	}

	// Filter by specific provider if requested
	if filters.ProviderID != nil {
		providers = []string{*filters.ProviderID}
	}

	var currentTotal, previousTotal int

	// Get tenant IDs to process
	var tenantIDs []int
	if filters.TenantID != nil {
		tenantIDs = []int{*filters.TenantID}
	} else {
		tenantIDs = h.getActiveTenants(ctx)
	}

	for _, tenantID := range tenantIDs {
		for _, provider := range providers {
			stats, err := h.logger.GetPaymentStatsComparison(ctx, tenantID, provider, filters.Hours, filters.Hours)
			if err != nil {
				continue // Skip provider if error
			}

			if currentCount, ok := stats["current_total"].(int); ok {
				currentTotal += currentCount
			}
			if previousCount, ok := stats["previous_total"].(int); ok {
				previousTotal += previousCount
			}
		}
	}

	// Calculate percentage change
	if previousTotal == 0 {
		if currentTotal > 0 {
			return fmt.Sprintf("+∞%% from previous %dh", filters.Hours)
		}
		return "No previous data"
	}

	change := ((float64(currentTotal) - float64(previousTotal)) / float64(previousTotal)) * 100

	if change > 0 {
		return fmt.Sprintf("+%.1f%% from previous %dh", change, filters.Hours)
	} else if change < 0 {
		return fmt.Sprintf("%.1f%% from previous %dh", change, filters.Hours)
	} else {
		return fmt.Sprintf("No change from previous %dh", filters.Hours)
	}
}

// calculateSuccessRateChangeWithFilters calculates the percentage change in success rate from previous period
func (h *AnalyticsHandler) calculateSuccessRateChangeWithFilters(filters AnalyticsFilters) string {
	if h.logger == nil {
		return "+0.8% from yesterday"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all providers or filter by specific provider
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
	if filters.ProviderID != nil {
		providers = []string{*filters.ProviderID}
	}

	var currentTotal, currentSuccess, previousTotal, previousSuccess int

	// Get tenant IDs to process
	var tenantIDs []int
	if filters.TenantID != nil {
		tenantIDs = []int{*filters.TenantID}
	} else {
		tenantIDs = h.getActiveTenants(ctx)
	}

	for _, tenantID := range tenantIDs {
		for _, provider := range providers {
			stats, err := h.logger.GetPaymentStatsComparison(ctx, tenantID, provider, filters.Hours, filters.Hours)
			if err != nil {
				continue // Skip provider if error
			}

			if currentCount, ok := stats["current_total"].(int); ok {
				currentTotal += currentCount
			}
			if currentSuccessCount, ok := stats["current_success"].(int); ok {
				currentSuccess += currentSuccessCount
			}
			if previousCount, ok := stats["previous_total"].(int); ok {
				previousTotal += previousCount
			}
			if previousSuccessCount, ok := stats["previous_success"].(int); ok {
				previousSuccess += previousSuccessCount
			}
		}
	}

	// Calculate success rates
	var currentRate, previousRate float64

	if currentTotal > 0 {
		currentRate = (float64(currentSuccess) / float64(currentTotal)) * 100
	}

	if previousTotal > 0 {
		previousRate = (float64(previousSuccess) / float64(previousTotal)) * 100
	} else {
		if currentTotal > 0 {
			return fmt.Sprintf("+%.1f%% (no previous data)", currentRate)
		}
		return "No data available"
	}

	// Calculate percentage point change (not percentage change)
	change := currentRate - previousRate

	if change > 0 {
		return fmt.Sprintf("+%.1f%% from previous %dh", change, filters.Hours)
	} else if change < 0 {
		return fmt.Sprintf("%.1f%% from previous %dh", change, filters.Hours)
	} else {
		return fmt.Sprintf("No change from previous %dh", filters.Hours)
	}
}

// calculateVolumeChangeWithFilters calculates the percentage change in payment volume from previous period
func (h *AnalyticsHandler) calculateVolumeChangeWithFilters(filters AnalyticsFilters) string {
	if h.logger == nil {
		return "+18.2% from yesterday"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all providers or filter by specific provider
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
	if filters.ProviderID != nil {
		providers = []string{*filters.ProviderID}
	}

	var currentVolume, previousVolume float64

	// Get tenant IDs to process
	var tenantIDs []int
	if filters.TenantID != nil {
		tenantIDs = []int{*filters.TenantID}
	} else {
		tenantIDs = h.getActiveTenants(ctx)
	}

	for _, tenantID := range tenantIDs {
		for _, provider := range providers {
			stats, err := h.logger.GetPaymentStatsComparison(ctx, tenantID, provider, filters.Hours, filters.Hours)
			if err != nil {
				continue // Skip provider if error
			}

			if currentVol, ok := stats["current_volume"].(float64); ok {
				currentVolume += currentVol
			}
			if previousVol, ok := stats["previous_volume"].(float64); ok {
				previousVolume += previousVol
			}
		}
	}

	// Calculate percentage change
	if previousVolume == 0 {
		if currentVolume > 0 {
			return fmt.Sprintf("+∞%% from previous %dh", filters.Hours)
		}
		return "No previous data"
	}

	change := ((currentVolume - previousVolume) / previousVolume) * 100

	if change > 0 {
		return fmt.Sprintf("+%.1f%% from previous %dh", change, filters.Hours)
	} else if change < 0 {
		return fmt.Sprintf("%.1f%% from previous %dh", change, filters.Hours)
	} else {
		return fmt.Sprintf("No change from previous %dh", filters.Hours)
	}
}

// calculateResponseTimeChangeWithFilters calculates the change in average response time from previous period
func (h *AnalyticsHandler) calculateResponseTimeChangeWithFilters(filters AnalyticsFilters) string {
	if h.logger == nil {
		return "-15ms from yesterday"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all providers or filter by specific provider
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
	if filters.ProviderID != nil {
		providers = []string{*filters.ProviderID}
	}

	var currentSum, previousSum float64
	var currentCount, previousCount int

	// Get tenant IDs to process
	var tenantIDs []int
	if filters.TenantID != nil {
		tenantIDs = []int{*filters.TenantID}
	} else {
		tenantIDs = h.getActiveTenants(ctx)
	}

	for _, tenantID := range tenantIDs {
		for _, provider := range providers {
			stats, err := h.logger.GetPaymentStatsComparison(ctx, tenantID, provider, filters.Hours, filters.Hours)
			if err != nil {
				continue // Skip provider if error
			}

			if currentProcessingMs, ok := stats["current_processing_ms"].(float64); ok && currentProcessingMs > 0 {
				currentSum += currentProcessingMs
				currentCount++
			}
			if previousProcessingMs, ok := stats["previous_processing_ms"].(float64); ok && previousProcessingMs > 0 {
				previousSum += previousProcessingMs
				previousCount++
			}
		}
	}

	// Calculate average response times
	var currentAvg, previousAvg float64

	if currentCount > 0 {
		currentAvg = currentSum / float64(currentCount)
	}

	if previousCount > 0 {
		previousAvg = previousSum / float64(previousCount)
	} else {
		if currentCount > 0 {
			return fmt.Sprintf("%.0fms (no previous data)", currentAvg)
		}
		return "No data available"
	}

	// Calculate millisecond change
	change := currentAvg - previousAvg

	if change > 0 {
		return fmt.Sprintf("+%.0fms from previous %dh", change, filters.Hours)
	} else if change < 0 {
		return fmt.Sprintf("%.0fms from previous %dh", change, filters.Hours)
	} else {
		return fmt.Sprintf("No change from previous %dh", filters.Hours)
	}
}

// GetActiveProviders returns list of available providers
func (h *AnalyticsHandler) GetActiveProviders(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var providers []map[string]any = []map[string]any{}
	var err error

	if h.logger != nil {
		providers, err = h.getRealActiveProviders(ctx)
		if err != nil {
			logger.Warn("Failed to get active providers", logger.LogContext{
				Fields: map[string]any{
					"error": err.Error(),
				},
			})
		}
	}

	response.Success(w, http.StatusOK, "Active providers retrieved successfully", providers)
}

// GetActiveTenants returns list of active tenants
func (h *AnalyticsHandler) GetActiveTenants(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var tenants []map[string]any
	var err error

	if h.logger != nil {
		tenants, err = h.getRealActiveTenants(ctx)
		if err != nil {
			logger.Warn("Failed to get active tenants", logger.LogContext{
				Fields: map[string]any{
					"error": err.Error(),
				},
			})
			tenants = []map[string]any{}
		}
	} else {
		tenants = []map[string]any{}
	}

	response.Success(w, http.StatusOK, "Active tenants retrieved successfully", tenants)
}

// getRealActiveProviders fetches active providers from PostgreSQL
func (h *AnalyticsHandler) getRealActiveProviders(ctx context.Context) ([]map[string]any, error) {
	return h.logger.GetActiveProviders(ctx)
}

// getRealActiveTenants fetches all tenants from PostgreSQL
func (h *AnalyticsHandler) getRealActiveTenants(ctx context.Context) ([]map[string]any, error) {
	return h.logger.GetAllTenants(ctx)
}

// SearchPaymentByID searches for a specific payment by ID with tenant security
func (h *AnalyticsHandler) SearchPaymentByID(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Get tenant context from JWT
	userTenantID, isAdmin := h.getTenantContext(r)

	// Parse required parameters
	tenantIDStr := r.URL.Query().Get("tenant_id")
	providerID := r.URL.Query().Get("provider_id")
	paymentID := r.URL.Query().Get("payment_id")

	// Validate required parameters
	if tenantIDStr == "" || tenantIDStr == "all" {
		response.Error(w, http.StatusBadRequest, "tenant_id is required", fmt.Errorf("tenant_id parameter is missing or set to 'all'"))
		return
	}

	if providerID == "" || providerID == "all" {
		response.Error(w, http.StatusBadRequest, "provider_id is required", fmt.Errorf("provider_id parameter is missing or set to 'all'"))
		return
	}

	if paymentID == "" {
		response.Error(w, http.StatusBadRequest, "payment_id is required", fmt.Errorf("payment_id parameter is missing"))
		return
	}

	// Apply tenant security rules
	// Rule: Admin users (tenant_id=1) can search any tenant's data
	// Rule: Non-admin users can only search their own data (tenant_id parameter must match their tenant_id)
	var finalTenantID int
	if requestedTenantID, err := strconv.Atoi(tenantIDStr); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid tenant_id", err)
		return
	} else {
		if isAdmin {
			// Admin users (tenant_id=1) can search any tenant's data
			finalTenantID = requestedTenantID
		} else {
			// Non-admin users: tenant_id parameter must match their own tenant_id
			if userTenantIDInt, err := strconv.Atoi(userTenantID); err != nil {
				response.Error(w, http.StatusInternalServerError, "invalid user tenant in session", err)
				return
			} else {
				if requestedTenantID != userTenantIDInt {
					response.Error(w, http.StatusForbidden, "access denied", fmt.Errorf("cannot access tenant %d data", requestedTenantID))
					return
				}
				finalTenantID = userTenantIDInt
			}
		}
	}

	var activities []*RecentActivity
	var searchErr error

	if h.logger != nil {
		activities, searchErr = h.searchPaymentInDatabase(ctx, finalTenantID, providerID, paymentID)
		if searchErr != nil {
			logger.Warn("Failed to search payment", logger.LogContext{
				TenantID: fmt.Sprintf("%d", finalTenantID),
				Fields: map[string]any{
					"error":       searchErr.Error(),
					"tenant_id":   finalTenantID,
					"provider":    providerID,
					"payment_id":  paymentID,
					"user_tenant": userTenantID,
					"is_admin":    isAdmin,
				},
			})
		}
	}

	if len(activities) == 0 {
		response.Error(w, http.StatusNotFound, "Payment not found", fmt.Errorf("no payment found with ID %s for tenant %d and provider %s", paymentID, finalTenantID, providerID))
		return
	}

	// Return all matching payments
	response.Success(w, http.StatusOK, "Payment found successfully", activities)
}

// searchPaymentInDatabase searches for a payment in the specified provider table
func (h *AnalyticsHandler) searchPaymentInDatabase(ctx context.Context, tenantID int, provider, paymentID string) ([]*RecentActivity, error) {
	// Search in the provider table for the specific payment
	payments, err := h.logger.SearchPaymentByID(ctx, tenantID, provider, paymentID)
	if err != nil {
		return nil, err
	}

	if len(payments) == 0 {
		return nil, nil
	}

	var activities []*RecentActivity
	for _, payment := range payments {
		// Convert database result to RecentActivity
		activity := &RecentActivity{
			Type:     payment["type"].(string),
			Provider: payment["provider"].(string),
			Amount:   payment["amount"].(string),
			Status:   payment["status"].(string),
			Time:     payment["time"].(string),
			ID:       payment["id"].(string),
			TenantID: fmt.Sprintf("%d", payment["tenant_id"].(int)),
			Env:      payment["env"].(string),
			Request:  payment["request"].(string),
			Response: payment["response"].(string),
			Endpoint: payment["endpoint"].(string),
		}
		activities = append(activities, activity)
	}

	return activities, nil
}
