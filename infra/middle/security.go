package middle

import (
	"net/http"
	"strings"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/response"
)

// SecurityHeadersMiddleware adds security headers to responses
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			next.ServeHTTP(w, r)
		})
	}
}

// IPWhitelistMiddleware restricts access to whitelisted IPs (optional)
func IPWhitelistMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get whitelist from environment
			whitelist := config.GetEnv("IP_WHITELIST", "")
			if whitelist == "" {
				// If no whitelist configured, allow all
				next.ServeHTTP(w, r)
				return
			}

			// Parse whitelist
			allowedIPs := strings.Split(whitelist, ",")
			clientIP := getClientIP(r)

			// Check if client IP is whitelisted
			allowed := false
			for _, ip := range allowedIPs {
				if strings.TrimSpace(ip) == clientIP {
					allowed = true
					break
				}
			}

			if !allowed {
				response.Error(w, http.StatusForbidden, "IP not whitelisted", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestValidationMiddleware validates common request properties
func RequestValidationMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check Content-Type for POST/PUT requests
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
				contentType := r.Header.Get("Content-Type")
				if contentType != "" && !strings.Contains(contentType, "application/json") &&
					!strings.Contains(contentType, "application/x-www-form-urlencoded") {
					response.Error(w, http.StatusUnsupportedMediaType, "Unsupported content type", nil)
					return
				}
			}

			// Check request size (max 10MB)
			if r.ContentLength > 10*1024*1024 {
				response.Error(w, http.StatusRequestEntityTooLarge, "Request body too large", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
