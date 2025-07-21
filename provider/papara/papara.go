package papara

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// API URLs
	apiSandboxURL    = "https://merchant.test.papara.com"
	apiProductionURL = "https://merchant.papara.com"

	// API Endpoints
	endpointPayment       = "/api/v1/payments"
	endpointPaymentStatus = "/api/v1/payments/%s" // %s will be replaced with paymentId
	endpointRefund        = "/api/v1/refunds"
	endpointAccount       = "/api/v1/account"

	// Papara Status Codes
	statusPending   = "PENDING"
	statusCompleted = "COMPLETED"
	statusRefunded  = "REFUNDED"
	statusFailed    = "FAILED"
	statusCancelled = "CANCELLED"

	// Default Values
	defaultCurrency = "TRY"
	defaultTimeout  = 30 * time.Second
)

// PaparaProvider implements the provider.PaymentProvider interface for Papara
type PaparaProvider struct {
	apiKey       string
	baseURL      string
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
	client       *http.Client
	logID        int64
}

// NewProvider creates a new Papara payment provider
func NewProvider() provider.PaymentProvider {
	return &PaparaProvider{}
}

// GetRequiredConfig returns the configuration fields required for Papara
func (p *PaparaProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "apiKey",
			Required:    true,
			Type:        "string",
			Description: "Papara API Key (provided by Papara)",
			Example:     "12345678-1234-1234-1234-123456789012",
			MinLength:   32,
			MaxLength:   50,
		},
		{
			Key:         "environment",
			Required:    true,
			Type:        "string",
			Description: "Environment setting (sandbox or production)",
			Example:     "sandbox",
			Pattern:     "^(sandbox|production)$",
		},
	}
}

// ValidateConfig validates the provided configuration against Papara requirements
func (p *PaparaProvider) ValidateConfig(config map[string]string) error {
	requiredFields := p.GetRequiredConfig(config["environment"])
	return provider.ValidateConfigFields("papara", config, requiredFields)
}

// Initialize sets up the Papara payment provider with authentication credentials
func (p *PaparaProvider) Initialize(conf map[string]string) error {
	p.apiKey = conf["apiKey"]

	if p.apiKey == "" {
		return errors.New("papara: apiKey is required")
	}

	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	p.isProduction = conf["environment"] == "production"
	if p.isProduction {
		p.baseURL = apiProductionURL
		// Production environment - use secure TLS
		p.client = &http.Client{
			Timeout: defaultTimeout,
		}
	} else {
		p.baseURL = apiSandboxURL
		// Sandbox environment - skip TLS verification for test endpoints
		p.client = &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}

	return nil
}

// CreatePayment makes a non-3D payment request
func (p *PaparaProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("papara: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *PaparaProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("papara: invalid 3D payment request: %w", err)
	}

	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *PaparaProvider) Complete3DPayment(ctx context.Context, paymentID, conversationID string, data map[string]string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("papara: paymentID is required")
	}

	// For Papara, typically we just need to check the payment status after 3D completion
	return p.GetPaymentStatus(ctx, paymentID)
}

// GetPaymentStatus retrieves the current status of a payment
func (p *PaparaProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("papara: paymentID is required")
	}

	// Papara API: /payments?id=paymentID (query parametreli)
	endpoint := p.baseURL + "/payments?id=" + paymentID

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("papara: failed to create request: %w", err)
	}

	p.addAuthHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("papara: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("papara: failed to read response: %w", err)
	}

	var paparaResp PaparaResponse
	if err := json.Unmarshal(body, &paparaResp); err != nil {
		return nil, fmt.Errorf("papara: failed to parse response: %w", err)
	}

	return p.mapToPaymentResponse(paparaResp), nil
}

