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
	IPAddress   string   `json:"ipAddress"`
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
	InstallmentCount int      `json:"installmentCount"`
	PaymentChannel   string   `json:"paymentChannel,omitempty"`
	PaymentGroup     string   `json:"paymentGroup,omitempty"`
	ConversationID   string   `json:"conversationId,omitempty"`
	Locale           string   `json:"locale,omitempty"`
	ClientIP         string   `json:"clientIp"`
	ClientUserAgent  string   `json:"clientUserAgent,omitempty"`
	Environment      string   `json:"environment,omitempty"`
	TenantID         int      `json:"tenantId,omitempty"`
	SessionID        string   `json:"sessionId,omitempty"`
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
	SessionID        string        `json:"sessionId,omitempty"`
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
	Installment      int       `json:"installment"`
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
	SessionID        string    `json:"sessionId"`
}

// InquireRequest contains information to request an installment count
type InstallmentInquireRequest struct {
	LogID       int64   `json:"logId,omitempty"`
	CardNumber  string  `json:"cardNumber,omitempty"`
	ExpireMonth string  `json:"expireMonth,omitempty"`
	ExpireYear  string  `json:"expireYear,omitempty"`
	CVV         string  `json:"cvv,omitempty"`
	Amount      float64 `json:"amount"`
}

type InstallmentInfo struct {
	Installment int     `json:"installment"`
	Commission  float64 `json:"commission"`
}

type InstallmentInquireResponse struct {
	Amount       float64                      `json:"amount"`
	Message      string                       `json:"message"`
	Installments map[string][]InstallmentInfo `json:"installments"`
}

type CommissionRequest struct {
	BinValue         string  `json:"binValue"`
	InstallmentCount int     `json:"installmentCount"`
	Amount           float64 `json:"amount"`
	Currency         string  `json:"currency"`
	LogID            int64   `json:"logId,omitempty"`
}

type CommissionResponse struct {
	Success          bool    `json:"success"`
	Message          string  `json:"message"`
	NetAmount        float64 `json:"netAmount"`
	GrossAmount      float64 `json:"grossAmount"`
	CommissionRate   float64 `json:"commissionRate"`
	CommissionAmount float64 `json:"commissionAmount"`
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

	// Validate tenant ID
	if state.TenantID <= 0 {
		return "", fmt.Errorf("invalid tenant ID: %d", state.TenantID)
	}

	// Serialize state data
	stateData, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}

	// Set expiration (30 minutes from now)
	expiresAt := time.Now().Add(30 * time.Minute)

	// Prepare nullable fields
	var originalCallback sql.NullString
	if state.OriginalCallback != "" {
		originalCallback = sql.NullString{String: state.OriginalCallback, Valid: true}
	}

	var currency sql.NullString
	if state.Currency != "" {
		currency = sql.NullString{String: state.Currency, Valid: true}
	}

	var conversationID sql.NullString
	if state.ConversationID != "" {
		conversationID = sql.NullString{String: state.ConversationID, Valid: true}
	}

	var logID sql.NullInt64
	if state.LogID > 0 {
		logID = sql.NullInt64{Int64: state.LogID, Valid: true}
	}

	var installment sql.NullInt32
	if state.Installment > 0 {
		installment = sql.NullInt32{Int32: int32(state.Installment), Valid: true}
	}

	var sessionID sql.NullString
	if state.SessionID != "" {
		sessionID = sql.NullString{String: state.SessionID, Valid: true}
	}

	// Insert into database and get auto-generated ID
	query := `
		INSERT INTO callbacks (
			tenant_id, provider, payment_id, original_callback, 
			amount, currency, conversation_id, log_id, environment, 
			client_ip, installment, session_id, state_data, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id
	`

	var stateID int
	err = db.QueryRowContext(ctx, query,
		state.TenantID, state.Provider, state.PaymentID, originalCallback,
		state.Amount, currency, conversationID, logID, state.Environment,
		state.ClientIP, installment, sessionID, string(stateData), expiresAt,
	).Scan(&stateID)

	if err != nil {
		return "", fmt.Errorf("failed to store callback state (tenant_id: %d): %w", state.TenantID, err)
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
	var sessionID sql.NullString

	query := `
		SELECT state_data, used, expires_at, session_id
		FROM callbacks 
		WHERE id = $1
	`

	err = db.QueryRowContext(ctx, query, id).Scan(&stateData, &used, &expiresAt, &sessionID)
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

	state.SessionID = sessionID.String

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
		callbackState, dbErr := RetrieveCallbackState(ctx, state)
		if dbErr == nil {
			return callbackState, nil
		}
		// If it's clearly an integer ID but not found in DB, return more specific error
		return nil, fmt.Errorf("callback state not found or expired (ID: %s): %w", state, dbErr)
	}

	// Fallback to old encrypted system for backward compatibility
	encryptor := NewCallbackEncryptor()
	return encryptor.DecryptCallbackState(state)
}

