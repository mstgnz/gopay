package handler

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
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
}

// ProviderStats represents provider-specific statistics
type ProviderStats struct {
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	ResponseTime string  `json:"responseTime"`
	Transactions int     `json:"transactions"`
	SuccessRate  float64 `json:"successRate"`
}

// RecentActivity represents recent payment activity
type RecentActivity struct {
	Type     string `json:"type"`
	Provider string `json:"provider"`
	Amount   string `json:"amount"`
	Status   string `json:"status"`
	Time     string `json:"time"`
	ID       string `json:"id"`
}

// GetDashboardStats returns main dashboard statistics
func (h *AnalyticsHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tenantID := middle.GetTenantIDFromContext(r.Context())
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 && h <= 168 {
			hours = h
		}
	}

	var stats DashboardStats
	var err error

	if h.logger != nil {
		stats, err = h.getRealDashboardStats(ctx, tenantID, hours)
		if err != nil {
			// Log error but fallback to demo data
			logger.Warn("Failed to get real dashboard stats", logger.LogContext{
				TenantID: tenantID,
				Fields: map[string]any{
					"error": err.Error(),
					"hours": hours,
				},
			})
			stats = h.generateDashboardStats(ctx, hours)
		}
	} else {
		stats = h.generateDashboardStats(ctx, hours)
	}

	response.Success(w, http.StatusOK, "Dashboard stats retrieved successfully", stats)
}

