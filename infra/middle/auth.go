package middle

import (
	"net/http"
	"strings"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/response"
)

// AuthMiddleware validates API key authentication
func AuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get API key from environment
			expectedAPIKey := config.GetEnv("API_KEY", "")
			if expectedAPIKey == "" {
				response.Error(w, http.StatusInternalServerError, "API key not configured", nil)
				return
			}

			// Get Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Error(w, http.StatusUnauthorized, "Authorization header required", nil)
				return
			}

			// Check Bearer token format
			if !strings.HasPrefix(authHeader, "Bearer ") {
				response.Error(w, http.StatusUnauthorized, "Invalid authorization format. Use: Bearer <api_key>", nil)
				return
			}

			// Extract API key
			apiKey := strings.TrimPrefix(authHeader, "Bearer ")
			if apiKey == "" {
				response.Error(w, http.StatusUnauthorized, "API key required", nil)
				return
			}

			// Validate API key
			if apiKey != expectedAPIKey {
				response.Error(w, http.StatusUnauthorized, "Invalid API key", nil)
				return
			}

			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}
