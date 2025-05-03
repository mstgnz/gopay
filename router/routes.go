package router

import (
	"github.com/go-chi/chi/v5"
	v1 "github.com/mstgnz/gopay/router/v1"
)

func Routes(r chi.Router) {
	v1.Routes(r)
}
