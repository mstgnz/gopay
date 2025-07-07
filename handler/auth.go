package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/auth"
	"github.com/mstgnz/gopay/infra/middle"
	"github.com/mstgnz/gopay/infra/response"
)

// AuthHandler handles authentication related HTTP requests
type AuthHandler struct {
	tenantService *auth.TenantService
	jwtService    *auth.JWTService
	validate      *validator.Validate
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(tenantService *auth.TenantService, jwtService *auth.JWTService, validate *validator.Validate) *AuthHandler {
	return &AuthHandler{
		tenantService: tenantService,
		jwtService:    jwtService,
		validate:      validate,
	}
}

// LoginRequest represents the login request structure
type LoginRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=6"`
}

// LoginResponse represents the login response structure
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Username  string    `json:"username"`
	TenantID  string    `json:"tenant_id"`
}

// ChangePasswordRequest represents the change password request structure
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,min=6"`
	NewPassword     string `json:"new_password" validate:"required,min=6"`
}

// CreateTenantRequest represents the create tenant request structure
type CreateTenantRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=6"`
}

// RefreshTokenRequest represents the refresh token request structure
type RefreshTokenRequest struct {
	Token string `json:"token" validate:"required"`
}

// Login handles tenant login requests
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Parse the login request
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	// Validate the request
	if err := h.validate.Struct(req); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return
	}

	// Create auth login request
	loginReq := auth.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	}

	// Authenticate tenant
	loginResp, err := h.tenantService.Login(loginReq)
	if err != nil {
		switch err {
		case auth.ErrInvalidCredentials:
			response.Error(w, http.StatusUnauthorized, "Invalid username or password", nil)
		case auth.ErrTenantNotFound:
			response.Error(w, http.StatusUnauthorized, "Invalid username or password", nil)
		default:
			response.Error(w, http.StatusInternalServerError, "Login failed", err)
		}
		return
	}

	// Return login response
	response.Success(w, http.StatusOK, "Login successful", loginResp)
}

// Logout handles tenant logout requests
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Get tenant information from context (set by JWT middleware)
	tenantID := middle.GetTenantIDFromContext(r.Context())
	username := middle.GetTenantUserFromContext(r.Context())

	if tenantID == "" || username == "" {
		response.Error(w, http.StatusUnauthorized, "Invalid session", nil)
		return
	}

	// For now, we just return success since JWT tokens are stateless
	// In a production system, you might want to maintain a blacklist of tokens
	responseData := map[string]string{
		"message": "Logged out successfully",
	}

	response.Success(w, http.StatusOK, "Logout successful", responseData)
}

// ChangePassword handles password change requests
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	// Get tenant information from context (set by JWT middleware)
	tenantIDStr := middle.GetTenantIDFromContext(r.Context())
	username := middle.GetTenantUserFromContext(r.Context())

	if tenantIDStr == "" || username == "" {
		response.Error(w, http.StatusUnauthorized, "Invalid session", nil)
		return
	}

	// Convert tenant ID to int
	tenantID, err := strconv.Atoi(tenantIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid tenant ID", nil)
		return
	}

	// Parse the change password request
	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	// Validate the request
	if err := h.validate.Struct(req); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return
	}

	// Change password
	err = h.tenantService.ChangePassword(tenantID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		switch err {
		case auth.ErrInvalidCredentials:
			response.Error(w, http.StatusUnauthorized, "Current password is incorrect", nil)
		case auth.ErrTenantNotFound:
			response.Error(w, http.StatusNotFound, "Tenant not found", nil)
		default:
			response.Error(w, http.StatusInternalServerError, "Failed to change password", err)
		}
		return
	}

	responseData := map[string]string{
		"message": "Password changed successfully",
	}

	response.Success(w, http.StatusOK, "Password changed", responseData)
}

