package provider

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/mstgnz/gopay/infra/config"
)

// PaymentStatus represents the current status of a payment
type PaymentStatus string

const (
	StatusPending    PaymentStatus = "pending"
	StatusProcessing PaymentStatus = "processing"
	StatusSuccessful PaymentStatus = "successful"
	StatusFailed     PaymentStatus = "failed"
	StatusCancelled  PaymentStatus = "cancelled"
	StatusRefunded   PaymentStatus = "refunded"
)

// Address represents a physical address
type Address struct {
	City        string `json:"city"`
	Country     string `json:"country"`
	Address     string `json:"address"`
	ZipCode     string `json:"zipCode"`
	Description string `json:"description,omitempty"`
}

// ConfigField represents a required configuration field for a payment provider
type ConfigField struct {
	Key         string `json:"key"`
	Required    bool   `json:"required"`
	Type        string `json:"type"` // "string", "number", "url", "email", "boolean"
	Description string `json:"description"`
	Example     string `json:"example"`
	Pattern     string `json:"pattern,omitempty"`   // regex pattern for validation
	MinLength   int    `json:"minLength,omitempty"` // minimum length for string fields
	MaxLength   int    `json:"maxLength,omitempty"` // maximum length for string fields
}

// Customer represents the buyer information
type Customer struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Surname     string   `json:"surname"`
	Email       string   `json:"email"`
	PhoneNumber string   `json:"phoneNumber,omitempty"`
	IPAddress   string   `json:"ipAddress,omitempty"`
	Address     *Address `json:"address,omitempty"`
}

// CardInfo represents credit card information
type CardInfo struct {
	CardHolderName string `json:"cardHolderName"`
	CardNumber     string `json:"cardNumber"`
	ExpireMonth    string `json:"expireMonth"`
	ExpireYear     string `json:"expireYear"`
	CVV            string `json:"cvv"`
}

// Item represents a product or service item in the payment
type Item struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Category    string  `json:"category,omitempty"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
}

// PaymentRequest contains all information required to create a payment
type PaymentRequest struct {
	ID               string   `json:"id,omitempty"`
	LogID            int64    `json:"logId,omitempty"`
	ReferenceID      string   `json:"referenceId,omitempty"`
	Currency         string   `json:"currency"`
	Amount           float64  `json:"amount"`
	Customer         Customer `json:"customer"`
	CardInfo         CardInfo `json:"cardInfo"`
	Items            []Item   `json:"items,omitempty"`
	Description      string   `json:"description,omitempty"`
	CallbackURL      string   `json:"callbackUrl"`
	Use3D            bool     `json:"use3D"`
	InstallmentCount int      `json:"installmentCount,omitempty"`
	PaymentChannel   string   `json:"paymentChannel,omitempty"`
	PaymentGroup     string   `json:"paymentGroup,omitempty"`
	ConversationID   string   `json:"conversationId,omitempty"`
	Locale           string   `json:"locale,omitempty"`
	ClientIP         string   `json:"clientIp,omitempty"`
	ClientUserAgent  string   `json:"clientUserAgent,omitempty"`
	MetaData         string   `json:"metaData,omitempty"`
	Environment      string   `json:"environment,omitempty"`
	TenantID         int      `json:"tenantId,omitempty"`
}

// PaymentResponse contains the result of a payment request
type PaymentResponse struct {
	Success          bool          `json:"success"`
	Status           PaymentStatus `json:"status"`
	Message          string        `json:"message,omitempty"`
	ErrorCode        string        `json:"errorCode,omitempty"`
	TransactionID    string        `json:"transactionId,omitempty"`
	PaymentID        string        `json:"paymentId,omitempty"`
	OrderID          string        `json:"orderId,omitempty"`
	Amount           float64       `json:"amount,omitempty"`
	Currency         string        `json:"currency"`
	RedirectURL      string        `json:"redirectUrl,omitempty"`
	HTML             string        `json:"html,omitempty"`
	SystemTime       *time.Time    `json:"systemTime,omitempty"`
	FraudStatus      int           `json:"fraudStatus,omitempty"`
	ProviderResponse any           `json:"providerResponse,omitempty"`
}

// RefundRequest contains information to request a refund
type RefundRequest struct {
	PaymentID      string  `json:"paymentId"`
	RefundAmount   float64 `json:"refundAmount,omitempty"`
	Reason         string  `json:"reason,omitempty"`
	Description    string  `json:"description,omitempty"`
	Currency       string  `json:"currency,omitempty"`
	ConversationID string  `json:"conversationId,omitempty"`
	LogID          int64   `json:"logId,omitempty"`
}

// CancelRequest contains information to request a cancel
type CancelRequest struct {
	PaymentID      string `json:"paymentId"`
	Reason         string `json:"reason,omitempty"`
	Description    string `json:"description,omitempty"`
	Currency       string `json:"currency,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	LogID          int64  `json:"logId,omitempty"`
}

