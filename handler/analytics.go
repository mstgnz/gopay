package handler

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/infra/postgres"
	"github.com/mstgnz/gopay/infra/response"
)

// AnalyticsHandler handles analytics related HTTP requests
type AnalyticsHandler struct {
	logger *postgres.Logger
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
}

// AnalyticsFilters represents the filters for analytics queries
type AnalyticsFilters struct {
	TenantID    *int    `json:"tenantId,omitempty"`
	ProviderID  *string `json:"providerId,omitempty"`
	Environment *string `json:"environment,omitempty"`
	Hours       int     `json:"hours"`
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
			// Log error but fallback to demo data
			logger.Warn("Failed to get real dashboard stats", logger.LogContext{
				TenantID: fmt.Sprintf("%v", filters.TenantID),
				Fields: map[string]any{
					"error":   err.Error(),
					"filters": filters,
				},
			})
			stats = h.generateDashboardStats(ctx, filters)
		}
	} else {
		stats = h.generateDashboardStats(ctx, filters)
	}

	response.Success(w, http.StatusOK, "Dashboard stats retrieved successfully", stats)
}

// parseAnalyticsFilters parses query parameters into analytics filters
func (h *AnalyticsHandler) parseAnalyticsFilters(r *http.Request) AnalyticsFilters {
	filters := AnalyticsFilters{
		Hours: 24, // default
	}

	// Parse hours
	if hoursStr := r.URL.Query().Get("hours"); hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 && h <= 168 {
			filters.Hours = h
		}
	}

	// Parse tenant_id
	if tenantStr := r.URL.Query().Get("tenant_id"); tenantStr != "" && tenantStr != "all" {
		if tenantID, err := strconv.Atoi(tenantStr); err == nil {
			filters.TenantID = &tenantID
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
	// Get all providers or filter by specific provider
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
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
		tenantIDs = h.getActiveTenants(ctx, filters.Hours)
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
			providers = h.generateProviderStats(ctx, filters)
		}
	} else {
		providers = h.generateProviderStats(ctx, filters)
	}

	response.Success(w, http.StatusOK, "Provider stats retrieved successfully", providers)
}

