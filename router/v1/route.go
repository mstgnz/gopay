package v1

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/handler"
	"github.com/mstgnz/gopay/infra/auth"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/postgres"
	"github.com/mstgnz/gopay/provider"
)

// Routes defines all v1 API routes
func Routes(r chi.Router, postgresLogger *postgres.Logger, paymentService *provider.PaymentService, providerConfig *config.ProviderConfig, jwtService *auth.JWTService, tenantService *auth.TenantService) {
	// Initialize handlers
	validator := validator.New()
	paymentHandler := handler.NewPaymentHandler(paymentService, validator)
	configHandler := handler.NewConfigHandler(providerConfig, paymentService, validator)
	authHandler := handler.NewAuthHandler(tenantService, jwtService, validator)

	// Protected auth endpoints (JWT authentication already applied in main.go)
	r.Post("/auth/logout", authHandler.Logout)
	r.Post("/auth/change-password", authHandler.ChangePassword)
	r.Get("/auth/profile", authHandler.GetProfile)
	r.Post("/auth/create-tenant", authHandler.CreateTenant) // Admin-only tenant creation

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

	// Legacy routes for backward compatibility
	r.Route("/set-env", func(r chi.Router) {
		r.Post("/", configHandler.SetEnv)
	})
}
