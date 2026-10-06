package middle

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/mstgnz/gopay/infra/auth"
	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/infra/response"
)

// TenantContextKey is the key for tenant information in request context
type TenantContextKey string

const (
	TenantIDKey     TenantContextKey = "tenant_id"
	TenantUserKey   TenantContextKey = "tenant_user"
	TenantClaimsKey TenantContextKey = "tenant_claims"
)

// TokenChecker confirms that a signature-valid token has not been revoked since it was issued.
type TokenChecker interface {
	CheckTokenCurrent(claims *auth.JWTClaims) error
}

// JWTAuthMiddleware validates JWT token authentication
func JWTAuthMiddleware(jwtService *auth.JWTService, checker TokenChecker) func(http.Handler) http.Handler {
	if checker == nil {
		panic("JWTAuthMiddleware: nil TokenChecker would skip revocation")
	}
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

			if !authorizeCurrentToken(w, checker, claims) {
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

// authorizeCurrentToken writes the error response and returns false when the token is revoked
// or revocation cannot be checked. A lookup failure is a 503, not a 401: answering 401 would
// tell clients their credentials are wrong when the database is what failed.
func authorizeCurrentToken(w http.ResponseWriter, checker TokenChecker, claims *auth.JWTClaims) bool {
	err := checker.CheckTokenCurrent(claims)
	switch {
	case err == nil:
		return true
	case errors.Is(err, auth.ErrTokenRevoked), errors.Is(err, auth.ErrInvalidClaims):
		response.Error(w, http.StatusUnauthorized, "Token has been revoked", nil)
	default:
		logger.Error("Token revocation check failed", err, logger.LogContext{TenantID: claims.TenantID})
		response.Error(w, http.StatusServiceUnavailable, "Authentication temporarily unavailable", nil)
	}
	return false
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
