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
	analyticsHandler := handler.NewAnalyticsHandler(postgresLogger)
	logsHandler := handler.NewLogsHandler(nil, postgresLogger)

	// Protected auth endpoints (JWT authentication already applied in main.go)
	r.Post("/auth/logout", authHandler.Logout)
	r.Post("/auth/change-password", authHandler.ChangePassword)
	r.Get("/auth/profile", authHandler.GetProfile)
	r.Post("/auth/create-tenant", authHandler.CreateTenant) // Admin-only tenant creation

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

	// Analytics routes (JWT protected)
	r.Route("/analytics", func(r chi.Router) {
		r.Get("/dashboard", analyticsHandler.GetDashboardStats) // GET /v1/analytics/dashboard?hours=24
		r.Get("/providers", analyticsHandler.GetProviderStats)  // GET /v1/analytics/providers
		r.Get("/activity", analyticsHandler.GetRecentActivity)  // GET /v1/analytics/activity?limit=10
		r.Get("/trends", analyticsHandler.GetPaymentTrends)     // GET /v1/analytics/trends?hours=24
	})

	// Logs routes (JWT protected)
	r.Route("/logs", func(r chi.Router) {
		r.Get("/{provider}", logsHandler.ListLogs)                           // GET /v1/logs/{provider}?status=success&hours=24
		r.Get("/{provider}/payment/{paymentID}", logsHandler.GetPaymentLogs) // GET /v1/logs/{provider}/payment/{paymentID}
		r.Get("/{provider}/errors", logsHandler.GetErrorLogs)                // GET /v1/logs/{provider}/errors?hours=24
		r.Get("/{provider}/stats", logsHandler.GetLogStats)                  // GET /v1/logs/{provider}/stats?hours=24
	})
}
