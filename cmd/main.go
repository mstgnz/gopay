package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/response"
	"github.com/mstgnz/gopay/infra/validate"
	"github.com/mstgnz/gopay/router"
)

var (
	PORT string
)

func init() {
	// Load Env
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Load Env Error: %v", err)
	}
	// init conf
	_ = config.App()
	validate.CustomValidate()

	PORT = os.Getenv("APP_PORT")
}

func main() {

	// Chi Define Routes
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Timestamp", "Hash", "Origin", "X-Requested-With"},
		ExposedHeaders:   []string{"Link", "Content-Length", "Access-Control-Allow-Origin"},
		AllowCredentials: true,
		MaxAge:           300, // Preflight cache time (second)
	}))

	workDir, _ := os.Getwd()
	fileServer(r, "/asset", http.Dir(filepath.Join(workDir, "asset")))

	// scalar
	r.Get("/scalar", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./view/scalar.html")
	})

	r.Route("/v1", func(r chi.Router) {
		router.Routes(r)
	})

	// Not Found
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		_ = response.WriteJSON(w, http.StatusUnauthorized, response.Response{Success: false, Message: "Not Found"})
	})

	// Create a context that listens for interrupt and terminate signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)
	defer stop()

	// Run your HTTP server in a goroutine
	go func() {
		server := &http.Server{
			Addr:              fmt.Sprintf(":%s", PORT),
			Handler:           r,
			ReadTimeout:       60 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 60 * time.Second,
		}
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err.Error())
		}
	}()

	log.Println("API is running on", PORT)

	// Block until a signal is received
	<-ctx.Done()

	log.Println("API is shutting on", PORT)
	log.Println("Shutting down gracefully...")

}

func fileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
