package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/mstgnz/gopay/infra/opensearch"
	v1 "github.com/mstgnz/gopay/router/v1"

	// Import for side-effect registration
	_ "github.com/mstgnz/gopay/provider/iyzico"
	_ "github.com/mstgnz/gopay/provider/nkolay"
	_ "github.com/mstgnz/gopay/provider/ozanpay"
	_ "github.com/mstgnz/gopay/provider/papara"
	_ "github.com/mstgnz/gopay/provider/paycell"
	_ "github.com/mstgnz/gopay/provider/stripe"
)

func Routes(r chi.Router, logger *opensearch.Logger) {
	v1.Routes(r, logger)
}
