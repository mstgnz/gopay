package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/mstgnz/gopay/infra/conn"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrTenantNotFound      = errors.New("tenant not found")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrTenantAlreadyExists = errors.New("tenant already exists")
	ErrDatabaseError       = errors.New("database error")
)

// Tenant represents a tenant in the system
type Tenant struct {
	ID        int        `json:"id"`
	Username  string     `json:"username"`
	Password  string     `json:"-"` // Never expose password in JSON
	LastLogin *time.Time `json:"last_login,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	Code      *string    `json:"code,omitempty"` // For password reset or SMS verification
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=6"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token     string    `json:"token"`
	TenantID  string    `json:"tenant_id"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateTenantRequest represents a tenant creation request
type CreateTenantRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=6"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=6"`
}

// TenantService handles tenant operations
type TenantService struct {
	db         *conn.DB
	jwtService *JWTService
}

// NewTenantService creates a new tenant service
func NewTenantService(db *conn.DB, jwtService *JWTService) *TenantService {
	return &TenantService{
		db:         db,
		jwtService: jwtService,
	}
}

// Login authenticates a tenant and returns a JWT token
func (s *TenantService) Login(req LoginRequest) (*LoginResponse, error) {
	// Get tenant by username
	tenant, err := s.GetTenantByUsername(req.Username)
	if err != nil {
		if err == ErrTenantNotFound {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(tenant.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Update last login
	if err := s.UpdateLastLogin(tenant.ID); err != nil {
		// Log error but don't fail login
		fmt.Printf("Warning: Failed to update last login for tenant %d: %v\n", tenant.ID, err)
	}

	// Generate JWT token
	tenantID := fmt.Sprintf("%d", tenant.ID)
	token, err := s.jwtService.GenerateToken(tenantID, tenant.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Calculate expiry time
	expiresAt := time.Now().Add(24 * time.Hour) // Default 24 hours

	return &LoginResponse{
		Token:     token,
		TenantID:  tenantID,
		Username:  tenant.Username,
		ExpiresAt: expiresAt,
	}, nil
}

// CreateTenant creates a new tenant
func (s *TenantService) CreateTenant(req CreateTenantRequest) (*Tenant, error) {
	// Check if tenant already exists
	existing, err := s.GetTenantByUsername(req.Username)
	if err != ErrTenantNotFound {
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, ErrTenantAlreadyExists
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert tenant
	query := `
		INSERT INTO tenants (username, password, created_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		RETURNING id, username, created_at
	`

	var tenant Tenant
	err = s.db.QueryRow(query, req.Username, string(hashedPassword)).Scan(
		&tenant.ID,
		&tenant.Username,
		&tenant.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	return &tenant, nil
}

// GetTenantByUsername retrieves a tenant by username
func (s *TenantService) GetTenantByUsername(username string) (*Tenant, error) {
	query := `
		SELECT id, username, password, last_login, created_at, code
		FROM tenants
		WHERE username = $1
	`

	var tenant Tenant
	err := s.db.QueryRow(query, username).Scan(
		&tenant.ID,
		&tenant.Username,
		&tenant.Password,
		&tenant.LastLogin,
		&tenant.CreatedAt,
		&tenant.Code,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrTenantNotFound
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	return &tenant, nil
}

// GetTenantByID retrieves a tenant by ID
func (s *TenantService) GetTenantByID(id int) (*Tenant, error) {
	query := `
		SELECT id, username, password, last_login, created_at, code
		FROM tenants
		WHERE id = $1
	`

	var tenant Tenant
	err := s.db.QueryRow(query, id).Scan(
		&tenant.ID,
		&tenant.Username,
		&tenant.Password,
		&tenant.LastLogin,
		&tenant.CreatedAt,
		&tenant.Code,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrTenantNotFound
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	return &tenant, nil
}

// UpdateLastLogin updates the last login time for a tenant
func (s *TenantService) UpdateLastLogin(tenantID int) error {
	query := `
		UPDATE tenants
		SET last_login = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := s.db.Exec(query, tenantID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

// ChangePassword changes the password for a tenant
func (s *TenantService) ChangePassword(tenantID int, oldPassword, newPassword string) error {
	// Get current tenant
	tenant, err := s.GetTenantByID(tenantID)
	if err != nil {
		return err
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(tenant.Password), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	query := `
		UPDATE tenants
		SET password = $1
		WHERE id = $2
	`

	_, err = s.db.Exec(query, string(hashedPassword), tenantID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// AdminChangePassword changes the password for a tenant without requiring the old password
// This method should only be used by administrators
func (s *TenantService) AdminChangePassword(tenantID int, newPassword string) error {
	// Check if target tenant exists
	_, err := s.GetTenantByID(tenantID)
	if err != nil {
		return err
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	query := `
		UPDATE tenants
		SET password = $1
		WHERE id = $2
	`

	_, err = s.db.Exec(query, string(hashedPassword), tenantID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// ValidateToken validates a JWT token and returns tenant information
func (s *TenantService) ValidateToken(tokenString string) (*Tenant, error) {
	// Validate JWT token
	claims, err := s.jwtService.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Get tenant by ID from token
	tenantID := 0
	if _, err := fmt.Sscanf(claims.TenantID, "%d", &tenantID); err != nil {
		return nil, ErrInvalidClaims
	}

	tenant, err := s.GetTenantByID(tenantID)
	if err != nil {
		return nil, err
	}

	return tenant, nil
}

// SetVerificationCode sets a verification code for a tenant (for password reset, etc.)
func (s *TenantService) SetVerificationCode(tenantID int, code string) error {
	query := `
		UPDATE tenants
		SET code = $1
		WHERE id = $2
	`

	_, err := s.db.Exec(query, code, tenantID)
	if err != nil {
		return fmt.Errorf("failed to set verification code: %w", err)
	}

	return nil
}

// ClearVerificationCode clears the verification code for a tenant
func (s *TenantService) ClearVerificationCode(tenantID int) error {
	query := `
		UPDATE tenants
		SET code = NULL
		WHERE id = $1
	`

	_, err := s.db.Exec(query, tenantID)
	if err != nil {
		return fmt.Errorf("failed to clear verification code: %w", err)
	}

	return nil
}

// CountTenants returns the total number of tenants in the system
func (s *TenantService) CountTenants() (int, error) {
	query := `SELECT COUNT(*) FROM tenants`

	var count int
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count tenants: %w", err)
	}

	return count, nil
}

// Register handles user registration with special rules:
// - Only allows registration if no tenants exist (first user becomes admin)
// - Blocks registration if tenants already exist (only admin can create new users)
func (s *TenantService) Register(req RegisterRequest) (*Tenant, error) {
	// Check how many tenants exist
	count, err := s.CountTenants()
	if err != nil {
		return nil, err
	}

	// If tenants already exist, registration is not allowed
	// Only admin can create new tenants via CreateTenant
	if count > 0 {
		return nil, errors.New("registration is closed - only administrators can create new accounts")
	}

	// First user registration is allowed and becomes admin
	return s.createFirstTenant(req)
}

// createFirstTenant creates the first tenant (admin) in the system
func (s *TenantService) createFirstTenant(req RegisterRequest) (*Tenant, error) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert first tenant (admin)
	query := `
		INSERT INTO tenants (username, password, created_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		RETURNING id, username, created_at
	`

	var tenant Tenant
	err = s.db.QueryRow(query, req.Username, string(hashedPassword)).Scan(
		&tenant.ID,
		&tenant.Username,
		&tenant.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create first tenant: %w", err)
	}

	return &tenant, nil
}