// CreateTenant handles tenant creation requests (admin only)
func (h *AuthHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	// Parse the create tenant request
	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	// Validate the request
	if err := h.validate.Struct(req); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return
	}

	// Create auth create tenant request
	createReq := auth.CreateTenantRequest{
		Username: req.Username,
		Password: req.Password,
	}

	// Create tenant
	tenant, err := h.tenantService.CreateTenant(createReq)
	if err != nil {
		switch err {
		case auth.ErrTenantAlreadyExists:
			response.Error(w, http.StatusConflict, "Username already exists", nil)
		default:
			response.Error(w, http.StatusInternalServerError, "Failed to create tenant", err)
		}
		return
	}

	// Return tenant information (without password)
	responseData := map[string]any{
		"tenant_id":  tenant.ID,
		"username":   tenant.Username,
		"created_at": tenant.CreatedAt,
	}

	response.Success(w, http.StatusCreated, "Tenant created successfully", responseData)
}

// RefreshToken handles token refresh requests
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// Parse the refresh token request
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return
	}

	// Validate the request
	if err := h.validate.Struct(req); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return
	}

	// Refresh token
	newToken, err := h.jwtService.RefreshToken(req.Token)
	if err != nil {
		switch err {
		case auth.ErrExpiredToken:
			response.Error(w, http.StatusUnauthorized, "Token has expired", nil)
		case auth.ErrInvalidToken:
			response.Error(w, http.StatusUnauthorized, "Invalid token", nil)
		default:
			response.Error(w, http.StatusInternalServerError, "Failed to refresh token", err)
		}
		return
	}

	// Calculate new expiry time (24 hours from now)
	expiresAt := time.Now().Add(24 * time.Hour)

	// Return new token
	tokenResponse := map[string]any{
		"token":      newToken,
		"expires_at": expiresAt,
	}

	response.Success(w, http.StatusOK, "Token refreshed successfully", tokenResponse)
}

// GetProfile returns the current user's profile information
func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	// Get tenant information from context (set by JWT middleware)
	tenantID := middle.GetTenantIDFromContext(r.Context())
	username := middle.GetTenantUserFromContext(r.Context())
	claims := middle.GetTenantClaimsFromContext(r.Context())

	if tenantID == "" || username == "" || claims == nil {
		response.Error(w, http.StatusUnauthorized, "Invalid session", nil)
		return
	}

	// Return profile information
	profileData := map[string]any{
		"tenant_id":  tenantID,
		"username":   username,
		"last_login": claims.LastLogin,
		"issued_at":  claims.IssuedAt,
		"expires_at": claims.ExpiresAt,
	}

	response.Success(w, http.StatusOK, "Profile retrieved successfully", profileData)
}

// ValidateToken validates a JWT token (utility endpoint)
func (h *AuthHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	// Get Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		response.Error(w, http.StatusBadRequest, "Authorization header required", nil)
		return
	}

	// Check Bearer token format
	if !strings.HasPrefix(authHeader, "Bearer ") {
		response.Error(w, http.StatusBadRequest, "Invalid authorization format. Use: Bearer <jwt_token>", nil)
		return
	}

	// Extract JWT token
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		response.Error(w, http.StatusBadRequest, "JWT token required", nil)
		return
	}

	// Validate JWT token
	claims, err := h.jwtService.ValidateToken(token)
	if err != nil {
		switch err {
		case auth.ErrExpiredToken:
			response.Error(w, http.StatusUnauthorized, "Token has expired", nil)
		case auth.ErrInvalidToken:
			response.Error(w, http.StatusUnauthorized, "Invalid token", nil)
		case auth.ErrInvalidClaims:
			response.Error(w, http.StatusUnauthorized, "Invalid token claims", nil)
		case auth.ErrMissingTenant:
			response.Error(w, http.StatusUnauthorized, "Missing tenant information in token", nil)
		default:
			response.Error(w, http.StatusUnauthorized, "Token validation failed", nil)
		}
		return
	}

	// Convert ExpiresAt to time.Time
	var expiresAt time.Time
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}

	// Return token information
	tokenInfo := map[string]any{
		"valid":       true,
		"tenant_id":   claims.TenantID,
		"username":    claims.Username,
		"last_login":  time.Unix(claims.LastLogin, 0),
		"issued_at":   claims.IssuedAt.Time,
		"expires_at":  expiresAt,
		"time_to_exp": time.Until(expiresAt).String(),
	}

	response.Success(w, http.StatusOK, "Token is valid", tokenInfo)
}
