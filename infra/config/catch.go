package config

import (
	"log"
	"net/http"
)

type HttpHandler func(w http.ResponseWriter, r *http.Request) error

func Catch(handler HttpHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			log.Println("HTTP Handler Error", "err", err.Error(), "path", r.URL.Path)
		}
	}
}
