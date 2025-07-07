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
	"github.com/mstgnz/gopay/infra/auth"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/infra/middle"
	"github.com/mstgnz/gopay/infra/postgres"
	"github.com/mstgnz/gopay/infra/response"
	"github.com/mstgnz/gopay/infra/validate"
	"github.com/mstgnz/gopay/provider"
	v1 "github.com/mstgnz/gopay/router/v1"
)

var (
	PORT           string
	postgresLogger *postgres.Logger
	jwtService     *auth.JWTService
	tenantService  *auth.TenantService
	paymentHandler *handler.PaymentHandler
)

func init() {
	// Load Env
	if err := godotenv.Load(".env"); err != nil {
		logger.Warn(fmt.Sprintf("Load Env Error: %v", err))
		log.Fatalf("Load Env Error: %v", err)
	}
	// init conf
	_ = config.App()
	validate.CustomValidate()

	PORT = config.GetEnv("APP_PORT", "9999")

	// Test connection
	if err := config.App().DB.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	// Initialize PostgreSQL logger
	cfg := config.GetAppConfig()
	if cfg.EnableLogging {
		postgresLogger = postgres.NewLogger(config.App().DB)
		log.Println("PostgreSQL logging initialized successfully")
	} else {
		log.Println("PostgreSQL logging is disabled")
	}

	// Initialize JWT service
	jwtSecret := config.App().SecretKey
	jwtIssuer := config.GetEnv("JWT_ISSUER", "gopay")
	jwtExpiry := 24 * time.Hour // 24 hours
	jwtService = auth.NewJWTService(jwtSecret, jwtIssuer, jwtExpiry)

	// Initialize tenant service
	tenantService = auth.NewTenantService(config.App().DB, jwtService)

	// Initialize global system logger
	logger.InitGlobalLogger(postgresLogger)
}

func main() {
	// Use structured logging from now on
	logger.Info("Starting GoPay application", logger.LogContext{
		Fields: map[string]any{
			"port":             PORT,
			"postgres_enabled": postgresLogger != nil,
		},
	})

	// Initialize global services for callback handlers
	paymentLogger := provider.NewDBPaymentLogger(config.App().DB.DB)
	paymentService := provider.NewPaymentService(paymentLogger)
	providerConfig := config.NewProviderConfig()
	providerConfig.LoadFromEnv()

	// Register providers (similar to v1 init)
	for _, providerName := range providerConfig.GetAvailableProviders() {
		providerCfg, err := providerConfig.GetConfig(providerName)
		if err != nil {
			logger.Error("Failed to get configuration for provider", err, logger.LogContext{
				Provider: providerName,
			})
			continue
		}
		providerCfg["gopayBaseURL"] = providerConfig.GetBaseURL()
		if err := paymentService.AddProvider(providerName, providerCfg); err != nil {
			logger.Error("Failed to register provider", err, logger.LogContext{
				Provider: providerName,
			})
			continue
		}
	}

	// Set default provider
	availableProviders := providerConfig.GetAvailableProviders()
	if len(availableProviders) > 0 {
		paymentService.SetDefaultProvider(availableProviders[0])
	}

	// Initialize payment handler
	validatorInstance := validator.New()
	paymentHandler = handler.NewPaymentHandler(paymentService, validatorInstance)

	// Chi Define Routes
	r := chi.NewRouter()

	// Basic Middleware
	r.Use(middle.PanicRecoveryMiddleware())
	r.Use(middleware.Logger)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(60 * time.Second))

	// Security Middleware
	rateLimiter := middle.NewRateLimiter()
	r.Use(middle.SecurityHeadersMiddleware())
	r.Use(middle.IPWhitelistMiddleware())
	r.Use(middle.RateLimitMiddleware(rateLimiter))
	r.Use(middle.RequestValidationMiddleware())

	// PostgreSQL Logging Middleware (add before authentication to log all requests)
	if postgresLogger != nil {
		r.Use(middle.PaymentLoggingMiddleware(postgresLogger))
		r.Use(middle.LoggingStatsMiddleware(postgresLogger))
		logger.Info("Payment logging middleware enabled")
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
			"status":           "ok",
			"timestamp":        time.Now().UTC(),
			"version":          "1.0.0",
			"postgres_enabled": postgresLogger != nil,
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

	// Analytics Dashboard (Main Page)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(workDir, "public", "index.html"))
	})

	// API Documentation (Scalar)
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(workDir, "public", "scalar.html"))
	})

	// Auth routes are now handled in v1.Routes()

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

	// Public v1 auth routes (no authentication required)
	r.Route("/v1/auth", func(r chi.Router) {
		// Initialize auth handler
		validatorInstance := validator.New()
		authHandler := handler.NewAuthHandler(tenantService, jwtService, validatorInstance)

		r.Post("/login", authHandler.Login)
		r.Post("/register", authHandler.Register) // Public registration (only if no users exist)
		r.Post("/refresh", authHandler.RefreshToken)
		r.Post("/validate", authHandler.ValidateToken)
	})

	// Protected v1 routes with authentication
	r.Route("/v1", func(r chi.Router) {
		// Add JWT authentication middleware only to protected routes
		r.Use(middle.JWTAuthMiddleware(jwtService))

		// Import v1 routes with required services (but exclude auth routes since they're handled above)
		v1.Routes(r, postgresLogger, paymentService, providerConfig, jwtService, tenantService)
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
			logger.Fatal("Server failed to start", err)
		}
	}()

	logger.Info("API is running", logger.LogContext{
		Fields: map[string]any{
			"port": PORT,
		},
	})

	// Block until a signal is received
	<-ctx.Done()

	logger.Info("Shutting down gracefully", logger.LogContext{
		Fields: map[string]any{
			"port": PORT,
		},
	})
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
