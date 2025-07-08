package auth

import (
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
)

// JWTClaims represents the JWT token claims
type JWTClaims struct {
	TenantID  string `json:"tenant_id"`
	Username  string `json:"username"`
	LastLogin int64  `json:"last_login"`
	jwt.RegisteredClaims
}

// JWTService handles JWT token operations
type JWTService struct {
	secretKey []byte
	expiry    time.Duration
}

// NewJWTService creates a new JWT service
func NewJWTService() *JWTService {
	jwtSecret := config.App().SecretKey
	jwtExpiry := 12 * time.Hour // 12 hours
	return &JWTService{
		secretKey: []byte(jwtSecret),
		expiry:    jwtExpiry,
	}
}

// GenerateToken generates a new JWT token for a tenant
func (s *JWTService) GenerateToken(tenantID, username string) (string, error) {
	now := time.Now()

	claims := JWTClaims{
		TenantID:  tenantID,
		Username:  username,
		LastLogin: now.Unix(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tenantID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expiry)),
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
		// Check signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

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

// RefreshToken generates a new token from an existing valid token
func (s *JWTService) RefreshToken(tokenString string) (string, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	// Generate new token with updated expiry
	return s.GenerateToken(claims.TenantID, claims.Username)
}

// ExtractTenantID extracts tenant ID from token without full validation
func (s *JWTService) ExtractTenantID(tokenString string) (string, error) {
	// Parse token without verification for tenant ID extraction
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &JWTClaims{})
	if err != nil {
		return "", ErrInvalidToken
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return "", ErrInvalidClaims
	}

	if claims.TenantID == "" {
		return "", ErrMissingTenant
	}

	return claims.TenantID, nil
}