// getRealProviderStats fetches real provider statistics from PostgreSQL
func (h *AnalyticsHandler) getRealProviderStats(ctx context.Context, filters AnalyticsFilters) ([]ProviderStats, error) {
	providers := []string{"İyzico", "Stripe", "OzanPay", "Paycell", "Papara", "Nkolay", "PayTR", "PayU"}
	providerKeys := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}

	// Filter by specific provider if requested
	if filters.ProviderID != nil {
		for i, key := range providerKeys {
			if key == *filters.ProviderID {
				providers = []string{providers[i]}
				providerKeys = []string{key}
				break
			}
		}
	}

	stats := make([]ProviderStats, len(providers))

	// Get tenant IDs to process
	var tenantIDs []int
	if filters.TenantID != nil {
		tenantIDs = []int{*filters.TenantID}
	} else {
		// Get all active tenant IDs (fallback if method doesn't exist)
		tenantIDs = h.getActiveTenants(ctx, filters.Hours)
	}

	for i, providerKey := range providerKeys {
		status := "online"
		responseTime := "150ms"
		transactions := 0
		successRate := 95.0
		tenantCount := 0

		// Aggregate stats across all tenants for this provider
		for _, tenantID := range tenantIDs {
			// Get provider stats from PostgreSQL with environment consideration
			providerStats, err := h.getPaymentStatsWithEnv(ctx, tenantID, providerKey, filters.Hours, filters.Environment)

			if err == nil {
				tenantCount++
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

		// Use fallback values if no data
		if transactions == 0 {
			transactions = 100 + rand.Intn(500)
			responseTime = fmt.Sprintf("%dms", 100+rand.Intn(200))
			if rand.Float32() < 0.1 {
				status = "degraded"
				responseTime = fmt.Sprintf("%dms", 400+rand.Intn(200))
			}
		}

		// Round success rate to 2 decimal places
		successRate = float64(int(successRate*100)) / 100

		environment := "all"
		if filters.Environment != nil {
			environment = *filters.Environment
		}

		stats[i] = ProviderStats{
			Name:         providers[i],
			Status:       status,
			ResponseTime: responseTime,
			Transactions: transactions,
			SuccessRate:  successRate,
			Environment:  environment,
			TenantCount:  tenantCount,
		}
	}

	return stats, nil
}

// getActiveTenants gets active tenant IDs from PostgreSQL (wrapper method)
func (h *AnalyticsHandler) getActiveTenants(ctx context.Context, hours int) []int {
	// This is a wrapper method since GetActiveTenants doesn't exist in postgres.Logger
	// We'll get tenants from existing payment stats
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
	tenantMap := make(map[int]bool)

	// Try different tenant IDs (you might want to get this from a tenants table)
	for tenantID := 1; tenantID <= 100; tenantID++ {
		for _, provider := range providers {
			stats, err := h.logger.GetPaymentStats(ctx, tenantID, provider, hours)
			if err == nil {
				if totalReq, ok := stats["total_requests"].(int); ok && totalReq > 0 {
					tenantMap[tenantID] = true
					break // Found activity for this tenant
				}
			}
		}
	}

	// Convert map to slice
	tenantIDs := make([]int, 0, len(tenantMap))
	for tenantID := range tenantMap {
		tenantIDs = append(tenantIDs, tenantID)
	}

	// If no tenants found, return a default set
	if len(tenantIDs) == 0 {
		tenantIDs = []int{0} // legacy tenant
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

	// If environment filter is specified, we might need to filter the results
	// This would ideally be done at the database level, but for now we'll return as-is
	// The environment filtering would need to be implemented in the PostgreSQL logger methods
	if environment != nil {
		// TODO: Add environment-specific filtering logic here
		// For now, we'll assume the data includes both environments
		// In a real implementation, you'd modify the SQL queries to filter by environment
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
			activities = h.generateRecentActivity(ctx, filters, limit)
		}
	} else {
		activities = h.generateRecentActivity(ctx, filters, limit)
	}

	response.Success(w, http.StatusOK, "Recent activity retrieved successfully", activities)
}

// getRealRecentActivity fetches real recent activity from PostgreSQL
func (h *AnalyticsHandler) getRealRecentActivity(ctx context.Context, filters AnalyticsFilters, limit int) ([]RecentActivity, error) {
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara"}
	if filters.ProviderID != nil {
		providers = []string{*filters.ProviderID}
	}

	var allActivities []RecentActivity

	// Get tenant IDs to process
	var tenantIDs []int
	if filters.TenantID != nil {
		tenantIDs = []int{*filters.TenantID}
	} else {
		tenantIDs = h.getActiveTenants(ctx, 2) // last 2 hours
	}

	for _, tenantID := range tenantIDs {
		for _, provider := range providers {
			// Create filters for recent payments
			searchFilters := map[string]any{
				"start_date": time.Now().Add(-2 * time.Hour), // Last 2 hours
				"end_date":   time.Now(),
			}

			// Add environment filter if specified
			if filters.Environment != nil {
				searchFilters["environment"] = *filters.Environment
			}

			logs, err := h.logger.SearchPaymentLogs(ctx, tenantID, provider, searchFilters)
			if err != nil {
				continue // Skip provider if error
			}

			// Convert logs to activities (take first few)
			for _, log := range logs {
				if len(allActivities) >= limit {
					break // Stop if we have enough activities
				}

				activityType := "payment"
				if log.Endpoint != "" && (log.Endpoint == "/refund" || log.Method == "refund") {
					activityType = "refund"
				}

				status := "success"
				if log.Error != nil && log.Error.Code != "" {
					status = "failed"
				} else if activityType == "refund" {
					status = "processed"
				}

				amount := "₺100.00" // Default
				if log.PaymentInfo != nil && log.PaymentInfo.Amount > 0 {
					amount = fmt.Sprintf("₺%.2f", log.PaymentInfo.Amount)
				}

				// Calculate time ago
				timeAgo := time.Since(log.Timestamp)
				timeStr := fmt.Sprintf("%.0f min ago", timeAgo.Minutes())
				if timeAgo.Hours() >= 1 {
					timeStr = fmt.Sprintf("%.0f hours ago", timeAgo.Hours())
				}

				paymentID := "pay_unknown"
				if log.PaymentInfo != nil && log.PaymentInfo.PaymentID != "" {
					paymentID = log.PaymentInfo.PaymentID
				}

				environment := "production" // default
				if filters.Environment != nil {
					environment = *filters.Environment
				}

				allActivities = append(allActivities, RecentActivity{
					Type:     activityType,
					Provider: provider,
					Amount:   amount,
					Status:   status,
					Time:     timeStr,
					ID:       paymentID,
					TenantID: fmt.Sprintf("%d", tenantID),
					Env:      environment,
				})
			}

			if len(allActivities) >= limit {
				break // Stop if we have enough activities
			}
		}

		if len(allActivities) >= limit {
			break // Stop if we have enough activities
		}
	}

	// If no real data, return fallback
	if len(allActivities) == 0 {
		return h.generateRecentActivity(ctx, filters, limit), nil
	}

	// Limit results
	if len(allActivities) > limit {
		allActivities = allActivities[:limit]
	}

	return allActivities, nil
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
			trends = h.generatePaymentTrends(ctx, filters)
		}
	} else {
		trends = h.generatePaymentTrends(ctx, filters)
	}

	response.Success(w, http.StatusOK, "Payment trends retrieved successfully", trends)
}