// GetPaymentStatusRequest contains information to request a payment status
type GetPaymentStatusRequest struct {
	PaymentID      string `json:"paymentId"`
	ConversationID string `json:"conversationId,omitempty"`
	LogID          int64  `json:"logId,omitempty"`
}

// RefundResponse contains the result of a refund request
type RefundResponse struct {
	Success      bool       `json:"success"`
	RefundID     string     `json:"refundId,omitempty"`
	PaymentID    string     `json:"paymentId,omitempty"`
	Status       string     `json:"status,omitempty"`
	RefundAmount float64    `json:"refundAmount,omitempty"`
	Message      string     `json:"message,omitempty"`
	ErrorCode    string     `json:"errorCode,omitempty"`
	SystemTime   *time.Time `json:"systemTime,omitempty"`
	RawResponse  any        `json:"rawResponse,omitempty"`
}

// CallbackState represents encrypted state data for secure callbacks across all providers
type CallbackState struct {
	TenantID         int       `json:"tenantId"`
	PaymentID        string    `json:"paymentId"`
	OriginalCallback string    `json:"originalCallback"`
	Amount           float64   `json:"amount"`
	Currency         string    `json:"currency"`
	ConversationID   string    `json:"conversationId"`
	LogID            int64     `json:"logId"`
	Provider         string    `json:"provider"`
	Environment      string    `json:"environment"`
	Timestamp        time.Time `json:"timestamp"`
	ClientIP         string    `json:"clientIp"`
}

var callbackEncryptor *CallbackEncryptor

// CallbackEncryptor provides secure encryption/decryption for callback state
type CallbackEncryptor struct {
	secretKey string
}

// NewCallbackEncryptor creates a new callback encryptor with the given secret key
func NewCallbackEncryptor() *CallbackEncryptor {
	if callbackEncryptor == nil {
		callbackEncryptor = &CallbackEncryptor{secretKey: config.App().EncryptKey}
	}
	return callbackEncryptor
}

// EncryptCallbackState encrypts callback state data using AES-GCM
func (e *CallbackEncryptor) EncryptCallbackState(state CallbackState) (string, error) {
	// Derive encryption key from secret
	key := e.deriveEncryptionKey()

	// Marshal state to JSON
	plaintext, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Combine nonce and ciphertext
	combined := append(nonce, ciphertext...)
	return base64.URLEncoding.EncodeToString(combined), nil
}

// DecryptCallbackState decrypts callback state data using AES-GCM
func (e *CallbackEncryptor) DecryptCallbackState(encryptedState string) (*CallbackState, error) {
	// Derive encryption key from secret
	key := e.deriveEncryptionKey()

	// Decode base64
	combined, err := base64.URLEncoding.DecodeString(encryptedState)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Check minimum length
	if len(combined) < gcm.NonceSize() {
		return nil, errors.New("encrypted state too short")
	}

	// Extract nonce and ciphertext
	nonce := combined[:gcm.NonceSize()]
	ciphertext := combined[gcm.NonceSize():]

	// Decrypt and verify
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	// Unmarshal state
	var state CallbackState
	if err := json.Unmarshal(plaintext, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Validate timestamp (prevent replay attacks)
	if time.Since(state.Timestamp) > 30*time.Minute {
		return nil, errors.New("callback state expired")
	}

	return &state, nil
}

// deriveEncryptionKey derives a 32-byte encryption key from the secret
func (e *CallbackEncryptor) deriveEncryptionKey() []byte {
	hash := sha256.Sum256([]byte(e.secretKey + "-callback-encryption-v1"))
	return hash[:]
}

// StoreCallbackState stores callback state in database and returns short ID
func StoreCallbackState(ctx context.Context, state CallbackState) (string, error) {
	db := config.App().DB
	if db == nil {
		return "", errors.New("database connection not available")
	}

	// Serialize state data
	stateData, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}

	// Set expiration (30 minutes from now)
	expiresAt := time.Now().Add(30 * time.Minute)

	// Insert into database and get auto-generated ID
	query := `
		INSERT INTO callbacks (
			tenant_id, provider, payment_id, original_callback, 
			amount, currency, conversation_id, log_id, environment, 
			client_ip, state_data, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`

	var stateID int
	err = db.QueryRowContext(ctx, query,
		state.TenantID, state.Provider, state.PaymentID, state.OriginalCallback,
		state.Amount, state.Currency, state.ConversationID, state.LogID, state.Environment,
		state.ClientIP, string(stateData), expiresAt,
	).Scan(&stateID)

	if err != nil {
		return "", fmt.Errorf("failed to store callback state: %w", err)
	}

	return fmt.Sprintf("%d", stateID), nil
}

