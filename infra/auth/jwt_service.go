package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mstgnz/gopay/infra/config"
)

var (
	ErrInvalidToken  = errors.New("invalid token")
	ErrExpiredToken  = errors.New("token has expired")
	ErrInvalidClaims = errors.New("invalid token claims")
	ErrMissingTenant = errors.New("tenant ID missing in token")
	ErrTokenRevoked  = errors.New("token has been revoked")
	ErrRefreshDenied = errors.New("token cannot be refreshed")
)

// maxSessionAge caps how far refresh can stretch one login, so a stolen token cannot be
// renewed forever.
const maxSessionAge = 24 * time.Hour

// JWTClaims represents the JWT token claims
type JWTClaims struct {
	TenantID  string `json:"tenant_id"`
	Username  string `json:"username"`
	LastLogin int64  `json:"last_login"`
	// PasswordFingerprint binds the token to the password hash it was issued under, so a
	// password change revokes every token issued before it.
	PasswordFingerprint string `json:"pfp,omitempty"`
	// AuthTime is the original login time; refresh carries it forward to enforce maxSessionAge.
	AuthTime int64 `json:"auth_time,omitempty"`
	jwt.RegisteredClaims
}

// PasswordFingerprint derives the pfp claim from a tenant's bcrypt hash.
func PasswordFingerprint(passwordHash string) string {
	sum := sha256.Sum256([]byte(passwordHash))
	return hex.EncodeToString(sum[:8])
}

// legacyTokenMargin covers a rolling deploy: an old replica may still sign pfp-less tokens for
// a few minutes after the first new replica starts its grace period.
const legacyTokenMargin = 15 * time.Minute

// verifyFingerprint reports whether the token still matches the tenant's current password.
// Tokens signed before the pfp claim existed carry none; they are accepted only until
// legacyUntil, by which time every one of them has expired, so no consumer is forced to log
// in. After that a leaked JWT_SECRET alone cannot mint a token: pfp needs the password hash.
func verifyFingerprint(claims *JWTClaims, currentHash string, now, legacyUntil time.Time) error {
	if claims.PasswordFingerprint == "" {
		if now.Before(legacyUntil) {
			return nil
		}
		return ErrTokenRevoked
	}
	if subtle.ConstantTimeCompare([]byte(claims.PasswordFingerprint), []byte(PasswordFingerprint(currentHash))) != 1 {
		return ErrTokenRevoked
	}
	return nil
}

// refreshAllowed rejects tokens that cannot be revoked (no pfp) and sessions older than
// maxSessionAge.
func refreshAllowed(claims *JWTClaims, now time.Time) error {
	if claims.PasswordFingerprint == "" || claims.AuthTime == 0 {
		return ErrRefreshDenied
	}
	if now.Sub(time.Unix(claims.AuthTime, 0)) > maxSessionAge {
		return ErrRefreshDenied
	}
	return nil
}

// JWTService handles JWT token operations
type JWTService struct {
	secretKey []byte
	expiry    time.Duration
	// legacyTokensUntil ends the acceptance of pfp-less tokens: one token lifetime plus a
	// rollout margin after this process started.
	legacyTokensUntil time.Time
}

// NewJWTService creates a new JWT service
func NewJWTService() *JWTService {
	return NewJWTServiceWithSecret(config.App().SecretKey, 12*time.Hour)
}

// NewJWTServiceWithSecret builds the service without touching global config.
func NewJWTServiceWithSecret(secret string, expiry time.Duration) *JWTService {
	return &JWTService{
		secretKey:         []byte(secret),
		expiry:            expiry,
		legacyTokensUntil: time.Now().Add(expiry + legacyTokenMargin),
	}
}

// GenerateToken generates a new JWT token for a tenant. authTime is the original login time.
func (s *JWTService) GenerateToken(tenantID, username, passwordFingerprint string, authTime time.Time) (string, error) {
	now := time.Now()
	return s.generateToken(tenantID, username, passwordFingerprint, authTime, now, now.Add(s.expiry))
}

// sessionExpiry is when a token signed now must expire: one token lifetime, but never past
// maxSessionAge after the original login, so refresh cannot stretch a session beyond it.
func (s *JWTService) sessionExpiry(authTime, now time.Time) time.Time {
	expiresAt := now.Add(s.expiry)
	if sessionEnd := authTime.Add(maxSessionAge); sessionEnd.Before(expiresAt) {
		return sessionEnd
	}
	return expiresAt
}

func (s *JWTService) generateToken(tenantID, username, passwordFingerprint string, authTime, now, expiresAt time.Time) (string, error) {
	claims := JWTClaims{
		TenantID:            tenantID,
		Username:            username,
		LastLogin:           now.Unix(),
		PasswordFingerprint: passwordFingerprint,
		AuthTime:            authTime.Unix(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tenantID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *JWTService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		return s.secretKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	// Check if tenant ID exists
	if claims.TenantID == "" {
		return nil, ErrMissingTenant
	}

	return claims, nil
}
