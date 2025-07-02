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
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/mstgnz/gopay/handler"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/middle"
	"github.com/mstgnz/gopay/infra/opensearch"
	"github.com/mstgnz/gopay/infra/response"
	"github.com/mstgnz/gopay/infra/validate"
	"github.com/mstgnz/gopay/provider"
	v1 "github.com/mstgnz/gopay/router/v1"
)

var (
	PORT             string
	openSearchLogger *opensearch.Logger
	paymentHandler   *handler.PaymentHandler
)

func init() {
	// Load Env
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Load Env Error: %v", err)
	}
	// init conf
	_ = config.App()
	validate.CustomValidate()

	PORT = config.GetEnv("APP_PORT", "9999")

	// Initialize OpenSearch client and logger
	cfg := config.GetAppConfig()
	if cfg.EnableLogging {
		osClient, err := opensearch.NewClient(cfg)
		if err != nil {
			log.Printf("Failed to initialize OpenSearch client: %v", err)
			log.Println("Continuing without OpenSearch logging...")
		} else {
			openSearchLogger = opensearch.NewLogger(osClient)
			log.Println("OpenSearch logging initialized successfully")
		}
	} else {
		log.Println("OpenSearch logging is disabled")
	}
}

func main() {
	// Initialize global services for callback handlers
	paymentService := provider.NewPaymentService()
	providerConfig := config.NewProviderConfig()
	providerConfig.LoadFromEnv()

	// Register providers (similar to v1 init)
	for _, providerName := range providerConfig.GetAvailableProviders() {
		providerCfg, err := providerConfig.GetConfig(providerName)
		if err != nil {
			log.Printf("Failed to get configuration for provider %s: %v", providerName, err)
			continue
		}
		providerCfg["gopayBaseURL"] = providerConfig.GetBaseURL()
		if err := paymentService.AddProvider(providerName, providerCfg); err != nil {
			log.Printf("Failed to register provider %s: %v", providerName, err)
			continue
		}
	}

	// Set default provider
	availableProviders := providerConfig.GetAvailableProviders()
	if len(availableProviders) > 0 {
		paymentService.SetDefaultProvider(availableProviders[0])
	}

	// Initialize payment handler
	validator := validator.New()
	paymentHandler = handler.NewPaymentHandler(paymentService, validator)

	// Chi Define Routes
	r := chi.NewRouter()

	// Basic Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Security Middleware
	rateLimiter := middle.NewRateLimiter()
	r.Use(middle.SecurityHeadersMiddleware())
	r.Use(middle.IPWhitelistMiddleware())
	r.Use(middle.RateLimitMiddleware(rateLimiter))
	r.Use(middle.RequestValidationMiddleware())

	// OpenSearch Logging Middleware (add before authentication to log all requests)
	if openSearchLogger != nil {
		r.Use(middle.PaymentLoggingMiddleware(openSearchLogger))
		r.Use(middle.LoggingStatsMiddleware(openSearchLogger))
		log.Println("Payment logging middleware enabled")
	}

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
	fileServer(r, "/public", http.Dir(filepath.Join(workDir, "public")))

	// Health check endpoint (no auth required)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		health := map[string]any{
			"status":             "ok",
			"timestamp":          time.Now().UTC(),
			"version":            "1.0.0",
			"opensearch_enabled": openSearchLogger != nil,
		}
		_ = response.WriteJSON(w, http.StatusOK, response.Response{
			Success: true,
			Message: "Service is healthy",
			Data:    health,
		})
	})

	// scalar
	r.Get("/scalar.yaml", func(w http.ResponseWriter, r *http.Request) {
		// Read the scalar file
		scalarContent, err := os.ReadFile(filepath.Join(workDir, "public", "scalar.yaml"))
		if err != nil {
			http.Error(w, "Failed to read scalar file", http.StatusInternalServerError)
			return
		}

		// Replace environment variables
		scalarContent = []byte(strings.ReplaceAll(string(scalarContent), "${APP_URL}", config.GetEnv("APP_URL", "http://localhost:9999")))

		// Set content type and send the modified content
		w.Header().Set("Content-Type", "text/yaml")
		w.Write(scalarContent)
	})

	// Index
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(workDir, "public", "scalar.html"))
	})

	// Callback routes for payment providers (no auth required)
	r.Route("/callback", func(r chi.Router) {
		// General callback route (uses default provider)
		r.HandleFunc("/", paymentHandler.HandleCallback)

		// Provider-specific callback routes
		r.HandleFunc("/{provider}", paymentHandler.HandleCallback)
	})

	// Webhook routes for payment notifications (no auth required)
	r.Route("/webhooks", func(r chi.Router) {
		// Provider-specific webhook routes
		r.Post("/{provider}", paymentHandler.HandleWebhook)
	})

	// API routes with authentication
	r.Route("/v1", func(r chi.Router) {
		// Add authentication middleware only to API routes
		r.Use(middle.AuthMiddleware())

		// Import v1 routes
		v1.Routes(r, openSearchLogger)
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
