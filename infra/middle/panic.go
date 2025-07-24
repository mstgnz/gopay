package middle

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/infra/response"
)

// PanicRecoveryMiddleware handles panics and converts them to HTTP 500 errors
func PanicRecoveryMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Get stack trace
					stack := debug.Stack()

					// Extract request information for logging
					tenantID := GetTenantIDFromContext(r.Context())
					requestID := r.Header.Get("X-Request-ID")
					if requestID == "" {
						requestID = "unknown"
					}

					// Safe logging to standard logger (fallback)
					log.Printf("PANIC RECOVERED: %v | Method: %s | URL: %s | Tenant: %s | Request ID: %s | Time: %s",
						err, r.Method, r.URL.String(), tenantID, requestID, time.Now().UTC().Format(time.RFC3339))

					// Log stack trace separately to avoid log line length issues
					log.Printf("PANIC STACK TRACE: %s", string(stack))

					logger.Error("Panic recovered", fmt.Errorf("%v", err), logger.LogContext{
						Provider: "gopay",
						TenantID: tenantID,
						Fields: map[string]any{
							"request_id": requestID,
							"method":     r.Method,
							"url":        r.URL.String(),
							"stack":      string(stack),
						},
					})

					// Set headers to prevent caching of error response
					w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Set("Pragma", "no-cache")
					w.Header().Set("Expires", "0")

					// Return structured error response
					response.Error(w, http.StatusInternalServerError, "Internal server error", fmt.Errorf("an unexpected error occurred"))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// PanicRecoveryWithCustomHandler allows custom panic handling
func PanicRecoveryWithCustomHandler(handler func(http.ResponseWriter, *http.Request, any)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					handler(w, r, err)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
