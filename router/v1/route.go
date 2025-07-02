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

// Routes registers all API routes
func Routes(r chi.Router, logger *opensearch.Logger) {
	// Initialize handlers with the global services
	validator := validator.New()
	paymentHandler := handler.NewPaymentHandler(paymentService, validator)
	configHandler := handler.NewConfigHandler(providerConfig, paymentService, validator)
	logsHandler := handler.NewLogsHandler(logger)

	// Configuration routes for tenant-based provider setup
	r.Post("/set-env", configHandler.SetEnv)                            // POST /v1/set-env
	r.Get("/config/tenant-config", configHandler.GetTenantConfig)       // GET /v1/config/tenant-config?provider=iyzico
	r.Delete("/config/tenant-config", configHandler.DeleteTenantConfig) // DELETE /v1/config/tenant-config?provider=iyzico
	r.Get("/stats", configHandler.GetStats)                             // GET /v1/stats

	// Payment routes
	r.Route("/payments", func(r chi.Router) {
		// General payment routes (uses default provider)
		r.Post("/", paymentHandler.ProcessPayment)
		r.Get("/{paymentID}", paymentHandler.GetPaymentStatus)
		r.Delete("/{paymentID}", paymentHandler.CancelPayment)
		r.Post("/refund", paymentHandler.RefundPayment)

		// Provider-specific payment routes
		r.Post("/{provider}", paymentHandler.ProcessPayment)
		r.Get("/{provider}/{paymentID}", paymentHandler.GetPaymentStatus)
		r.Delete("/{provider}/{paymentID}", paymentHandler.CancelPayment)
		r.Post("/{provider}/refund", paymentHandler.RefundPayment)
	})

	// Logs routes (tenant and provider specific)
	r.Route("/logs", func(r chi.Router) {
		r.Get("/{provider}", logsHandler.ListLogs)                           // GET /v1/logs/{provider}?paymentId=123&status=success&errorsOnly=true&hours=24
		r.Get("/{provider}/payment/{paymentID}", logsHandler.GetPaymentLogs) // GET /v1/logs/{provider}/payment/{paymentID}
		r.Get("/{provider}/errors", logsHandler.GetErrorLogs)                // GET /v1/logs/{provider}/errors?hours=24
		r.Get("/{provider}/stats", logsHandler.GetLogStats)                  // GET /v1/logs/{provider}/stats?hours=24
	})

	// Stats endpoint for logging statistics (handled by middleware)
	// GET /v1/stats?provider=iyzico&hours=24
}
