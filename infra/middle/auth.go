package middle

import (
	"context"
	"net/http"
	"strings"

	"github.com/mstgnz/gopay/infra/auth"
	"github.com/mstgnz/gopay/infra/response"
)

// TenantContextKey is the key for tenant information in request context
type TenantContextKey string

const (
	TenantIDKey     TenantContextKey = "tenant_id"
	TenantUserKey   TenantContextKey = "tenant_user"
	TenantClaimsKey TenantContextKey = "tenant_claims"
)

// JWTAuthMiddleware validates JWT token authentication
func JWTAuthMiddleware(jwtService *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Error(w, http.StatusUnauthorized, "Authorization header required", nil)
				return
			}

			// Check Bearer token format
			if !strings.HasPrefix(authHeader, "Bearer ") {
				response.Error(w, http.StatusUnauthorized, "Invalid authorization format. Use: Bearer <jwt_token>", nil)
				return
			}

			// Extract JWT token
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				response.Error(w, http.StatusUnauthorized, "JWT token required", nil)
				return
			}

			// Validate JWT token
			claims, err := jwtService.ValidateToken(token)
			if err != nil {
				switch err {
				case auth.ErrExpiredToken:
					response.Error(w, http.StatusUnauthorized, "Token has expired", nil)
				case auth.ErrInvalidToken:
					response.Error(w, http.StatusUnauthorized, "Invalid token", nil)
				case auth.ErrInvalidClaims:
					response.Error(w, http.StatusUnauthorized, "Invalid token claims", nil)
				case auth.ErrMissingTenant:
					response.Error(w, http.StatusUnauthorized, "Missing tenant information in token", nil)
				default:
					response.Error(w, http.StatusUnauthorized, "Token validation failed", nil)
				}
				return
			}

			// Add tenant information to request context
			ctx := context.WithValue(r.Context(), TenantIDKey, claims.TenantID)
			ctx = context.WithValue(ctx, TenantUserKey, claims.Username)
			ctx = context.WithValue(ctx, TenantClaimsKey, claims)

			// Continue to next handler with enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LegacyAPIKeyMiddleware validates API key authentication (for backward compatibility)
func LegacyAPIKeyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get API key from environment
			expectedAPIKey := "gopay-api-key-2024" // Fixed API key for now

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

			// Add default tenant information to request context for legacy compatibility
			ctx := context.WithValue(r.Context(), TenantIDKey, "legacy")

			// Continue to next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTenantIDFromContext extracts tenant ID from request context
func GetTenantIDFromContext(ctx context.Context) string {
	if tenantID, ok := ctx.Value(TenantIDKey).(string); ok {
		return tenantID
	}
	return ""
}

// GetTenantUserFromContext extracts tenant username from request context
func GetTenantUserFromContext(ctx context.Context) string {
	if username, ok := ctx.Value(TenantUserKey).(string); ok {
		return username
	}
	return ""
}

// GetTenantClaimsFromContext extracts JWT claims from request context
func GetTenantClaimsFromContext(ctx context.Context) *auth.JWTClaims {
	if claims, ok := ctx.Value(TenantClaimsKey).(*auth.JWTClaims); ok {
		return claims
	}
	return nil
}
