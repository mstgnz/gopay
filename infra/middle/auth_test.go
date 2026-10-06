package middle

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mstgnz/gopay/infra/auth"
)

type stubChecker struct {
	err  error
	seen *[]*auth.JWTClaims
}

func (s stubChecker) CheckTokenCurrent(claims *auth.JWTClaims) error {
	if s.seen != nil {
		*s.seen = append(*s.seen, claims)
	}
	return s.err
}

func serveWithToken(t *testing.T, checker TokenChecker, token string) *httptest.ResponseRecorder {
	t.Helper()
	jwtService := auth.NewJWTServiceWithSecret("test-secret", time.Hour)
	handler := JWTAuthMiddleware(jwtService, checker)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetTenantIDFromContext(r.Context()) != "2" {
			t.Errorf("tenant id not propagated, got %q", GetTenantIDFromContext(r.Context()))
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/payments/paycell/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func signedToken(t *testing.T, fingerprint string) string {
	t.Helper()
	token, err := auth.NewJWTServiceWithSecret("test-secret", time.Hour).GenerateToken("2", "tenant-a", fingerprint, time.Now())
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return token
}

func TestJWTAuthMiddleware_CurrentTokenPasses(t *testing.T) {
	var seen []*auth.JWTClaims
	rr := serveWithToken(t, stubChecker{seen: &seen}, signedToken(t, "abcdef0123456789"))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(seen) != 1 || seen[0].PasswordFingerprint != "abcdef0123456789" {
		t.Fatalf("checker did not receive the token claims: %+v", seen)
	}
}

func TestJWTAuthMiddleware_RevokedTokenIs401(t *testing.T) {
	rr := serveWithToken(t, stubChecker{err: auth.ErrTokenRevoked}, signedToken(t, "abcdef0123456789"))

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

// A database outage must not read as "your credentials are wrong": clients that re-login on
// 401 would hammer login while the real failure is elsewhere.
func TestJWTAuthMiddleware_LookupFailureIs503(t *testing.T) {
	rr := serveWithToken(t, stubChecker{err: errors.New("connection refused")}, signedToken(t, "abcdef0123456789"))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestJWTAuthMiddleware_TokenFromOtherSecretRejected(t *testing.T) {
	foreign, err := auth.NewJWTServiceWithSecret("other-secret", time.Hour).GenerateToken("2", "tenant-a", "", time.Now())
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	var seen []*auth.JWTClaims
	rr := serveWithToken(t, stubChecker{seen: &seen}, foreign)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if len(seen) != 0 {
		t.Fatal("revocation lookup ran for a token with a bad signature")
	}
}

func TestJWTAuthMiddleware_NilCheckerPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a nil TokenChecker")
		}
	}()
	JWTAuthMiddleware(auth.NewJWTServiceWithSecret("s", time.Hour), nil)
}
