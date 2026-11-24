package middle

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mstgnz/gopay/infra/auth"
)

// tightLimiter allows one unauthenticated request per IP and three per tenant, per minute.
func tightLimiter() *TenantRateLimiter {
	return &TenantRateLimiter{
		tenants: make(map[string]*tenantBucket),
		ips:     make(map[string]*visitor),
		config: &TenantRateLimitConfig{
			DefaultGlobalRate:   3,
			DefaultPaymentRate:  3,
			DefaultRefundRate:   3,
			DefaultStatusRate:   3,
			DefaultWindow:       time.Minute,
			UnauthenticatedRate: 1,
			TenantOverrides:     make(map[string]*TenantLimits),
			PremiumTenants:      make(map[string]bool),
		},
	}
}

func tokenFor(t *testing.T, jwt *auth.JWTService, tenant string) string {
	t.Helper()
	token, err := jwt.GenerateToken(tenant, "user-"+tenant, "abcdef0123456789", time.Now())
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return token
}

func statusRequest(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/v1/payments/paycell/pay-1", nil)
	req.RemoteAddr = "203.0.113.7:4000" // one consumer server for every tenant
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// A consumer server polling status must be counted against its tenant's limit, not the per-IP
// unauthenticated one; that limit capped tenant 2 at 10 requests a minute per replica.
func TestTenantAuthLimitsByTenantAfterAuthentication(t *testing.T) {
	jwt := auth.NewJWTServiceWithSecret("test-secret", time.Hour)
	handler := TenantAuth(jwt, stubChecker{}, tightLimiter())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	serve := func(token string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, statusRequest(token))
		return rr
	}

	// An invalid token is refused before the limiter and spends nobody's budget.
	if rr := serve("not-a-token"); rr.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token: status %d, want 401", rr.Code)
	}

	tenant2 := tokenFor(t, jwt, "2")
	for i := 1; i <= 3; i++ { // three times the unauthenticated limit, from one IP
		rr := serve(tenant2)
		if rr.Code != http.StatusOK {
			t.Fatalf("tenant 2 request %d: status %d, want 200", i, rr.Code)
		}
		if got := rr.Header().Get("X-RateLimit-Tenant"); got != "2" {
			t.Fatalf("tenant 2 request %d counted as %q, want tenant 2", i, got)
		}
	}
	if rr := serve(tenant2); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("tenant 2 over its limit: status %d, want 429", rr.Code)
	}

	// Another tenant behind the same IP has its own budget.
	if rr := serve(tokenFor(t, jwt, "3")); rr.Code != http.StatusOK {
		t.Fatalf("tenant 3 after tenant 2 was limited: status %d, want 200", rr.Code)
	}
}

// A signature-valid token the checker refuses (password changed, tenant gone) is answered 401 before
// the limiter: it must not spend the tenant's budget, or a leaked old token could 429 its payments.
func TestTenantAuthRevokedTokenSpendsNoBudget(t *testing.T) {
	jwt := auth.NewJWTServiceWithSecret("test-secret", time.Hour)
	trl := tightLimiter()
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	revoked := TenantAuth(jwt, stubChecker{err: auth.ErrTokenRevoked}, trl)(ok)
	current := TenantAuth(jwt, stubChecker{}, trl)(ok)
	token := tokenFor(t, jwt, "2")

	for i := range 10 {
		rr := httptest.NewRecorder()
		revoked.ServeHTTP(rr, statusRequest(token))
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("revoked token %d: status %d, want 401", i+1, rr.Code)
		}
	}
	for i := 1; i <= 3; i++ {
		rr := httptest.NewRecorder()
		current.ServeHTTP(rr, statusRequest(token))
		if rr.Code != http.StatusOK {
			t.Fatalf("tenant 2 request %d after the revoked ones: status %d, want 200", i, rr.Code)
		}
	}
}

// Routes without a tenant (3D callbacks, webhooks) keep the per-IP limit.
func TestTenantRateLimitMiddlewareKeepsCallbacksPerIP(t *testing.T) {
	handler := TenantRateLimitMiddleware(tightLimiter())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodPost, "/v1/callback/paycell?state=x", nil)
		req.RemoteAddr = "198.51.100.9:5000"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != want {
			t.Fatalf("callback %d: status %d, want %d", i+1, rr.Code, want)
		}
	}
}
