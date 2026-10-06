package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testHash = "$2a$10$abcdefghijklmnopqrstuuJ8Zq0n7m0bO1y0o3m3r1x9fQyQyQyQy"

func TestPasswordFingerprint_StableAndHashDependent(t *testing.T) {
	a := PasswordFingerprint(testHash)
	if a != PasswordFingerprint(testHash) {
		t.Fatal("fingerprint is not deterministic")
	}
	if len(a) != 16 {
		t.Fatalf("fingerprint length = %d, want 16", len(a))
	}
	if a == PasswordFingerprint(testHash+"x") {
		t.Fatal("different hashes produced the same fingerprint")
	}
}

func TestVerifyFingerprint(t *testing.T) {
	current := PasswordFingerprint(testHash)
	now := time.Now()
	inGrace, afterGrace := now.Add(time.Minute), now.Add(-time.Minute)

	tests := []struct {
		name        string
		claim       string
		hash        string
		legacyUntil time.Time
		wantErr     error
	}{
		{"matching password", current, testHash, inGrace, nil},
		{"matching password after the grace period", current, testHash, afterGrace, nil},
		{"password changed after issue", current, testHash + "changed", inGrace, ErrTokenRevoked},
		{"forged fingerprint", "0000000000000000", testHash, inGrace, ErrTokenRevoked},
		{"pfp-less token inside the grace period", "", testHash, inGrace, nil},
		// A leaked JWT_SECRET alone mints only pfp-less tokens; past the grace they are refused.
		{"pfp-less token after the grace period", "", testHash, afterGrace, ErrTokenRevoked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyFingerprint(&JWTClaims{PasswordFingerprint: tt.claim}, tt.hash, now, tt.legacyUntil)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRefreshAllowed(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		claims  JWTClaims
		wantErr error
	}{
		{"fresh session", JWTClaims{PasswordFingerprint: "f", AuthTime: now.Add(-time.Hour).Unix()}, nil},
		{"just inside the cap", JWTClaims{PasswordFingerprint: "f", AuthTime: now.Add(-maxSessionAge + time.Minute).Unix()}, nil},
		{"session older than the cap", JWTClaims{PasswordFingerprint: "f", AuthTime: now.Add(-maxSessionAge - time.Minute).Unix()}, ErrRefreshDenied},
		{"legacy token without pfp", JWTClaims{AuthTime: now.Unix()}, ErrRefreshDenied},
		{"legacy token without auth_time", JWTClaims{PasswordFingerprint: "f"}, ErrRefreshDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := refreshAllowed(&tt.claims, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateAndValidateToken_CarriesSessionClaims(t *testing.T) {
	svc := NewJWTServiceWithSecret("test-secret", time.Hour)
	authTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)

	token, err := svc.GenerateToken("2", "tenant-a", "abcdef0123456789", authTime)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.TenantID != "2" || claims.PasswordFingerprint != "abcdef0123456789" || claims.AuthTime != authTime.Unix() {
		t.Fatalf("claims not carried: %+v", claims)
	}
}

// Every token an old replica can still sign during a rollout must expire before the grace ends.
func TestNewJWTService_LegacyGraceOutlivesOldTokens(t *testing.T) {
	before := time.Now()
	svc := NewJWTServiceWithSecret("test-secret", 12*time.Hour)
	if latestOldTokenExpiry := before.Add(12 * time.Hour); svc.legacyTokensUntil.Before(latestOldTokenExpiry.Add(legacyTokenMargin)) {
		t.Fatalf("grace ends %v, before the last old token can expire plus the rollout margin", svc.legacyTokensUntil)
	}
}

// A login gets a full lifetime; a refresh near the end of the session gets a token that dies
// with the session, and expires_at reports exactly what was signed.
func TestSignToken_RefreshNeverOutlivesSession(t *testing.T) {
	svc := NewTenantService(nil, NewJWTServiceWithSecret("test-secret", 12*time.Hour))

	login, err := svc.signToken("2", "u", "f", time.Now())
	if err != nil {
		t.Fatalf("signToken: %v", err)
	}
	if remaining := time.Until(login.ExpiresAt); remaining < 12*time.Hour-time.Minute {
		t.Fatalf("login token lives %v, want a full 12h", remaining)
	}

	authTime := time.Now().Add(-23 * time.Hour)
	refreshed, err := svc.signToken("2", "u", "f", authTime)
	if err != nil {
		t.Fatalf("signToken: %v", err)
	}
	sessionEnd := authTime.Add(maxSessionAge)
	if !refreshed.ExpiresAt.Equal(sessionEnd) {
		t.Fatalf("expires_at = %v, want the session end %v", refreshed.ExpiresAt, sessionEnd)
	}
	claims, err := svc.jwtService.ValidateToken(refreshed.Token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if signed := claims.ExpiresAt.Time; signed.After(sessionEnd) {
		t.Fatalf("signed exp %v is past the session end %v", signed, sessionEnd)
	}
}

func TestValidateToken_RejectsOtherAlgorithms(t *testing.T) {
	claims := JWTClaims{TenantID: "1", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}

	hs512, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign HS512: %v", err)
	}
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}

	svc := NewJWTServiceWithSecret("test-secret", time.Hour)
	for name, token := range map[string]string{"HS512": hs512, "none": none} {
		if _, err := svc.ValidateToken(token); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s token: err = %v, want ErrInvalidToken", name, err)
		}
	}
}

func TestValidateToken_Expired(t *testing.T) {
	svc := NewJWTServiceWithSecret("test-secret", -time.Minute)
	token, err := svc.GenerateToken("2", "u", "f", time.Now())
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, err := svc.ValidateToken(token); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("err = %v, want ErrExpiredToken", err)
	}
}

func TestIssueToken_RequiresPasswordHash(t *testing.T) {
	svc := NewTenantService(nil, NewJWTServiceWithSecret("test-secret", time.Hour))
	if _, err := svc.IssueToken(&Tenant{ID: 2, Username: "u"}); err == nil || !strings.Contains(err.Error(), "password hash") {
		t.Fatalf("err = %v, want a refusal to issue an unrevocable token", err)
	}

	resp, err := svc.IssueToken(&Tenant{ID: 2, Username: "u", Password: testHash})
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	claims, err := svc.jwtService.ValidateToken(resp.Token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.PasswordFingerprint != PasswordFingerprint(testHash) || resp.TenantID != "2" {
		t.Fatalf("issued token not bound to the password: %+v / %+v", claims, resp)
	}
}