// getRealDashboardStats fetches real analytics data from PostgreSQL
func (h *AnalyticsHandler) getRealDashboardStats(ctx context.Context, tenantID string, hours int) (DashboardStats, error) {
	// Get all providers data
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}

	var totalPayments int
	var totalSuccessful int
	var totalVolume float64
	var totalResponseTime float64
	var responseTimeCount int

	// Convert tenantID to int for PostgreSQL
	tenantIDInt := 0
	if tenantID != "" && tenantID != "legacy" {
		fmt.Sscanf(tenantID, "%d", &tenantIDInt)
	}

	for _, provider := range providers {
		// Get provider stats from PostgreSQL
		providerStats, err := h.logger.GetPaymentStats(ctx, tenantIDInt, provider, hours)
		if err != nil {
			continue // Skip provider if error
		}

		// Extract stats from PostgreSQL response
		if totalReq, ok := providerStats["total_requests"].(int); ok {
			totalPayments += totalReq
		}
		if successReq, ok := providerStats["success_count"].(int); ok {
			totalSuccessful += successReq
		}
		if avgTime, ok := providerStats["avg_processing_ms"].(float64); ok && avgTime > 0 {
			totalResponseTime += avgTime
			responseTimeCount++
		}

		// Get payment volumes from PostgreSQL
		volume, err := h.getProviderVolume(ctx, tenantIDInt, provider, hours)
		if err == nil {
			totalVolume += volume
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

	return DashboardStats{
		TotalPayments:       totalPayments,
		SuccessRate:         successRate,
		TotalVolume:         totalVolume,
		AvgResponseTime:     avgResponseTime,
		TotalPaymentsChange: h.calculatePaymentChange(tenantIDInt, hours),
		SuccessRateChange:   h.calculateSuccessRateChange(tenantIDInt, hours),
		TotalVolumeChange:   h.calculateVolumeChange(tenantIDInt, hours),
		AvgResponseChange:   h.calculateResponseTimeChange(tenantIDInt, hours),
	}, nil
}

// getProviderVolume calculates total payment volume for a provider
func (h *AnalyticsHandler) getProviderVolume(ctx context.Context, tenantID int, provider string, hours int) (float64, error) {
	// Create filters for PostgreSQL search
	filters := map[string]any{
		"start_date": time.Now().Add(-time.Duration(hours) * time.Hour),
		"end_date":   time.Now(),
	}

	logs, err := h.logger.SearchPaymentLogs(ctx, tenantID, provider, filters)
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

	tenantID := middle.GetTenantIDFromContext(r.Context())

	var providers []ProviderStats
	var err error

	if h.logger != nil {
		providers, err = h.getRealProviderStats(ctx, tenantID)
		if err != nil {
			logger.Warn("Failed to get real provider stats", logger.LogContext{
				TenantID: tenantID,
				Fields: map[string]any{
					"error": err.Error(),
				},
			})
			providers = h.generateProviderStats(ctx)
		}
	} else {
		providers = h.generateProviderStats(ctx)
	}

	response.Success(w, http.StatusOK, "Provider stats retrieved successfully", providers)
}

// getRealProviderStats fetches real provider statistics from PostgreSQL
func (h *AnalyticsHandler) getRealProviderStats(ctx context.Context, tenantID string) ([]ProviderStats, error) {
	providers := []string{"İyzico", "Stripe", "OzanPay", "Paycell", "Papara", "Nkolay", "PayTR", "PayU"}
	providerKeys := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}

	stats := make([]ProviderStats, len(providers))

	// Convert tenantID to int for PostgreSQL
	tenantIDInt := 0
	if tenantID != "" && tenantID != "legacy" {
		fmt.Sscanf(tenantID, "%d", &tenantIDInt)
	}

	for i, providerKey := range providerKeys {
		// Get provider stats from PostgreSQL
		providerStats, err := h.logger.GetPaymentStats(ctx, tenantIDInt, providerKey, 24)

		status := "online"
		responseTime := "150ms"
		transactions := 0
		successRate := 95.0

		if err == nil {
			// Extract stats from PostgreSQL response
			if totalReq, ok := providerStats["total_requests"].(int); ok {
				transactions = totalReq
			}
			if successCount, ok := providerStats["success_count"].(int); ok && transactions > 0 {
				successRate = (float64(successCount) / float64(transactions)) * 100
			}
			if avgTime, ok := providerStats["avg_processing_ms"].(float64); ok && avgTime > 0 {
				responseTime = fmt.Sprintf("%.0fms", avgTime)
				// Mark as degraded if response time > 400ms
				if avgTime > 400 {
					status = "degraded"
				}
			}
		} else {
			// If no data, use fallback values
			transactions = 100 + rand.Intn(500)
			responseTime = fmt.Sprintf("%dms", 100+rand.Intn(200))
			if rand.Float32() < 0.1 {
				status = "degraded"
				responseTime = fmt.Sprintf("%dms", 400+rand.Intn(200))
			}
		}

		// Round success rate to 2 decimal places
		successRate = float64(int(successRate*100)) / 100

		stats[i] = ProviderStats{
			Name:         providers[i],
			Status:       status,
			ResponseTime: responseTime,
			Transactions: transactions,
			SuccessRate:  successRate,
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

	tenantID := middle.GetTenantIDFromContext(r.Context())

	var activities []RecentActivity
	var err error

	if h.logger != nil {
		activities, err = h.getRealRecentActivity(ctx, tenantID, limit)
		if err != nil {
			logger.Warn("Failed to get real recent activity", logger.LogContext{
				TenantID: tenantID,
				Fields: map[string]any{
					"error": err.Error(),
					"limit": limit,
				},
			})
			activities = h.generateRecentActivity(ctx, limit)
		}
	} else {
		activities = h.generateRecentActivity(ctx, limit)
	}

	response.Success(w, http.StatusOK, "Recent activity retrieved successfully", activities)
}

// getRealRecentActivity fetches real recent activity from PostgreSQL
func (h *AnalyticsHandler) getRealRecentActivity(ctx context.Context, tenantID string, limit int) ([]RecentActivity, error) {
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara"}
	var allActivities []RecentActivity

	// Convert tenantID to int for PostgreSQL
	tenantIDInt := 0
	if tenantID != "" && tenantID != "legacy" {
		fmt.Sscanf(tenantID, "%d", &tenantIDInt)
	}

	for _, provider := range providers {
		// Create filters for recent payments
		filters := map[string]any{
			"start_date": time.Now().Add(-2 * time.Hour), // Last 2 hours
			"end_date":   time.Now(),
		}

		logs, err := h.logger.SearchPaymentLogs(ctx, tenantIDInt, provider, filters)
		if err != nil {
			continue // Skip provider if error
		}

		// Convert logs to activities (take first few)
		for i, log := range logs {
			if i >= limit/len(providers) { // Distribute across providers
				break
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

			allActivities = append(allActivities, RecentActivity{
				Type:     activityType,
				Provider: provider,
				Amount:   amount,
				Status:   status,
				Time:     timeStr,
				ID:       paymentID,
			})
		}
	}

	// If no real data, return fallback
	if len(allActivities) == 0 {
		return h.generateRecentActivity(ctx, limit), nil
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

	// Parse hours parameter (default 24 hours)
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 && h <= 168 {
			hours = h
		}
	}

	tenantID := middle.GetTenantIDFromContext(r.Context())

	var trends map[string]any
	var err error

	if h.logger != nil {
		trends, err = h.getRealPaymentTrends(ctx, tenantID, hours)
		if err != nil {
			logger.Warn("Failed to get real payment trends", logger.LogContext{
				TenantID: tenantID,
				Fields: map[string]any{
					"error": err.Error(),
					"hours": hours,
				},
			})
			trends = h.generatePaymentTrends(ctx, hours)
		}
	} else {
		trends = h.generatePaymentTrends(ctx, hours)
	}

	response.Success(w, http.StatusOK, "Payment trends retrieved successfully", trends)
}

// getRealPaymentTrends fetches real payment trends from PostgreSQL
func (h *AnalyticsHandler) getRealPaymentTrends(ctx context.Context, tenantID string, hours int) (map[string]any, error) {
	// Convert tenantID to int for PostgreSQL
	tenantIDInt := 0
	if tenantID != "" && tenantID != "legacy" {
		fmt.Sscanf(tenantID, "%d", &tenantIDInt)
	}

	// Get all providers and combine their trends
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}

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

	for _, provider := range providers {
		trends, err := h.logger.GetPaymentTrends(ctx, tenantIDInt, provider, hours)
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

	// If no data found, return empty trends structure
	if len(hourlyData) == 0 {
		return h.generatePaymentTrends(ctx, hours), nil
	}

	// Convert map back to arrays, maintaining chronological order
	for i := hours - 1; i >= 0; i-- {
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
func (h *AnalyticsHandler) generateDashboardStats(_ context.Context, hours int) DashboardStats {
	basePayments := 5000 + rand.Intn(5000)
	successRate := 95.0 + rand.Float64()*5.0
	totalVolume := 500000.0 + rand.Float64()*1000000.0
	responseTime := 150.0 + rand.Float64()*100.0

	// Round to 2 decimal places
	successRate = float64(int(successRate*100)) / 100
	totalVolume = float64(int(totalVolume*100)) / 100
	responseTime = float64(int(responseTime*100)) / 100

	return DashboardStats{
		TotalPayments:       basePayments,
		SuccessRate:         successRate,
		TotalVolume:         totalVolume,
		AvgResponseTime:     responseTime,
		TotalPaymentsChange: h.calculatePaymentChange(0, hours),
		SuccessRateChange:   h.calculateSuccessRateChange(0, hours),
		TotalVolumeChange:   h.calculateVolumeChange(0, hours),
		AvgResponseChange:   h.calculateResponseTimeChange(0, hours),
	}
}

func (h *AnalyticsHandler) generateProviderStats(_ context.Context) []ProviderStats {
	// FALLBACK DATA - Used when PostgreSQL has no data or is unavailable
	providers := []string{"İyzico", "Stripe", "OzanPay", "Paycell", "Papara", "Nkolay", "PayTR", "PayU"}
	stats := make([]ProviderStats, len(providers))

	for i, provider := range providers {
		status := "online"
		responseTime := 100 + rand.Intn(200)

		// Simulate occasional degraded performance
		if rand.Float32() < 0.1 {
			status = "degraded"
			responseTime = 400 + rand.Intn(200)
		}

		stats[i] = ProviderStats{
			Name:         provider,
			Status:       status,
			ResponseTime: strconv.Itoa(responseTime) + "ms",
			Transactions: 100 + rand.Intn(500),
			SuccessRate:  94.0 + rand.Float64()*6.0,
		}
	}

	return stats
}

func (h *AnalyticsHandler) generateRecentActivity(_ context.Context, limit int) []RecentActivity {
	// FALLBACK DATA - Used when PostgreSQL has no data or is unavailable
	providers := []string{"İyzico", "Stripe", "OzanPay", "Paycell", "Papara"}
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

		activities[i] = RecentActivity{
			Type:     activityType,
			Provider: provider,
			Amount:   "₺" + strconv.FormatFloat(amount, 'f', 2, 64),
			Status:   status,
			Time:     strconv.Itoa(minutesAgo) + " min ago",
			ID:       "pay_" + strconv.Itoa(rand.Intn(999999)),
		}
	}

	return activities
}

func (h *AnalyticsHandler) generatePaymentTrends(_ context.Context, hours int) map[string]any {
	// FALLBACK DATA - Used when PostgreSQL has no data or is unavailable
	labels := make([]string, hours)
	successData := make([]int, hours)
	failedData := make([]int, hours)

	for i := 0; i < hours; i++ {
		labels[i] = strconv.Itoa(hours-i-1) + "h ago"
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

// calculatePaymentChange calculates the percentage change in payment count from previous period
func (h *AnalyticsHandler) calculatePaymentChange(tenantID int, hours int) string {
	if h.logger == nil {
		return "+12.5% from yesterday"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all providers
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}

	var currentTotal, previousTotal int

	for _, provider := range providers {
		stats, err := h.logger.GetPaymentStatsComparison(ctx, tenantID, provider, hours, hours)
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

	// Calculate percentage change
	if previousTotal == 0 {
		if currentTotal > 0 {
			return fmt.Sprintf("+∞%% from previous %dh", hours)
		}
		return "No previous data"
	}

	change := ((float64(currentTotal) - float64(previousTotal)) / float64(previousTotal)) * 100

	if change > 0 {
		return fmt.Sprintf("+%.1f%% from previous %dh", change, hours)
	} else if change < 0 {
		return fmt.Sprintf("%.1f%% from previous %dh", change, hours)
	} else {
		return fmt.Sprintf("No change from previous %dh", hours)
	}
}

// calculateSuccessRateChange calculates the percentage change in success rate from previous period
func (h *AnalyticsHandler) calculateSuccessRateChange(tenantID int, hours int) string {
	if h.logger == nil {
		return "+0.8% from yesterday"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all providers
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}

	var currentTotal, currentSuccess, previousTotal, previousSuccess int

	for _, provider := range providers {
		stats, err := h.logger.GetPaymentStatsComparison(ctx, tenantID, provider, hours, hours)
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
		return fmt.Sprintf("+%.1f%% from previous %dh", change, hours)
	} else if change < 0 {
		return fmt.Sprintf("%.1f%% from previous %dh", change, hours)
	} else {
		return fmt.Sprintf("No change from previous %dh", hours)
	}
}

// calculateVolumeChange calculates the percentage change in payment volume from previous period
func (h *AnalyticsHandler) calculateVolumeChange(tenantID int, hours int) string {
	if h.logger == nil {
		return "+18.2% from yesterday"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all providers
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}

	var currentVolume, previousVolume float64

	for _, provider := range providers {
		stats, err := h.logger.GetPaymentStatsComparison(ctx, tenantID, provider, hours, hours)
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

	// Calculate percentage change
	if previousVolume == 0 {
		if currentVolume > 0 {
			return fmt.Sprintf("+∞%% from previous %dh", hours)
		}
		return "No previous data"
	}

	change := ((currentVolume - previousVolume) / previousVolume) * 100

	if change > 0 {
		return fmt.Sprintf("+%.1f%% from previous %dh", change, hours)
	} else if change < 0 {
		return fmt.Sprintf("%.1f%% from previous %dh", change, hours)
	} else {
		return fmt.Sprintf("No change from previous %dh", hours)
	}
}

// calculateResponseTimeChange calculates the change in average response time from previous period
func (h *AnalyticsHandler) calculateResponseTimeChange(tenantID int, hours int) string {
	if h.logger == nil {
		return "-15ms from yesterday"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all providers
	providers := []string{"iyzico", "stripe", "ozanpay", "paycell", "papara", "nkolay", "paytr", "payu"}

	var currentSum, previousSum float64
	var currentCount, previousCount int

	for _, provider := range providers {
		stats, err := h.logger.GetPaymentStatsComparison(ctx, tenantID, provider, hours, hours)
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
		return fmt.Sprintf("+%.0fms from previous %dh", change, hours)
	} else if change < 0 {
		return fmt.Sprintf("%.0fms from previous %dh", change, hours)
	} else {
		return fmt.Sprintf("No change from previous %dh", hours)
	}
}
