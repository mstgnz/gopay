package v1

import (
	"log"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/handler"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
	_ "github.com/mstgnz/gopay/provider/iyzico"  // Import for side-effect registration
	_ "github.com/mstgnz/gopay/provider/ozanpay" // Import for side-effect registration
)

// Create a global payment service
var paymentService *provider.PaymentService

// Initialize payment service and register providers
func init() {
	// Create payment service
	paymentService = provider.NewPaymentService()

	// Load provider configurations from environment variables
	providerConfig := config.NewProviderConfig()
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

// Routes registers all API routes
func Routes(r chi.Router) {
	// Initialize handler with the global payment service
	paymentHandler := handler.NewPaymentHandler(paymentService, validator.New())

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

	// Callback routes for 3D Secure payments
	r.Route("/callback", func(r chi.Router) {
		// General callback route (uses default provider)
		r.HandleFunc("/", paymentHandler.HandleCallback)

		// Provider-specific callback routes
		r.HandleFunc("/{provider}", paymentHandler.HandleCallback)
	})

	// Webhook routes for payment notifications
	r.Route("/webhooks", func(r chi.Router) {
		// Provider-specific webhook routes
		r.Post("/{provider}", paymentHandler.HandleWebhook)
	})

	// Stats endpoint for logging statistics (handled by middleware)
	// GET /v1/stats?provider=iyzico&hours=24
}
