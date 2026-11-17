package middle

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
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

// ClientIP resolves the client address once per request from X-Real-IP, the one header the host
// nginx overwrites on every request (from CF-Connecting-IP through its realip module); the docker
// nginx passes it through. X-Forwarded-For and True-Client-IP are caller-controlled and never read.
// Mount it before every middleware that calls GetClientIP.
func ClientIP() func(http.Handler) http.Handler {
	return middleware.ClientIPFromHeader("X-Real-IP")
}

// GetClientIP returns the address ClientIP resolved. A request that did not pass the host nginx
// (no X-Real-IP, e.g. a health check inside the docker network) falls back to its TCP peer.
// It keys the rate limits and the login throttle and is the clientIp sent to the providers.
func GetClientIP(r *http.Request) string {
	if ip := middleware.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