// CancelPayment cancels a payment
func (p *PaparaProvider) CancelPayment(ctx context.Context, paymentID, reason string) (*provider.PaymentResponse, error) {
	// Papara doesn't have a direct cancel endpoint, but we can treat this as a refund
	// First get the payment details to determine the amount
	paymentResp, err := p.GetPaymentStatus(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	// Create a full refund request
	refundReq := provider.RefundRequest{
		PaymentID:    paymentID,
		RefundAmount: paymentResp.Amount,
		Reason:       reason,
		Currency:     paymentResp.Currency,
	}

	refundResp, err := p.RefundPayment(ctx, refundReq)
	if err != nil {
		return nil, err
	}

	// Convert refund response to payment response
	return &provider.PaymentResponse{
		Success:          refundResp.Success,
		Status:           provider.StatusCancelled,
		Message:          refundResp.Message,
		ErrorCode:        refundResp.ErrorCode,
		PaymentID:        paymentID,
		Amount:           refundResp.RefundAmount,
		Currency:         paymentResp.Currency,
		SystemTime:       refundResp.SystemTime,
		ProviderResponse: refundResp.RawResponse,
	}, nil
}

// RefundPayment issues a refund for a payment
func (p *PaparaProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("papara: paymentID is required")
	}

	// Papara API: Refund için PUT /payments?id=paymentID
	endpoint := p.baseURL + "/payments?id=" + request.PaymentID

	paparaReq := map[string]any{}
	if request.RefundAmount > 0 {
		paparaReq["amount"] = request.RefundAmount
	}
	if request.Description != "" {
		paparaReq["description"] = request.Description
	}
	if request.ConversationID != "" {
		paparaReq["referenceId"] = request.ConversationID
	}
	if request.Currency != "" {
		paparaReq["currency"] = request.Currency
	}

	reqBody, err := json.Marshal(paparaReq)
	if err != nil {
		return nil, fmt.Errorf("papara: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", endpoint, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("papara: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	p.addAuthHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("papara: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("papara: failed to read response: %w", err)
	}

	var paparaResp PaparaResponse
	if err := json.Unmarshal(body, &paparaResp); err != nil {
		return nil, fmt.Errorf("papara: failed to parse response: %w", err)
	}

	return p.mapToRefundResponse(paparaResp), nil
}

// ValidateWebhook validates an incoming webhook notification
func (p *PaparaProvider) ValidateWebhook(ctx context.Context, data, headers map[string]string) (bool, map[string]string, error) {
	// Get the signature from headers
	signature, ok := headers["X-Papara-Signature"]
	if !ok {
		return false, nil, errors.New("papara: missing signature header")
	}

	// Get the payload
	payload, ok := data["payload"]
	if !ok {
		return false, nil, errors.New("papara: missing payload")
	}

	// Validate signature
	expectedSignature := p.generateWebhookSignature(payload)
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return false, nil, errors.New("papara: invalid signature")
	}

	// Parse the payload to extract relevant data
	var webhookData map[string]any
	if err := json.Unmarshal([]byte(payload), &webhookData); err != nil {
		return false, nil, fmt.Errorf("papara: failed to parse webhook payload: %w", err)
	}

	// Return the parsed webhook data
	result := make(map[string]string)
	for k, v := range webhookData {
		if str, ok := v.(string); ok {
			result[k] = str
		} else {
			result[k] = fmt.Sprintf("%v", v)
		}
	}

	return true, result, nil
}

// validatePaymentRequest validates the payment request
func (p *PaparaProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
	if request.TenantID == 0 {
		return errors.New("tenantID is required")
	}

	if request.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}

	if request.Currency == "" {
		return errors.New("currency is required")
	}

	if request.Customer.Email == "" {
		return errors.New("customer email is required")
	}

	if is3D && request.CallbackURL == "" {
		return errors.New("callback URL is required for 3D payments")
	}

	return nil
}

