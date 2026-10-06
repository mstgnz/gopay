package middle

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

// ProxyClientIP keys the login throttle, so a caller-set header must never decide it.
func TestProxyClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{"X-Real-IP from the host nginx", map[string]string{"X-Real-IP": " 192.0.2.10 "}, "172.18.0.1:40000", "192.0.2.10"},
		{"no header falls back to the peer", nil, "192.0.2.20:51234", "192.0.2.20"},
		{"IPv6 peer", nil, "[2001:db8::1]:443", "2001:db8::1"},
		{"spoofed True-Client-IP and XFF ignored", map[string]string{
			"X-Real-IP":       "192.0.2.10",
			"True-Client-IP":  "198.51.100.7",
			"X-Forwarded-For": "198.51.100.7, 192.0.2.10, 172.18.0.1",
		}, "172.18.0.3:40000", "192.0.2.10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			// Run behind chi RealIP as cmd/main.go does: it rewrites RemoteAddr from
			// True-Client-IP, which must not leak into the result.
			var got string
			middleware.RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = ProxyClientIP(r)
			})).ServeHTTP(httptest.NewRecorder(), req)

			if got != tt.want {
				t.Fatalf("ProxyClientIP = %q, want %q", got, tt.want)
			}
		})
	}
}
