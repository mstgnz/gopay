package v1

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/handler"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/postgres"
	"github.com/mstgnz/gopay/provider"

	// Import for side-effect registration
	_ "github.com/mstgnz/gopay/provider/iyzico"
	_ "github.com/mstgnz/gopay/provider/nkolay"
	_ "github.com/mstgnz/gopay/provider/ozanpay"
	_ "github.com/mstgnz/gopay/provider/papara"
	_ "github.com/mstgnz/gopay/provider/paycell"
	_ "github.com/mstgnz/gopay/provider/paytr"
	_ "github.com/mstgnz/gopay/provider/payu"
	_ "github.com/mstgnz/gopay/provider/stripe"
)

// Routes defines all v1 API routes
func Routes(r chi.Router, postgresLogger *postgres.Logger, paymentService *provider.PaymentService, providerConfig *config.ProviderConfig) {
	// Initialize handlers
	validator := validator.New()
	paymentHandler := handler.NewPaymentHandler(paymentService, validator)
	configHandler := handler.NewConfigHandler(providerConfig, paymentService, validator)

	// Initialize provider-specific logger for logs handler
	providerLogger := provider.NewProviderSpecificLogger(config.App().DB)
	logsHandler := handler.NewLogsHandler(providerLogger, postgresLogger)

	// Payment routes (JWT protected)
	r.Route("/payments", func(r chi.Router) {
		r.Post("/{provider}", paymentHandler.ProcessPayment)
		r.Get("/{provider}/{paymentID}", paymentHandler.GetPaymentStatus)
		r.Delete("/{provider}/{paymentID}", paymentHandler.CancelPayment)
		r.Post("/{provider}/refund", paymentHandler.RefundPayment)
	})

	// Configuration routes (JWT protected)
	r.Route("/config", func(r chi.Router) {
		r.Post("/tenant", configHandler.PostTenantConfig)
		r.Get("/tenant", configHandler.GetTenantConfig)
		r.Delete("/tenant", configHandler.DeleteTenantConfig)
	})

	// Logs routes (JWT protected)
	r.Route("/logs", func(r chi.Router) {
		r.Get("/{provider}", logsHandler.ListLogs)                           // GET /v1/logs/{provider}?status=success&hours=24
		r.Get("/{provider}/payment/{paymentID}", logsHandler.GetPaymentLogs) // GET /v1/logs/{provider}/payment/{paymentID}
		r.Get("/{provider}/errors", logsHandler.GetErrorLogs)                // GET /v1/logs/{provider}/errors?hours=24
		r.Get("/{provider}/stats", logsHandler.GetLogStats)                  // GET /v1/logs/{provider}/stats?hours=24
	})
}