// getRealPaymentTrends fetches real payment trends from PostgreSQL
func (h *AnalyticsHandler) getRealPaymentTrends(ctx context.Context, filters AnalyticsFilters) (map[string]any, error) {
	// Get all providers or filter by specific provider
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
	if filters.ProviderID != nil {
		providers = []string{*filters.ProviderID}
	}

	combinedLabels := make([]string, 0)
	combinedSuccessData := make([]int, 0)
	combinedFailedData := make([]int, 0)
	combinedVolumeData := make([]float64, 0)

	// Track which hours we've seen
	hourlyData := make(map[string]struct {
		successful int
		failed     int
		volume     float64
	})

	// Get tenant IDs to process
	var tenantIDs []int
	if filters.TenantID != nil {
		tenantIDs = []int{*filters.TenantID}
	} else {
		tenantIDs = h.getActiveTenants(ctx, filters.Hours)
	}

	for _, tenantID := range tenantIDs {
		for _, provider := range providers {
			trends, err := h.logger.GetPaymentTrends(ctx, tenantID, provider, filters.Hours)
			if err != nil {
				continue // Skip provider if error
			}

			// Extract data from trends
			if labels, ok := trends["labels"].([]string); ok {
				if datasets, ok := trends["datasets"].([]map[string]any); ok && len(datasets) >= 2 {
					if successData, ok := datasets[0]["data"].([]int); ok {
						if failedData, ok := datasets[1]["data"].([]int); ok {
							if volumeData, ok := trends["volume"].([]float64); ok {
								// Combine data by hour
								for i, label := range labels {
									if i < len(successData) && i < len(failedData) && i < len(volumeData) {
										if existing, exists := hourlyData[label]; exists {
											existing.successful += successData[i]
											existing.failed += failedData[i]
											existing.volume += volumeData[i]
											hourlyData[label] = existing
										} else {
											hourlyData[label] = struct {
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

	// If no data found, return empty trends structure
	if len(hourlyData) == 0 {
		return h.generatePaymentTrends(ctx, filters), nil
	}

	// Convert map back to arrays, maintaining chronological order
	for i := filters.Hours - 1; i >= 0; i-- {
		var label string
		if i == 0 {
			label = "Now"
		} else {
			label = fmt.Sprintf("%dh ago", i)
		}

		if data, exists := hourlyData[label]; exists {
			combinedLabels = append(combinedLabels, label)
			combinedSuccessData = append(combinedSuccessData, data.successful)
			combinedFailedData = append(combinedFailedData, data.failed)
			combinedVolumeData = append(combinedVolumeData, data.volume)
		} else {
			// Fill gaps with zeros
			combinedLabels = append(combinedLabels, label)
			combinedSuccessData = append(combinedSuccessData, 0)
			combinedFailedData = append(combinedFailedData, 0)
			combinedVolumeData = append(combinedVolumeData, 0.0)
		}
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

// Helper methods to generate realistic data (FALLBACK when PostgreSQL is empty or unavailable)

// generateDashboardStats generates fallback dashboard statistics
// FALLBACK DATA - Used when PostgreSQL has no data or is unavailable
func (h *AnalyticsHandler) generateDashboardStats(_ context.Context, filters AnalyticsFilters) DashboardStats {
	basePayments := 5000 + rand.Intn(5000)
	successRate := 95.0 + rand.Float64()*5.0
	totalVolume := 500000.0 + rand.Float64()*1000000.0
	responseTime := 150.0 + rand.Float64()*100.0

	// Round to 2 decimal places
	successRate = float64(int(successRate*100)) / 100
	totalVolume = float64(int(totalVolume*100)) / 100
	responseTime = float64(int(responseTime*100)) / 100

	environment := "all"
	if filters.Environment != nil {
		environment = *filters.Environment
	}

	return DashboardStats{
		TotalPayments:       basePayments,
		SuccessRate:         successRate,
		TotalVolume:         totalVolume,
		AvgResponseTime:     responseTime,
		TotalPaymentsChange: h.calculatePaymentChangeWithFilters(filters),
		SuccessRateChange:   h.calculateSuccessRateChangeWithFilters(filters),
		TotalVolumeChange:   h.calculateVolumeChangeWithFilters(filters),
		AvgResponseChange:   h.calculateResponseTimeChangeWithFilters(filters),
		ActiveTenants:       3, // Default fallback
		ActiveProviders:     len([]string{"iyzico", "stripe", "ozanpay", "paycell", "papara"}),
		Environment:         environment,
	}
}

func (h *AnalyticsHandler) generateProviderStats(_ context.Context, filters AnalyticsFilters) []ProviderStats {
	// FALLBACK DATA - Used when PostgreSQL has no data or is unavailable
	providers := []string{"İyzico", "Stripe", "OzanPay", "Paycell", "Papara", "Nkolay", "PayTR", "PayU"}

	// Filter by specific provider if requested
	if filters.ProviderID != nil {
		for _, provider := range providers {
			if provider == *filters.ProviderID {
				providers = []string{provider}
				break
			}
		}
	}

	stats := make([]ProviderStats, len(providers))

	for i, provider := range providers {
		status := "online"
		responseTime := 100 + rand.Intn(200)

		// Simulate occasional degraded performance
		if rand.Float32() < 0.1 {
			status = "degraded"
			responseTime = 400 + rand.Intn(200)
		}

		environment := "all"
		if filters.Environment != nil {
			environment = *filters.Environment
		}

		stats[i] = ProviderStats{
			Name:         provider,
			Status:       status,
			ResponseTime: strconv.Itoa(responseTime) + "ms",
			Transactions: 100 + rand.Intn(500),
			SuccessRate:  94.0 + rand.Float64()*6.0,
			Environment:  environment,
			TenantCount:  1 + rand.Intn(5), // Random tenant count
		}
	}

	return stats
}

func (h *AnalyticsHandler) generateRecentActivity(_ context.Context, filters AnalyticsFilters, limit int) []RecentActivity {
	// FALLBACK DATA - Used when PostgreSQL has no data or is unavailable
	providers := []string{"İyzico", "Stripe", "OzanPay", "Paycell", "Papara"}

	// Filter by specific provider if requested
	if filters.ProviderID != nil {
		providers = []string{*filters.ProviderID}
	}

	types := []string{"payment", "refund"}
	statuses := []string{"success", "failed", "processed"}

	activities := make([]RecentActivity, limit)

	for i := 0; i < limit; i++ {
		activityType := types[rand.Intn(len(types))]
		provider := providers[rand.Intn(len(providers))]
		status := statuses[rand.Intn(len(statuses))]
		amount := 50.0 + rand.Float64()*500.0
		minutesAgo := rand.Intn(60) + 1

		// Adjust status probabilities
		if rand.Float32() < 0.9 {
			status = "success"
		}

		environment := "production"
		if filters.Environment != nil {
			environment = *filters.Environment
		}

		tenantID := "1"
		if filters.TenantID != nil {
			tenantID = fmt.Sprintf("%d", *filters.TenantID)
		}

		activities[i] = RecentActivity{
			Type:     activityType,
			Provider: provider,
			Amount:   "₺" + strconv.FormatFloat(amount, 'f', 2, 64),
			Status:   status,
			Time:     strconv.Itoa(minutesAgo) + " min ago",
			ID:       "pay_" + strconv.Itoa(rand.Intn(999999)),
			TenantID: tenantID,
			Env:      environment,
		}
	}

	return activities
}

func (h *AnalyticsHandler) generatePaymentTrends(_ context.Context, filters AnalyticsFilters) map[string]any {
	// FALLBACK DATA - Used when PostgreSQL has no data or is unavailable
	labels := make([]string, filters.Hours)
	successData := make([]int, filters.Hours)
	failedData := make([]int, filters.Hours)

	for i := 0; i < filters.Hours; i++ {
		labels[i] = strconv.Itoa(filters.Hours-i-1) + "h ago"
		successData[i] = 50 + rand.Intn(100)
		failedData[i] = 2 + rand.Intn(10)
	}

	return map[string]any{
		"labels": labels,
		"datasets": []map[string]any{
			{
				"label":           "Successful Payments",
				"data":            successData,
				"borderColor":     "#10B981",
				"backgroundColor": "rgba(16, 185, 129, 0.1)",
			},
			{
				"label":           "Failed Payments",
				"data":            failedData,
				"borderColor":     "#EF4444",
				"backgroundColor": "rgba(239, 68, 68, 0.1)",
			},
		},
	}
}

// calculatePaymentChangeWithFilters calculates the percentage change in payment count from previous period
func (h *AnalyticsHandler) calculatePaymentChangeWithFilters(filters AnalyticsFilters) string {
	if h.logger == nil {
		return "+12.5% from yesterday"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all providers or filter by specific provider
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
	if filters.ProviderID != nil {
		providers = []string{*filters.ProviderID}
	}

	var currentTotal, previousTotal int

	// Get tenant IDs to process
	var tenantIDs []int
	if filters.TenantID != nil {
		tenantIDs = []int{*filters.TenantID}
	} else {
		tenantIDs = h.getActiveTenants(ctx, filters.Hours)
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
		tenantIDs = h.getActiveTenants(ctx, filters.Hours)
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
		tenantIDs = h.getActiveTenants(ctx, filters.Hours)
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
		tenantIDs = h.getActiveTenants(ctx, filters.Hours)
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

	var providers []map[string]any
	var err error

	if h.logger != nil {
		providers, err = h.getRealActiveProviders(ctx)
		if err != nil {
			logger.Warn("Failed to get active providers", logger.LogContext{
				Fields: map[string]any{
					"error": err.Error(),
				},
			})
			providers = h.generateActiveProviders()
		}
	} else {
		providers = h.generateActiveProviders()
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
			tenants = h.generateActiveTenants()
		}
	} else {
		tenants = h.generateActiveTenants()
	}

	response.Success(w, http.StatusOK, "Active tenants retrieved successfully", tenants)
}

// getRealActiveProviders fetches active providers from PostgreSQL
func (h *AnalyticsHandler) getRealActiveProviders(ctx context.Context) ([]map[string]any, error) {
	// Provider'ları sistemde tanımlı provider'lardan al
	providerKeys := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
	providerNames := []string{"İyzico", "Stripe", "OzanPay", "Paycell", "Papara", "Nkolay", "PayTR", "PayU"}

	var activeProviders []map[string]any

	// Her provider için son 24 saatte aktivite olup olmadığını kontrol et
	for i, providerKey := range providerKeys {
		// Tüm tenant'larda bu provider'ın aktivitesini kontrol et
		hasActivity := false

		// Birkaç tenant ID'si kontrol et (gerçek implementasyonda tüm tenant'ları kontrol edebilirsiniz)
		for tenantID := 1; tenantID <= 20; tenantID++ {
			stats, err := h.logger.GetPaymentStats(ctx, tenantID, providerKey, 24)
			if err == nil {
				if totalReq, ok := stats["total_requests"].(int); ok && totalReq > 0 {
					hasActivity = true
					break
				}
			}
		}

		// Aktivite varsa provider'ı listeye ekle
		if hasActivity {
			activeProviders = append(activeProviders, map[string]any{
				"id":   providerKey,
				"name": providerNames[i],
			})
		}
	}

	// Eğer hiç aktif provider yoksa, en azından varsayılan provider'ları döndür
	if len(activeProviders) == 0 {
		for i, providerKey := range providerKeys {
			activeProviders = append(activeProviders, map[string]any{
				"id":   providerKey,
				"name": providerNames[i],
			})
		}
	}

	return activeProviders, nil
}

// getRealActiveTenants fetches active tenants from PostgreSQL
func (h *AnalyticsHandler) getRealActiveTenants(ctx context.Context) ([]map[string]any, error) {
	var activeTenants []map[string]any

	// Son 24 saatte aktivitesi olan tenant'ları bul
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
	tenantMap := make(map[int]bool)

	// Belirli bir aralıkta tenant ID'leri kontrol et (gerçek implementasyonda tenant tablosundan alınabilir)
	for tenantID := 1; tenantID <= 100; tenantID++ {
		for _, provider := range providers {
			stats, err := h.logger.GetPaymentStats(ctx, tenantID, provider, 24)
			if err == nil {
				if totalReq, ok := stats["total_requests"].(int); ok && totalReq > 0 {
					tenantMap[tenantID] = true
					break // Bu tenant için aktivite bulundu
				}
			}
		}
	}

	// Bulunan aktif tenant'ları listeye ekle
	for tenantID := range tenantMap {
		// Gerçek implementasyonda tenant bilgileri tenant tablosundan alınabilir
		tenantName := h.getTenantName(tenantID)
		activeTenants = append(activeTenants, map[string]any{
			"id":   tenantID,
			"name": tenantName,
		})
	}

	// Eğer hiç aktif tenant yoksa, varsayılan tenant'ları döndür
	if len(activeTenants) == 0 {
		activeTenants = h.generateActiveTenants()
	}

	return activeTenants, nil
}

// getTenantName gets tenant name by ID (bu metod gerçek implementasyonda tenant tablosundan veri alabilir)
func (h *AnalyticsHandler) getTenantName(tenantID int) string {
	// Gerçek implementasyonda bu bilgi tenant tablosundan alınabilir
	tenantNames := map[int]string{
		1:  "E-commerce Platform",
		2:  "Digital Banking",
		3:  "Fintech Startup",
		4:  "Marketplace",
		5:  "SaaS Company",
		6:  "Gaming Platform",
		7:  "Travel Agency",
		8:  "Food Delivery",
		9:  "Education Platform",
		10: "Healthcare System",
	}

	if name, exists := tenantNames[tenantID]; exists {
		return name
	}

	return fmt.Sprintf("Tenant %d", tenantID)
}

// generateActiveProviders generates fallback provider list
func (h *AnalyticsHandler) generateActiveProviders() []map[string]any {
	// FALLBACK DATA - Used when PostgreSQL has no data or is unavailable
	providerKeys := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}
	providerNames := []string{"İyzico", "Stripe", "OzanPay", "Paycell", "Papara", "Nkolay", "PayTR", "PayU"}

	var providers []map[string]any
	for i, providerKey := range providerKeys {
		providers = append(providers, map[string]any{
			"id":   providerKey,
			"name": providerNames[i],
		})
	}

	return providers
}

// generateActiveTenants generates fallback tenant list
func (h *AnalyticsHandler) generateActiveTenants() []map[string]any {
	// FALLBACK DATA - Used when PostgreSQL has no data or is unavailable
	return []map[string]any{
		{"id": 1, "name": "E-commerce Platform"},
		{"id": 2, "name": "Digital Banking"},
		{"id": 3, "name": "Fintech Startup"},
		{"id": 4, "name": "Marketplace"},
		{"id": 5, "name": "SaaS Company"},
		{"id": 6, "name": "Gaming Platform"},
		{"id": 7, "name": "Travel Agency"},
		{"id": 8, "name": "Food Delivery"},
	}
}
