package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/postgres"
	"github.com/mstgnz/gopay/infra/response"
	"github.com/mstgnz/gopay/provider"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	db             *sql.DB
	postgresLogger *postgres.Logger
	paymentService *provider.PaymentService
	providerConfig *config.ProviderConfig
	startTime      time.Time
}

// HealthStatus represents overall system health
type HealthStatus struct {
	Status      string                     `json:"status"`
	Version     string                     `json:"version"`
	Timestamp   time.Time                  `json:"timestamp"`
	Uptime      string                     `json:"uptime"`
	Environment string                     `json:"environment"`
	Database    *DatabaseHealth            `json:"database"`
	Providers   map[string]*ProviderHealth `json:"providers"`
	System      *SystemHealth              `json:"system"`
	Services    map[string]*ServiceHealth  `json:"services"`
}

// DatabaseHealth represents database health status
type DatabaseHealth struct {
	Status       string        `json:"status"`
	Connected    bool          `json:"connected"`
	ResponseTime time.Duration `json:"response_time_ms"`
	OpenConns    int           `json:"open_connections"`
	InUseConns   int           `json:"in_use_connections"`
	IdleConns    int           `json:"idle_connections"`
	WaitCount    int64         `json:"wait_count"`
	Version      string        `json:"version,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// ProviderHealth represents payment provider health
type ProviderHealth struct {
	Status       string  `json:"status"`
	Available    bool    `json:"available"`
	ResponseTime string  `json:"response_time"`
	LastCheck    string  `json:"last_check"`
	Configured   bool    `json:"configured"`
	ErrorRate    float64 `json:"error_rate,omitempty"`
	Error        string  `json:"error,omitempty"`
}

// SystemHealth represents system resource health
type SystemHealth struct {
	Memory     *MemoryHealth `json:"memory"`
	Disk       *DiskHealth   `json:"disk"`
	GoRoutines int           `json:"goroutines"`
	CGoCalls   int64         `json:"cgo_calls"`
}

// MemoryHealth represents memory usage
type MemoryHealth struct {
	Alloc        string  `json:"alloc"`
	TotalAlloc   string  `json:"total_alloc"`
	Sys          string  `json:"sys"`
	GCRuns       uint32  `json:"gc_runs"`
	UsagePercent float64 `json:"usage_percent"`
}

// DiskHealth represents disk usage
type DiskHealth struct {
	Available    string  `json:"available"`
	Used         string  `json:"used"`
	Total        string  `json:"total"`
	UsagePercent float64 `json:"usage_percent"`
	Status       string  `json:"status"`
}

// ServiceHealth represents individual service health
type ServiceHealth struct {
	Status      string `json:"status"`
	Healthy     bool   `json:"healthy"`
	LastCheck   string `json:"last_check"`
	Description string `json:"description,omitempty"`
	Error       string `json:"error,omitempty"`
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *sql.DB, postgresLogger *postgres.Logger, paymentService *provider.PaymentService, providerConfig *config.ProviderConfig) *HealthHandler {
	return &HealthHandler{
		db:             db,
		postgresLogger: postgresLogger,
		paymentService: paymentService,
		providerConfig: providerConfig,
		startTime:      time.Now(),
	}
}

// CheckHealth performs comprehensive health checks
func (h *HealthHandler) CheckHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	health := &HealthStatus{
		Version:     "1.0.0",
		Timestamp:   time.Now().UTC(),
		Uptime:      time.Since(h.startTime).String(),
		Environment: getEnvironment(),
		Database:    h.checkDatabaseHealth(ctx),
		Providers:   h.checkProvidersHealth(ctx),
		System:      h.checkSystemHealth(),
		Services:    h.checkServicesHealth(ctx),
	}

	// Determine overall status
	health.Status = h.determineOverallStatus(health)

	// Set appropriate HTTP status code
	statusCode := http.StatusOK
	switch health.Status {
	case "degraded":
		statusCode = http.StatusOK // Still 200, but clients can check status field
	case "unhealthy":
		statusCode = http.StatusServiceUnavailable
	}

	_ = response.WriteJSON(w, statusCode, response.Response{
		Success: health.Status != "unhealthy",
		Message: fmt.Sprintf("Service is %s", health.Status),
		Data:    health,
	})
}

// checkDatabaseHealth checks PostgreSQL database health
func (h *HealthHandler) checkDatabaseHealth(ctx context.Context) *DatabaseHealth {
	dbHealth := &DatabaseHealth{
		Status:    "unknown",
		Connected: false,
	}

	if h.db == nil {
		dbHealth.Status = "not_configured"
		dbHealth.Error = "Database not configured"
		return dbHealth
	}

	start := time.Now()

	// Test database connection with ping
	if err := h.db.PingContext(ctx); err != nil {
		dbHealth.Status = "unhealthy"
		dbHealth.Error = err.Error()
		dbHealth.ResponseTime = time.Since(start)
		return dbHealth
	}

	dbHealth.Connected = true
	dbHealth.ResponseTime = time.Since(start)

	// Get database stats
	stats := h.db.Stats()
	dbHealth.OpenConns = stats.OpenConnections
	dbHealth.InUseConns = stats.InUse
	dbHealth.IdleConns = stats.Idle
	dbHealth.WaitCount = stats.WaitCount

	// Get PostgreSQL version
	var version string
	if err := h.db.QueryRowContext(ctx, "SELECT version()").Scan(&version); err == nil {
		// Extract just the version number
		if parts := strings.Fields(version); len(parts) > 1 {
			dbHealth.Version = parts[1]
		}
	}

	// Determine status based on metrics
	if dbHealth.ResponseTime > 1*time.Second {
		dbHealth.Status = "degraded"
	} else if dbHealth.WaitCount > 100 {
		dbHealth.Status = "degraded"
	} else {
		dbHealth.Status = "healthy"
	}

	return dbHealth
}

// checkProvidersHealth checks payment providers health
func (h *HealthHandler) checkProvidersHealth(ctx context.Context) map[string]*ProviderHealth {
	providers := make(map[string]*ProviderHealth)

	// Get available providers from registry
	availableProviders := provider.GetAvailableProviders()

	for _, providerName := range availableProviders {
		providers[providerName] = h.checkSingleProviderHealth(ctx, providerName)
	}

	return providers
}

// checkSingleProviderHealth checks health of a single provider
func (h *HealthHandler) checkSingleProviderHealth(ctx context.Context, providerName string) *ProviderHealth {
	health := &ProviderHealth{
		Configured: true,
		Available:  true,
		LastCheck:  time.Now().UTC().Format(time.RFC3339),
	}

	// Check if provider is registered in the registry
	_, err := provider.Get(providerName)
	if err != nil {
		health.Status = "not_available"
		health.Available = false
		health.Configured = false
		health.Error = err.Error()
		return health
	}

	// Since provider is registered, it's available
	start := time.Now()
	responseTime := time.Since(start)
	health.ResponseTime = fmt.Sprintf("%.0fms", float64(responseTime.Nanoseconds())/1e6)
	health.Status = "healthy"

	// Get error rate from PostgreSQL if available
	if h.postgresLogger != nil {
		if stats, err := h.postgresLogger.GetPaymentStats(ctx, 0, providerName, 24); err == nil {
			if totalReq, ok := stats["total_requests"].(int); ok && totalReq > 0 {
				if errorCount, ok := stats["error_count"].(int); ok {
					health.ErrorRate = (float64(errorCount) / float64(totalReq)) * 100
					if health.ErrorRate > 10 { // More than 10% error rate
						health.Status = "degraded"
					}
				}
			}
		}
	}

	return health
}

// checkSystemHealth checks system resource health
func (h *HealthHandler) checkSystemHealth() *SystemHealth {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get disk usage
	diskHealth := h.getDiskUsage()

	return &SystemHealth{
		Memory: &MemoryHealth{
			Alloc:        formatBytes(memStats.Alloc),
			TotalAlloc:   formatBytes(memStats.TotalAlloc),
			Sys:          formatBytes(memStats.Sys),
			GCRuns:       memStats.NumGC,
			UsagePercent: calculateMemoryUsagePercent(memStats),
		},
		Disk:       diskHealth,
		GoRoutines: runtime.NumGoroutine(),
		CGoCalls:   runtime.NumCgoCall(),
	}
}

// checkServicesHealth checks individual service health
func (h *HealthHandler) checkServicesHealth(ctx context.Context) map[string]*ServiceHealth {
	services := make(map[string]*ServiceHealth)

	// Check PostgreSQL Logger Service
	services["postgresql_logger"] = &ServiceHealth{
		LastCheck: time.Now().UTC().Format(time.RFC3339),
	}

	if h.postgresLogger != nil {
		services["postgresql_logger"].Status = "healthy"
		services["postgresql_logger"].Healthy = true
		services["postgresql_logger"].Description = "Payment logging to PostgreSQL"
	} else {
		services["postgresql_logger"].Status = "not_configured"
		services["postgresql_logger"].Healthy = false
		services["postgresql_logger"].Description = "PostgreSQL logging not configured"
	}

	// Check Payment Service
	services["payment_service"] = &ServiceHealth{
		LastCheck: time.Now().UTC().Format(time.RFC3339),
	}

	if h.paymentService != nil {
		services["payment_service"].Status = "healthy"
		services["payment_service"].Healthy = true
		services["payment_service"].Description = "Payment processing service"
	} else {
		services["payment_service"].Status = "unhealthy"
		services["payment_service"].Healthy = false
		services["payment_service"].Error = "Payment service not initialized"
	}

	// Check Provider Config Service
	services["provider_config"] = &ServiceHealth{
		LastCheck: time.Now().UTC().Format(time.RFC3339),
	}

	if h.providerConfig != nil {
		services["provider_config"].Status = "healthy"
		services["provider_config"].Healthy = true
		services["provider_config"].Description = "Payment provider configuration service"
	} else {
		services["provider_config"].Status = "unhealthy"
		services["provider_config"].Healthy = false
		services["provider_config"].Error = "Provider config service not initialized"
	}

	return services
}

// determineOverallStatus determines overall system status
func (h *HealthHandler) determineOverallStatus(health *HealthStatus) string {
	// Check database
	if health.Database != nil && health.Database.Status == "unhealthy" {
		return "unhealthy"
	}

	// Check critical services
	criticalServices := []string{"payment_service", "provider_config"}
	for _, serviceName := range criticalServices {
		if service, exists := health.Services[serviceName]; exists {
			if !service.Healthy {
				return "unhealthy"
			}
		}
	}

	// Check if any provider is healthy (at least one should work)
	hasHealthyProvider := false
	degradedProviders := 0
	totalConfiguredProviders := 0

	for _, provider := range health.Providers {
		if provider.Configured {
			totalConfiguredProviders++
			if provider.Status == "healthy" {
				hasHealthyProvider = true
			} else if provider.Status == "degraded" {
				degradedProviders++
			}
		}
	}

	// System status based on providers and resources
	if !hasHealthyProvider && totalConfiguredProviders > 0 {
		return "unhealthy"
	}

	// Check system resources
	if health.System != nil {
		if health.System.Memory.UsagePercent > 90 {
			return "degraded"
		}
		if health.System.Disk != nil && health.System.Disk.UsagePercent > 90 {
			return "degraded"
		}
	}

	// Check database performance
	if health.Database != nil && health.Database.Status == "degraded" {
		return "degraded"
	}

	// If more than half of configured providers are degraded
	if totalConfiguredProviders > 0 && float64(degradedProviders)/float64(totalConfiguredProviders) > 0.5 {
		return "degraded"
	}

	return "healthy"
}

// Helper functions

func getEnvironment() string {
	if env := config.GetEnv("ENVIRONMENT", ""); env != "" {
		return env
	}
	if env := config.GetEnv("ENV", ""); env != "" {
		return env
	}
	return "development"
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func calculateMemoryUsagePercent(memStats runtime.MemStats) float64 {
	// This is a simplified calculation
	// In production, you might want to get actual system memory limits
	return (float64(memStats.Alloc) / float64(memStats.Sys)) * 100
}

func (h *HealthHandler) getDiskUsage() *DiskHealth {
	// This is a simplified implementation
	// For production, you'd want to use syscalls to get actual disk usage
	var stat syscall.Statfs_t
	wd := "/"

	disk := &DiskHealth{
		Status: "unknown",
	}

	if err := syscall.Statfs(wd, &stat); err != nil {
		disk.Status = "error"
		return disk
	}

	// Available space
	available := stat.Bavail * uint64(stat.Bsize)
	// Total space
	total := stat.Blocks * uint64(stat.Bsize)
	// Used space
	used := total - (stat.Bfree * uint64(stat.Bsize))

	disk.Available = formatBytes(available)
	disk.Total = formatBytes(total)
	disk.Used = formatBytes(used)
	disk.UsagePercent = (float64(used) / float64(total)) * 100

	if disk.UsagePercent > 90 {
		disk.Status = "critical"
	} else if disk.UsagePercent > 80 {
		disk.Status = "warning"
	} else {
		disk.Status = "healthy"
	}

	return disk
}