// processPayment processes a payment request
func (p *PaparaProvider) processPayment(ctx context.Context, request provider.PaymentRequest, is3D bool) (*provider.PaymentResponse, error) {
	paparaReq := p.mapToPaparaRequest(request, is3D)

	reqBody, err := json.Marshal(paparaReq)
	if err != nil {
		return nil, fmt.Errorf("papara: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+endpointPayment, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("papara: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	p.addAuthHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("papara: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("papara: failed to read response: %w", err)
	}

	var paparaResp PaparaResponse
	if err := json.Unmarshal(body, &paparaResp); err != nil {
		return nil, fmt.Errorf("papara: failed to parse response: %w", err)
	}

	// add provider request to client request
	_ = provider.AddProviderRequestToClientRequest("papara", "providerRequest", paparaReq, p.logID)

	return p.mapToPaymentResponse(paparaResp), nil
}

// mapToPaparaRequest maps a generic payment request to Papara-specific format
func (p *PaparaProvider) mapToPaparaRequest(request provider.PaymentRequest, _ bool) map[string]any {
	paparaReq := map[string]any{
		"amount":           request.Amount,
		"referenceId":      request.ReferenceID,
		"orderDescription": request.Description,
		"currency":         request.Currency,
	}

	// Add conversation ID if available
	if request.ConversationID != "" {
		paparaReq["conversationId"] = request.ConversationID
	} else {
		paparaReq["conversationId"] = uuid.New().String()
	}

	// Add redirect and notification URLs - route through GoPay
	if request.CallbackURL != "" {
		// Build callback URL through GoPay (like other providers)
		gopayCallbackURL := fmt.Sprintf("%s/v1/callback/papara", p.gopayBaseURL)
		if request.CallbackURL != "" {
			gopayCallbackURL += "?originalCallbackUrl=" + request.CallbackURL
			// Add tenant ID to callback URL for proper tenant identification
			if request.TenantID != 0 {
				gopayCallbackURL += fmt.Sprintf("&tenantId=%d", request.TenantID)
			}
		} else {
			// Add tenant ID to callback URL for proper tenant identification
			if request.TenantID != 0 {
				gopayCallbackURL += fmt.Sprintf("?tenantId=%d", request.TenantID)
			}
		}

		// Add tenant ID to webhook URL for proper tenant identification
		notificationURL := fmt.Sprintf("%s/v1/webhooks/papara", p.gopayBaseURL)
		if request.TenantID != 0 {
			notificationURL += fmt.Sprintf("?tenantId=%d", request.TenantID)
		}
		paparaReq["notificationUrl"] = notificationURL
		paparaReq["redirectUrl"] = gopayCallbackURL
	}

	return paparaReq
}

// mapToPaymentResponse maps Papara response to generic payment response
func (p *PaparaProvider) mapToPaymentResponse(paparaResp PaparaResponse) *provider.PaymentResponse {
	now := time.Now()
	resp := &provider.PaymentResponse{
		Success:          paparaResp.Succeeded,
		TransactionID:    paparaResp.Data.ID,
		PaymentID:        paparaResp.Data.ID,
		Amount:           paparaResp.Data.Amount,
		Currency:         paparaResp.Data.Currency,
		SystemTime:       &now,
		ProviderResponse: paparaResp,
	}

	if paparaResp.Succeeded {
		switch paparaResp.Data.Status {
		case statusPending:
			resp.Status = provider.StatusPending
		case statusCompleted:
			resp.Status = provider.StatusSuccessful
		case statusRefunded:
			resp.Status = provider.StatusRefunded
		case statusFailed:
			resp.Status = provider.StatusFailed
		case statusCancelled:
			resp.Status = provider.StatusCancelled
		default:
			resp.Status = provider.StatusPending
		}

		if paparaResp.Data.PaymentURL != "" {
			resp.RedirectURL = paparaResp.Data.PaymentURL
		}
	} else {
		resp.Status = provider.StatusFailed
		if paparaResp.Error.Message != "" {
			resp.Message = paparaResp.Error.Message
		}
		if paparaResp.Error.Code != "" {
			resp.ErrorCode = paparaResp.Error.Code
		}
	}

	return resp
}

// mapToRefundResponse maps Papara response to generic refund response
func (p *PaparaProvider) mapToRefundResponse(paparaResp PaparaResponse) *provider.RefundResponse {
	now := time.Now()
	resp := &provider.RefundResponse{
		Success:     paparaResp.Succeeded,
		RefundID:    paparaResp.Data.ID,
		PaymentID:   paparaResp.Data.PaymentID,
		SystemTime:  &now,
		RawResponse: paparaResp,
	}

	if paparaResp.Succeeded {
		resp.RefundAmount = paparaResp.Data.Amount
		resp.Status = paparaResp.Data.Status
	} else {
		if paparaResp.Error.Message != "" {
			resp.Message = paparaResp.Error.Message
		}
		if paparaResp.Error.Code != "" {
			resp.ErrorCode = paparaResp.Error.Code
		}
	}

	return resp
}

// addAuthHeaders adds authentication headers to the request
func (p *PaparaProvider) addAuthHeaders(req *http.Request) {
	req.Header.Set("ApiKey", p.apiKey) // Papara API için zorunlu
	req.Header.Set("Accept", "application/json")
	// Eğer Authorization header'ı da gerekiyorsa, Papara dokümantasyonuna göre eklenebilir.
	// req.Header.Set("Authorization", "ApiKey "+p.apiKey)
}

// generateWebhookSignature generates webhook signature for validation
func (p *PaparaProvider) generateWebhookSignature(payload string) string {
	h := hmac.New(sha256.New, []byte(p.apiKey))
	h.Write([]byte(payload))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// PaparaResponse represents the standard Papara API response
type PaparaResponse struct {
	Succeeded bool        `json:"succeeded"`
	Data      PaparaData  `json:"data,omitempty"`
	Error     PaparaError `json:"error,omitempty"`
}

// PaparaData represents the data part of Papara response
type PaparaData struct {
	ID         string  `json:"id"`
	PaymentID  string  `json:"paymentId,omitempty"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	Status     string  `json:"status"`
	PaymentURL string  `json:"paymentUrl,omitempty"`
	CreatedAt  string  `json:"createdAt,omitempty"`
}

// PaparaError represents the error part of Papara response
type PaparaError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ValidationResponse Papara kullanıcı doğrulama response'u için örnek struct
// (Gerekirse gerçek Papara API response'una göre güncellenebilir)
type ValidationResponse struct {
	Succeeded bool `json:"succeeded"`
	Data      any  `json:"data"`
	Error     any  `json:"error"`
}

// ValidateAccountNumber Papara numarası ile kullanıcı doğrulama
func (p *PaparaProvider) ValidateAccountNumber(ctx context.Context, accountNumber string) (*ValidationResponse, error) {
	url := p.baseURL + "/validation/accountNumber?accountNumber=" + accountNumber
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	p.addAuthHeaders(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result ValidationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ValidatePhoneNumber telefon numarası ile kullanıcı doğrulama
func (p *PaparaProvider) ValidatePhoneNumber(ctx context.Context, phoneNumber string) (*ValidationResponse, error) {
	url := p.baseURL + "/validation/phoneNumber?phoneNumber=" + phoneNumber
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	p.addAuthHeaders(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result ValidationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ValidateTCKN TCKN ile kullanıcı doğrulama
func (p *PaparaProvider) ValidateTCKN(ctx context.Context, tckn string) (*ValidationResponse, error) {
	url := p.baseURL + "/validation/tckn?tckn=" + tckn
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	p.addAuthHeaders(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result ValidationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AccountResponse Papara hesap bilgisi response'u için örnek struct
// (Gerekirse gerçek Papara API response'una göre güncellenebilir)
type AccountResponse struct {
	Succeeded bool `json:"succeeded"`
	Data      any  `json:"data"`
	Error     any  `json:"error"`
}

// GetAccountInfo Papara hesabı bilgisi çekme
func (p *PaparaProvider) GetAccountInfo(ctx context.Context) (*AccountResponse, error) {
	url := p.baseURL + "/account"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	p.addAuthHeaders(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result AccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