// UpdateCallbackState updates callback state in database
func UpdateCallbackState(ctx context.Context, stateID string, referenceCode string) error {
	db := config.App().DB
	if db == nil {
		return errors.New("database connection not available")
	}

	query := `
		UPDATE callbacks 
		SET payment_id = $1,
		    state_data = jsonb_set(state_data::jsonb, '{paymentId}', to_jsonb($2::text))
		WHERE id = $3
	`

	_, err := db.ExecContext(ctx, query, referenceCode, referenceCode, stateID)
	if err != nil {
		return fmt.Errorf("failed to update callback state: %w", err)
	}

	return nil
}

// PaymentProvider defines the interface that all payment gateways must implement
type PaymentProvider interface {
	// Initialize sets up the payment provider with authentication and configuration
	Initialize(config map[string]string) error

	// GetRequiredConfig returns the configuration fields required for this provider
	GetRequiredConfig(environment string) []ConfigField

	// GetInstallmentCount returns the installment count for a payment
	GetInstallmentCount(ctx context.Context, request InstallmentInquireRequest) (InstallmentInquireResponse, error)

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

	GetCommission(ctx context.Context, request CommissionRequest) (CommissionResponse, error)

	// ValidateWebhook validates an incoming webhook notification
	ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error)
}

// ProviderFactory is a function type that creates a new PaymentProvider
type ProviderFactory func() PaymentProvider

// ErrCardStorageUnsupported is returned when a card-storage operation is requested
// from a provider that does not implement the optional CardStorageProvider capability.
var ErrCardStorageUnsupported = errors.New("provider does not support card storage")

// CardStorageProvider is an OPTIONAL capability interface implemented only by providers that
// support saving a customer's card once (OTP-protected) and charging it later with just a
// provider card id. Providers that do not implement it keep working unchanged; the card service
// type-asserts on this interface and returns ErrCardStorageUnsupported otherwise. The core
// PaymentProvider interface is intentionally NOT extended so existing providers are untouched.
type CardStorageProvider interface {
	// SendCardOTP sends an OTP SMS to the customer's MSISDN. It returns the referenceNumber
	// that MUST be echoed back to ValidateCardOTP (and to ListProviderCards for the PM_LIST flow).
	SendCardOTP(ctx context.Context, request CardOTPSendRequest) (*CardOTPSendResponse, error)

	// ValidateCardOTP validates the OTP the customer received for the given referenceNumber.
	ValidateCardOTP(ctx context.Context, request CardOTPValidateRequest) (*CardOTPValidateResponse, error)

	// RegisterCard tokenizes and saves a card in the provider wallet and returns the provider
	// card id plus masked metadata. The caller persists only the masked metadata + card id;
	// raw card data must never be stored.
	RegisterCard(ctx context.Context, request RegisterCardRequest) (*RegisterCardResponse, error)

	// ListProviderCards lists the cards saved in the provider wallet for an MSISDN (requires a
	// validated OTP referenceNumber for the PM_LIST flow).
	ListProviderCards(ctx context.Context, request ListCardsRequest) (*ListCardsResponse, error)

	// DeleteProviderCard removes a saved card from the provider wallet.
	DeleteProviderCard(ctx context.Context, request DeleteCardRequest) (*DeleteCardResponse, error)

	// PayWithSavedCard charges a saved card by provider card id, without 3D secure.
	PayWithSavedCard(ctx context.Context, request SavedCardPaymentRequest) (*PaymentResponse, error)

	// Create3DPaymentWithSavedCard starts a 3D secure payment using a saved provider card id.
	// Completion reuses the standard Complete3DPayment callback path.
	Create3DPaymentWithSavedCard(ctx context.Context, request SavedCardPaymentRequest) (*PaymentResponse, error)
}

// CardOTPSendRequest asks the provider to send an OTP SMS to MSISDN.
type CardOTPSendRequest struct {
	LogID    int64  `json:"logId,omitempty"`
	MSISDN   string `json:"msisdn"`
	ClientIP string `json:"clientIp,omitempty"`
}

// CardOTPSendResponse carries the referenceNumber that ties the OTP send/validate/list calls together.
type CardOTPSendResponse struct {
	Success          bool   `json:"success"`
	ReferenceNumber  string `json:"referenceNumber"`
	Token            string `json:"token,omitempty"`
	ExpiresInSeconds int    `json:"expiresInSeconds,omitempty"`
	Message          string `json:"message,omitempty"`
	ErrorCode        string `json:"errorCode,omitempty"`
	ProviderResponse any    `json:"providerResponse,omitempty"`
}

