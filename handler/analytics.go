package handler

import (
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/mstgnz/gopay/infra/opensearch"
	"github.com/mstgnz/gopay/infra/response"
)

// AnalyticsHandler handles analytics related HTTP requests
type AnalyticsHandler struct {
	logger *opensearch.Logger
}

// NewAnalyticsHandler creates a new analytics handler
func NewAnalyticsHandler(logger *opensearch.Logger) *AnalyticsHandler {
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

	// Parse hours parameter (default 24 hours)
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 && h <= 168 {
			hours = h
		}
	}

	// Generate realistic stats (in production, this would come from OpenSearch)
	stats := h.generateDashboardStats(ctx, hours)

	response.Success(w, http.StatusOK, "Dashboard stats retrieved successfully", stats)
}

// GetProviderStats returns provider-specific statistics
func (h *AnalyticsHandler) GetProviderStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Generate provider stats
	providers := h.generateProviderStats(ctx)

	response.Success(w, http.StatusOK, "Provider stats retrieved successfully", providers)
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

	// Generate recent activity
	activities := h.generateRecentActivity(ctx, limit)

	response.Success(w, http.StatusOK, "Recent activity retrieved successfully", activities)
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

	// Generate trend data
	trends := h.generatePaymentTrends(ctx, hours)

	response.Success(w, http.StatusOK, "Payment trends retrieved successfully", trends)
}

// Helper methods to generate realistic data

func (h *AnalyticsHandler) generateDashboardStats(ctx context.Context, hours int) DashboardStats {
	// In production, these would be real queries to OpenSearch (FAKE DATA for demo)
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
		TotalPaymentsChange: "+12.5% from yesterday",
		SuccessRateChange:   "+0.8% from yesterday",
		TotalVolumeChange:   "+18.2% from yesterday",
		AvgResponseChange:   "-15ms from yesterday",
	}
}

func (h *AnalyticsHandler) generateProviderStats(ctx context.Context) []ProviderStats {
	// FAKE DATA for demo - In production, these would be real provider metrics
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

func (h *AnalyticsHandler) generateRecentActivity(ctx context.Context, limit int) []RecentActivity {
	// FAKE DATA for demo - In production, these would be real transaction logs
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

func (h *AnalyticsHandler) generatePaymentTrends(ctx context.Context, hours int) map[string]any {
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