// RetrieveCallbackState retrieves callback state from database using ID
func RetrieveCallbackState(ctx context.Context, stateID string) (*CallbackState, error) {
	db := config.App().DB
	if db == nil {
		return nil, errors.New("database connection not available")
	}

	// Convert string ID to integer
	id, err := strconv.Atoi(stateID)
	if err != nil {
		return nil, fmt.Errorf("invalid callback state ID format: %w", err)
	}

	var stateData string
	var used bool
	var expiresAt time.Time

	query := `
		SELECT state_data, used, expires_at 
		FROM callbacks 
		WHERE id = $1
	`

	err = db.QueryRowContext(ctx, query, id).Scan(&stateData, &used, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("callback state not found")
		}
		return nil, fmt.Errorf("failed to retrieve callback state: %w", err)
	}

	// Check if expired
	if time.Now().After(expiresAt) {
		return nil, errors.New("callback state expired")
	}

	// Check if already used (optional security measure)
	if used {
		return nil, errors.New("callback state already used")
	}

	// Mark as used (optional - prevents replay attacks)
	_, err = db.ExecContext(ctx, "UPDATE callbacks SET used = true WHERE id = $1", id)
	if err != nil {
		// Log error but don't fail the callback
		fmt.Printf("Warning: failed to mark callback state as used: %v\n", err)
	}

	// Deserialize state data
	var state CallbackState
	if err := json.Unmarshal([]byte(stateData), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// CleanupExpiredCallbackStates removes expired callback states from database
func CleanupExpiredCallbackStates(ctx context.Context) error {
	db := config.App().DB
	if db == nil {
		return errors.New("database connection not available")
	}

	query := "DELETE FROM callbacks WHERE expires_at < NOW()"
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired callback states: %w", err)
	}

	return nil
}

// CreateSecureCallbackURL creates a secure callback URL with encrypted state (DEPRECATED - use CreateShortCallbackURL)
func CreateSecureCallbackURL(gopayBaseURL, provider string, state CallbackState) (string, error) {
	// For backward compatibility, use short callback URL
	return CreateShortCallbackURL(context.Background(), gopayBaseURL, provider, state)
}

// CreateShortCallbackURL creates a callback URL with short database-stored state ID
func CreateShortCallbackURL(ctx context.Context, gopayBaseURL, provider string, state CallbackState) (string, error) {
	stateID, err := StoreCallbackState(ctx, state)
	if err != nil {
		return "", fmt.Errorf("failed to store callback state: %w", err)
	}

	return fmt.Sprintf("%s/v1/callback/%s?state=%s", gopayBaseURL, provider, stateID), nil
}

// HandleEncryptedCallbackState is a helper function for providers to handle encrypted callback state (DEPRECATED)
func HandleEncryptedCallbackState(state string) (*CallbackState, error) {
	// Try new short ID system first
	if callbackState, err := RetrieveCallbackState(context.Background(), state); err == nil {
		return callbackState, nil
	}

	// Fallback to old encrypted system for backward compatibility
	encryptor := NewCallbackEncryptor()
	return encryptor.DecryptCallbackState(state)
}

// HandleCallbackState handles both new integer ID and old encrypted callback states
func HandleCallbackState(ctx context.Context, state string) (*CallbackState, error) {
	// Try new integer ID system first (primary method)
	if _, err := strconv.Atoi(state); err == nil {
		// It's a valid integer, try database lookup
		if callbackState, err := RetrieveCallbackState(ctx, state); err == nil {
			return callbackState, nil
		}
	}

	// Fallback to old encrypted system for backward compatibility
	encryptor := NewCallbackEncryptor()
	return encryptor.DecryptCallbackState(state)
}

// PaymentProvider defines the interface that all payment gateways must implement
type PaymentProvider interface {
	// Initialize sets up the payment provider with authentication and configuration
	Initialize(config map[string]string) error

	// GetRequiredConfig returns the configuration fields required for this provider
	GetRequiredConfig(environment string) []ConfigField

	// ValidateConfig validates the provided configuration against provider requirements
	ValidateConfig(config map[string]string) error

	// CreatePayment makes a non-3D payment request
	CreatePayment(ctx context.Context, request PaymentRequest) (*PaymentResponse, error)

	// Create3DPayment starts a 3D secure payment process
	Create3DPayment(ctx context.Context, request PaymentRequest) (*PaymentResponse, error)

	// Complete3DPayment completes a 3D secure payment after user authentication
	Complete3DPayment(ctx context.Context, callbackState *CallbackState, data map[string]string) (*PaymentResponse, error)

	// GetPaymentStatus retrieves the current status of a payment
	GetPaymentStatus(ctx context.Context, request GetPaymentStatusRequest) (*PaymentResponse, error)

	// CancelPayment cancels a payment
	CancelPayment(ctx context.Context, request CancelRequest) (*PaymentResponse, error)

	// RefundPayment issues a refund for a payment
	RefundPayment(ctx context.Context, request RefundRequest) (*RefundResponse, error)

	// ValidateWebhook validates an incoming webhook notification
	ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error)
}

// ProviderFactory is a function type that creates a new PaymentProvider
type ProviderFactory func() PaymentProvider
