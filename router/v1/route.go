package v1

import (
	"log"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/handler"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/opensearch"
	"github.com/mstgnz/gopay/provider"
)

// Create global services
var (
	paymentService *provider.PaymentService
	providerConfig *config.ProviderConfig
)

// Initialize payment service and register providers
func init() {
	// Create payment service and provider config
	paymentService = provider.NewPaymentService()
	providerConfig = config.NewProviderConfig()

	// Load provider configurations from environment variables
	providerConfig.LoadFromEnv()

	// Register all configured providers
	for _, providerName := range providerConfig.GetAvailableProviders() {
		providerCfg, err := providerConfig.GetConfig(providerName)
		if err != nil {
			log.Printf("Failed to get configuration for provider %s: %v", providerName, err)
			continue
		}

		// Add GoPay base URL to provider config
		providerCfg["gopayBaseURL"] = providerConfig.GetBaseURL()

		if err := paymentService.AddProvider(providerName, providerCfg); err != nil {
			log.Printf("Failed to register provider %s: %v", providerName, err)
			continue
		}

		log.Printf("Registered payment provider: %s", providerName)
	}

	// Set the first available provider as default
	availableProviders := providerConfig.GetAvailableProviders()
	if len(availableProviders) > 0 {
		if err := paymentService.SetDefaultProvider(availableProviders[0]); err != nil {
			log.Printf("Failed to set default provider: %v", err)
		} else {
			log.Printf("Default payment provider set to: %s", availableProviders[0])
		}
	} else {
		log.Println("No payment providers configured!")
	}
}

// Cleanup should be called on application shutdown to close SQLite connections
func Cleanup() {
	if providerConfig != nil {
		if err := providerConfig.Close(); err != nil {
			log.Printf("Warning: Failed to close provider config: %v", err)
		}
	}
}

// Routes defines all v1 API routes
func Routes(r chi.Router, openSearchLogger *opensearch.Logger) {
	// Initialize services
	paymentService := provider.NewPaymentService()
	providerConfig := config.NewProviderConfig()
	providerConfig.LoadFromEnv()

	// Initialize handlers
	validator := validator.New()
	paymentHandler := handler.NewPaymentHandler(paymentService, validator)
	configHandler := handler.NewConfigHandler(providerConfig, paymentService, validator)
	analyticsHandler := handler.NewAnalyticsHandler(openSearchLogger)
	logsHandler := handler.NewLogsHandler(openSearchLogger, openSearchLogger)

	// Payment routes
	r.Route("/payments", func(r chi.Router) {
		r.Post("/{provider}", paymentHandler.ProcessPayment)
		r.Get("/{provider}/{paymentID}", paymentHandler.GetPaymentStatus)
		r.Delete("/{provider}/{paymentID}", paymentHandler.CancelPayment)
		r.Post("/{provider}/refund", paymentHandler.RefundPayment)
	})

	// Configuration routes
	r.Route("/config", func(r chi.Router) {
		r.Post("/tenant-config", configHandler.SetEnv)
		r.Get("/tenant-config", configHandler.GetTenantConfig)
		r.Delete("/tenant-config", configHandler.DeleteTenantConfig)
	})

	// Analytics routes
	r.Route("/analytics", func(r chi.Router) {
		r.Get("/dashboard", analyticsHandler.GetDashboardStats)
		r.Get("/providers", analyticsHandler.GetProviderStats)
		r.Get("/activity", analyticsHandler.GetRecentActivity)
		r.Get("/trends", analyticsHandler.GetPaymentTrends)
	})

	// Logs routes
	r.Route("/logs", func(r chi.Router) {
		// Payment logs
		r.Get("/payments", logsHandler.GetPaymentLogs) // GET /v1/logs/payments?provider=iyzico&hours=24&payment_id=123

		// System logs
		r.Get("/system", logsHandler.GetSystemLogs) // GET /v1/logs/system?level=error&component=provider&hours=24&limit=100

		// Log statistics
		r.Get("/stats", logsHandler.GetLogStats) // GET /v1/logs/stats?hours=24
	})

	// Legacy routes for backward compatibility
	r.Route("/set-env", func(r chi.Router) {
		r.Post("/", configHandler.SetEnv)
	})

	r.Route("/stats", func(r chi.Router) {
		r.Get("/", analyticsHandler.GetDashboardStats)
	})
}
