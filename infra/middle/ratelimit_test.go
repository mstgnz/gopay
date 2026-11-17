package middle

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// GetClientIP keys the rate limits and the login throttle and is the clientIp sent to the
// providers, so a caller-set header must never decide it. Run through a chi router with
// ClientIP mounted, as cmd/main.go does.
func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{"X-Real-IP from the host nginx", map[string]string{"X-Real-IP": "192.0.2.10"}, "172.18.0.1:40000", "192.0.2.10"},
		{"IPv6 X-Real-IP", map[string]string{"X-Real-IP": "2001:db8::7"}, "172.18.0.1:40000", "2001:db8::7"},
		{"no header falls back to the peer", nil, "192.0.2.20:51234", "192.0.2.20"},
		{"IPv6 peer", nil, "[2001:db8::1]:443", "2001:db8::1"},
		{"spoofed True-Client-IP and XFF ignored", map[string]string{
			"X-Real-IP":       "192.0.2.10",
			"True-Client-IP":  "198.51.100.7",
			"X-Forwarded-For": "198.51.100.7, 192.0.2.10, 172.18.0.1",
		}, "172.18.0.3:40000", "192.0.2.10"},
		{"XFF alone never decides", map[string]string{
			"X-Forwarded-For": "198.51.100.7",
			"True-Client-IP":  "198.51.100.8",
		}, "172.18.0.3:40000", "172.18.0.3"},
		{"unparsable X-Real-IP falls back to the peer", map[string]string{"X-Real-IP": "not-an-ip"}, "172.18.0.3:40000", "172.18.0.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			r := chi.NewRouter()
			r.Use(ClientIP())
			r.Post("/v1/auth/login", func(w http.ResponseWriter, r *http.Request) { got = GetClientIP(r) })

			req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			r.ServeHTTP(httptest.NewRecorder(), req)

			if got != tt.want {
				t.Fatalf("GetClientIP = %q, want %q", got, tt.want)
			}
			if req.RemoteAddr != tt.remoteAddr {
				t.Fatalf("RemoteAddr was rewritten to %q", req.RemoteAddr)
			}
		})
	}
}

// RateLimitMiddleware must key on X-Real-IP: a client rotating X-Forwarded-For, arriving through
// different proxy replicas (different peers), stays in one bucket.
func TestRateLimitKeysOnRealIP(t *testing.T) {
	rl := &RateLimiter{visitors: map[string]*visitor{}, rate: 2, window: time.Minute}
	r := chi.NewRouter()
	r.Use(ClientIP())
	r.Use(RateLimitMiddleware(rl))
	r.Get("/x", func(w http.ResponseWriter, r *http.Request) {})

	codes := []int{}
	for i, xff := range []string{"198.51.100.1", "198.51.100.2", "198.51.100.3"} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Real-IP", "192.0.2.10")
		req.Header.Set("X-Forwarded-For", xff)
		req.RemoteAddr = fmt.Sprintf("172.18.0.%d:40000", i+1)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Fatalf("one X-Real-IP escaped the limit through rotating headers or peers: %v", codes)
	}
}
