package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/mstgnz/gopay/infra/auth"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/postgres"
	"github.com/mstgnz/gopay/provider"
	v1 "github.com/mstgnz/gopay/router/v1"

	// Import for side-effect registration
	_ "github.com/mstgnz/gopay/provider/iyzico"
	_ "github.com/mstgnz/gopay/provider/nkolay"
	_ "github.com/mstgnz/gopay/provider/ozanpay"
	_ "github.com/mstgnz/gopay/provider/papara"
	_ "github.com/mstgnz/gopay/provider/paycell"
	_ "github.com/mstgnz/gopay/provider/stripe"
)

func Routes(r chi.Router, logger *postgres.Logger, paymentService *provider.PaymentService, providerConfig *config.ProviderConfig, jwtService *auth.JWTService, tenantService *auth.TenantService) {
	v1.Routes(r, logger, paymentService, providerConfig, jwtService, tenantService)
}
