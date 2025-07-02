package middle

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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
	rateStr := os.Getenv("RATE_LIMIT_PER_MINUTE")
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

	for {
		select {
		case <-ticker.C:
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
}

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP
			clientIP := getClientIP(r)

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

// getClientIP extracts the real client IP
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in case of multiple
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}

	return r.RemoteAddr
}
