package provider

import (
	"context"
	"time"
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
	CallbackURL      string   `json:"callbackUrl,omitempty"`
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
	Amount           float64       `json:"amount"`
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
	Complete3DPayment(ctx context.Context, paymentID string, conversationID string, data map[string]string) (*PaymentResponse, error)

	// GetPaymentStatus retrieves the current status of a payment
	GetPaymentStatus(ctx context.Context, paymentID string) (*PaymentResponse, error)

	// CancelPayment cancels a payment
	CancelPayment(ctx context.Context, paymentID string, reason string) (*PaymentResponse, error)

	// RefundPayment issues a refund for a payment
	RefundPayment(ctx context.Context, request RefundRequest) (*RefundResponse, error)

	// ValidateWebhook validates an incoming webhook notification
	ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error)
}

// ProviderFactory is a function type that creates a new PaymentProvider
type ProviderFactory func() PaymentProvider