// CardOTPValidateRequest validates an OTP for a referenceNumber issued by SendCardOTP.
type CardOTPValidateRequest struct {
	LogID           int64  `json:"logId,omitempty"`
	MSISDN          string `json:"msisdn"`
	ReferenceNumber string `json:"referenceNumber"`
	OTP             string `json:"otp"`
	Token           string `json:"token,omitempty"`
	ClientIP        string `json:"clientIp,omitempty"`
}

// CardOTPValidateResponse is the result of an OTP validation.
type CardOTPValidateResponse struct {
	Success          bool   `json:"success"`
	Message          string `json:"message,omitempty"`
	ErrorCode        string `json:"errorCode,omitempty"`
	ProviderResponse any    `json:"providerResponse,omitempty"`
}

// RegisterCardRequest registers (tokenizes + saves) a card in the provider wallet for an MSISDN.
type RegisterCardRequest struct {
	LogID           int64    `json:"logId,omitempty"`
	MSISDN          string   `json:"msisdn"`
	Card            CardInfo `json:"card"`
	Alias           string   `json:"alias,omitempty"`
	ReferenceNumber string   `json:"referenceNumber,omitempty"`
	ClientIP        string   `json:"clientIp,omitempty"`
}

// RegisterCardResponse returns the provider card id and masked metadata. No raw PAN/CVV.
type RegisterCardResponse struct {
	Success          bool   `json:"success"`
	ProviderCardID   string `json:"providerCardId"`
	MaskedCardNo     string `json:"maskedCardNo,omitempty"`
	CardBrand        string `json:"cardBrand,omitempty"`
	CardType         string `json:"cardType,omitempty"`
	Message          string `json:"message,omitempty"`
	ErrorCode        string `json:"errorCode,omitempty"`
	ProviderResponse any    `json:"providerResponse,omitempty"`
}

// ProviderCard is a single card entry returned by the provider wallet.
type ProviderCard struct {
	ProviderCardID string `json:"providerCardId"`
	MaskedCardNo   string `json:"maskedCardNo,omitempty"`
	CardBrand      string `json:"cardBrand,omitempty"`
	CardType       string `json:"cardType,omitempty"`
	Alias          string `json:"alias,omitempty"`
}

// ListCardsRequest lists the provider wallet cards for an MSISDN.
type ListCardsRequest struct {
	LogID           int64  `json:"logId,omitempty"`
	MSISDN          string `json:"msisdn"`
	ReferenceNumber string `json:"referenceNumber,omitempty"`
	ClientIP        string `json:"clientIp,omitempty"`
}

// ListCardsResponse is the provider wallet card list.
type ListCardsResponse struct {
	Success          bool           `json:"success"`
	Cards            []ProviderCard `json:"cards"`
	Message          string         `json:"message,omitempty"`
	ErrorCode        string         `json:"errorCode,omitempty"`
	ProviderResponse any            `json:"providerResponse,omitempty"`
}

// DeleteCardRequest removes a saved card from the provider wallet.
type DeleteCardRequest struct {
	LogID          int64  `json:"logId,omitempty"`
	MSISDN         string `json:"msisdn"`
	ProviderCardID string `json:"providerCardId"`
	ClientIP       string `json:"clientIp,omitempty"`
}

// DeleteCardResponse is the result of a wallet card deletion.
type DeleteCardResponse struct {
	Success          bool   `json:"success"`
	Message          string `json:"message,omitempty"`
	ErrorCode        string `json:"errorCode,omitempty"`
	ProviderResponse any    `json:"providerResponse,omitempty"`
}

// SavedCardPaymentRequest charges a previously saved card identified by ProviderCardID.
// No card data or monetary fields beyond Amount/Currency cross the wire from the client; the
// service resolves ProviderCardID from GoPay's saved_cards table scoped by tenant + MSISDN.
type SavedCardPaymentRequest struct {
	LogID            int64   `json:"logId,omitempty"`
	TenantID         int     `json:"tenantId,omitempty"`
	Environment      string  `json:"environment,omitempty"`
	MSISDN           string  `json:"msisdn"`
	ProviderCardID   string  `json:"providerCardId"`
	Amount           float64 `json:"amount"`
	Currency         string  `json:"currency"`
	InstallmentCount int     `json:"installmentCount"`
	CallbackURL      string  `json:"callbackUrl,omitempty"`
	ClientIP         string  `json:"clientIp,omitempty"`
	ClientUserAgent  string  `json:"clientUserAgent,omitempty"`
	ConversationID   string  `json:"conversationId,omitempty"`
	SessionID        string  `json:"sessionId,omitempty"`
}
