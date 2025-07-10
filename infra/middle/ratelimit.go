package middle

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/response"
)

// RateLimiter represents a simple rate limiter
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int
	window   time.Duration
}

type visitor struct {
	count     int
	lastReset time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	// Get rate limit from environment (default: 100 requests per minute)
	rateStr := config.GetEnv("RATE_LIMIT_PER_MINUTE", "100")
	rate := 100
	if rateStr != "" {
		if r, err := strconv.Atoi(rateStr); err == nil && r > 0 {
			rate = r
		}
	}

	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		window:   time.Minute,
	}

	// Cleanup routine
	go rl.cleanup()

	return rl
}

// Allow checks if the request is allowed
func (rl *RateLimiter) Allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[clientIP]

	if !exists || now.Sub(v.lastReset) > rl.window {
		rl.visitors[clientIP] = &visitor{
			count:     1,
			lastReset: now,
		}
		return true
	}

	if v.count >= rl.rate {
		return false
	}

	v.count++
	return true
}

// cleanup removes old entries
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, v := range rl.visitors {
			if now.Sub(v.lastReset) > rl.window*2 {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP
			clientIP := GetClientIP(r)

			// Check rate limit
			if !rl.Allow(clientIP) {
				response.Error(w, http.StatusTooManyRequests, "Rate limit exceeded", nil)
				return
			}

			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// GetClientIP extracts the real client IP
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in case of multiple
		if idx := strings.Index(xff, ","); idx != -1 {
			ip := strings.TrimSpace(xff[:idx])
			return ip
		}
		ip := strings.TrimSpace(xff)
		return ip
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip := strings.TrimSpace(xri)
		return ip
	}

	// Fall back to RemoteAddr
	remoteAddr := r.RemoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		ip := remoteAddr[:idx]
		// Handle IPv6 localhost addresses
		if ip == "[::1]" {
			return "127.0.0.1"
		}
		return ip
	}

	// Handle case where RemoteAddr doesn't have port
	if remoteAddr == "[::1]" {
		return "127.0.0.1"
	}

	return remoteAddr
}
