package payu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// API URLs
	apiSandboxURL    = "https://secure-test.payu.tr"
	apiProductionURL = "https://secure.payu.tr"

	// API Endpoints
	endpointPayment       = "/api/payment"
	endpointPayment3D     = "/api/payment/3d"
	endpointPaymentStatus = "/api/payment/%s" // %s will be replaced with paymentId
	endpointRefund        = "/api/refund"
	endpointCancel        = "/api/cancel"
	endpointComplete3D    = "/api/payment/3d/complete"

	// PayU Status Codes
	statusSuccess    = "SUCCESS"
	statusPending    = "PENDING"
	statusFailed     = "FAILED"
	statusCancelled  = "CANCELLED"
	statusRefunded   = "REFUNDED"
	statusAuthorized = "AUTHORIZED"

	// Default Values
	defaultCurrency = "TRY"
	defaultLanguage = "tr"
)

// PayUProvider implements the provider.PaymentProvider interface for PayU Turkey
type PayUProvider struct {
	merchantID   string
	secretKey    string
	baseURL      string
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
	httpClient   *provider.ProviderHTTPClient
	logID        int64
}

// NewProvider creates a new PayU Turkey payment provider
func NewProvider() provider.PaymentProvider {
	return &PayUProvider{}
}

// GetRequiredConfig returns the configuration fields required for PayU Turkey
func (p *PayUProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "merchantId",
			Required:    true,
			Type:        "string",
			Description: "PayU Merchant ID (provided by PayU)",
			Example:     "123456",
			MinLength:   3,
			MaxLength:   20,
		},
		{
			Key:         "secretKey",
			Required:    true,
			Type:        "string",
			Description: "PayU Secret Key (provided by PayU)",
			Example:     "PAYU_SECRET_KEY_123",
			MinLength:   10,
			MaxLength:   100,
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

// ValidateConfig validates the provided configuration against PayU Turkey requirements
func (p *PayUProvider) ValidateConfig(config map[string]string) error {
	requiredFields := p.GetRequiredConfig(config["environment"])
	return provider.ValidateConfigFields("payu", config, requiredFields)
}

// Initialize sets up the PayU Turkey payment provider with authentication credentials
func (p *PayUProvider) Initialize(conf map[string]string) error {
	p.merchantID = conf["merchantId"]
	p.secretKey = conf["secretKey"]

	if p.merchantID == "" || p.secretKey == "" {
		return errors.New("payu: merchantId and secretKey are required")
	}

	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	p.isProduction = conf["environment"] == "production"
	if p.isProduction {
		p.baseURL = apiProductionURL
	} else {
		p.baseURL = apiSandboxURL
	}

	// Initialize HTTP client
	p.httpClient = provider.NewProviderHTTPClient(provider.CreateHTTPClientConfig(p.baseURL, p.isProduction))

	return nil
}

// GetInstallmentCount returns the installment count for a payment
func (p *PayUProvider) GetInstallmentCount(ctx context.Context, request provider.InstallmentInquireRequest) (provider.InstallmentInquireResponse, error) {
	return provider.InstallmentInquireResponse{}, nil
}

// GetCommission returns the commission for a payment
func (p *PayUProvider) GetCommission(ctx context.Context, request provider.CommissionRequest) (provider.CommissionResponse, error) {
	return provider.CommissionResponse{}, nil
}

// CreatePayment makes a non-3D payment request
func (p *PayUProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("payu: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *PayUProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("payu: invalid 3D payment request: %w", err)
	}

	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *PayUProvider) Complete3DPayment(ctx context.Context, callbackState *provider.CallbackState, data map[string]string) (*provider.PaymentResponse, error) {
	if callbackState.PaymentID == "" {
		return nil, errors.New("payu: paymentID is required")
	}

	payuReq := p.mapTo3DCompleteRequest(callbackState.PaymentID, callbackState.ConversationID, data)
	respBody, err := p.doPayURequest(ctx, "POST", endpointComplete3D, payuReq, nil)
	if err != nil {
		return nil, fmt.Errorf("payu: request failed: %w", err)
	}

	var payuResp PayUResponse
	if err := json.Unmarshal(respBody, &payuResp); err != nil {
		return nil, fmt.Errorf("payu: failed to parse response: %w", err)
	}

	return p.mapToPaymentResponse(payuResp), nil
}

// GetPaymentStatus retrieves the current status of a payment
func (p *PayUProvider) GetPaymentStatus(ctx context.Context, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("payu: paymentID is required")
	}

	url := fmt.Sprintf(endpointPaymentStatus, request.PaymentID)
	respBody, err := p.doPayURequest(ctx, "GET", url, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("payu: request failed: %w", err)
	}

	var payuResp PayUResponse
	if err := json.Unmarshal(respBody, &payuResp); err != nil {
		return nil, fmt.Errorf("payu: failed to parse response: %w", err)
	}

	return p.mapToPaymentResponse(payuResp), nil
}

// CancelPayment cancels a payment
func (p *PayUProvider) CancelPayment(ctx context.Context, request provider.CancelRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("payu: paymentID is required")
	}

	payuReq := map[string]any{
		"paymentId": request.PaymentID,
		"reason":    request.Reason,
	}

	respBody, err := p.doPayURequest(ctx, "POST", endpointCancel, payuReq, nil)
	if err != nil {
		return nil, fmt.Errorf("payu: request failed: %w", err)
	}

	var payuResp PayUResponse
	if err := json.Unmarshal(respBody, &payuResp); err != nil {
		return nil, fmt.Errorf("payu: failed to parse response: %w", err)
	}

	return p.mapToPaymentResponse(payuResp), nil
}

// RefundPayment issues a refund for a payment
func (p *PayUProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("payu: paymentID is required")
	}

	payuReq := p.mapToRefundRequest(request)
	respBody, err := p.doPayURequest(ctx, "POST", endpointRefund, payuReq, nil)
	if err != nil {
		return nil, fmt.Errorf("payu: request failed: %w", err)
	}

	var payuResp PayURefundResponse
	if err := json.Unmarshal(respBody, &payuResp); err != nil {
		return nil, fmt.Errorf("payu: failed to parse response: %w", err)
	}

	return p.mapToRefundResponse(payuResp), nil
}

// ValidateWebhook validates an incoming webhook notification
func (p *PayUProvider) ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	// PayU webhook validation using signature
	signature, exists := headers["X-PayU-Signature"]
	if !exists {
		return false, nil, errors.New("payu: webhook signature not found")
	}

	// Get webhook payload
	payload, exists := data["payload"]
	if !exists {
		return false, nil, errors.New("payu: webhook payload not found")
	}

	// Calculate expected signature
	expectedSignature := p.calculateWebhookSignature(payload)
	if signature != expectedSignature {
		return false, nil, errors.New("payu: webhook signature validation failed")
	}

	// Parse webhook data
	var webhookData map[string]any
	if err := json.Unmarshal([]byte(payload), &webhookData); err != nil {
		return false, nil, fmt.Errorf("payu: failed to parse webhook payload: %w", err)
	}

	// Convert to string map
	result := make(map[string]string)
	for key, value := range webhookData {
		if str, ok := value.(string); ok {
			result[key] = str
		} else {
			result[key] = fmt.Sprintf("%v", value)
		}
	}

	return true, result, nil
}

// validatePaymentRequest validates the payment request
func (p *PayUProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
	if request.TenantID == 0 {
		return errors.New("tenantID is required")
	}

	if request.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}

	if request.Currency == "" {
		request.Currency = defaultCurrency
	}

	if request.ReferenceID == "" {
		return errors.New("referenceID is required")
	}

	if is3D && request.CallbackURL == "" {
		return errors.New("callbackURL is required for 3D payments")
	}

	// Validate card details if provided
	if request.CardInfo.CardNumber != "" {
		if len(request.CardInfo.CardNumber) < 13 || len(request.CardInfo.CardNumber) > 19 {
			return errors.New("invalid card number length")
		}
		if request.CardInfo.CVV == "" || len(request.CardInfo.CVV) < 3 || len(request.CardInfo.CVV) > 4 {
			return errors.New("invalid CVV")
		}
		expireMonth := request.CardInfo.ExpireMonth
		expireYear := request.CardInfo.ExpireYear
		if expireMonth == "" || expireYear == "" {
			return errors.New("expiry month and year are required")
		}
		// Convert string month to int for validation
		if len(expireMonth) != 2 || expireMonth < "01" || expireMonth > "12" {
			return errors.New("invalid expiry month")
		}
		// Basic year validation (assuming 4-digit year)
		if len(expireYear) != 4 || expireYear < "2020" {
			return errors.New("invalid expiry year")
		}
	}

	return nil
}

// processPayment processes both regular and 3D payments
func (p *PayUProvider) processPayment(ctx context.Context, request provider.PaymentRequest, is3D bool) (*provider.PaymentResponse, error) {
	var endpoint string
	if is3D {
		endpoint = endpointPayment3D
	} else {
		endpoint = endpointPayment
	}

	payuReq := p.mapToPayURequest(request, is3D)
	respBody, err := p.doPayURequest(ctx, "POST", endpoint, payuReq, nil)
	if err != nil {
		return nil, fmt.Errorf("payu: request failed: %w", err)
	}

	var payuResp PayUResponse
	if err := json.Unmarshal(respBody, &payuResp); err != nil {
		return nil, fmt.Errorf("payu: failed to parse response: %w", err)
	}

	_ = provider.AddProviderRequestToClientRequest("payu", "providerRequest", payuReq, p.logID)

	return p.mapToPaymentResponse(payuResp), nil
}

// doPayURequest is a helper to send HTTP requests to PayU API using the shared HTTP client
func (p *PayUProvider) doPayURequest(ctx context.Context, method, endpoint string, body any, extraHeaders map[string]string) ([]byte, error) {
	httpReq := &provider.HTTPRequest{
		Method:   method,
		Endpoint: endpoint,
		Body:     body,
		Headers:  map[string]string{"Authorization": "Bearer " + p.secretKey},
	}
	for k, v := range extraHeaders {
		httpReq.Headers[k] = v
	}
	resp, err := p.httpClient.SendJSON(ctx, httpReq)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// mapToPayURequest maps a generic payment request to PayU-specific format
func (p *PayUProvider) mapToPayURequest(request provider.PaymentRequest, is3D bool) map[string]any {
	payuReq := map[string]any{
		"merchantId":  p.merchantID,
		"amount":      fmt.Sprintf("%.2f", request.Amount),
		"currency":    request.Currency,
		"orderId":     request.ReferenceID,
		"description": request.Description,
		"language":    defaultLanguage,
		"timestamp":   time.Now().Unix(),
	}

	// Add conversation ID if available
	if request.ConversationID != "" {
		payuReq["conversationId"] = request.ConversationID
	} else {
		payuReq["conversationId"] = uuid.New().String()
	}

	// Add card details if provided
	if request.CardInfo.CardNumber != "" {
		payuReq["card"] = map[string]any{
			"number":      request.CardInfo.CardNumber,
			"cvv":         request.CardInfo.CVV,
			"expiryMonth": request.CardInfo.ExpireMonth,
			"expiryYear":  request.CardInfo.ExpireYear,
			"holderName":  request.CardInfo.CardHolderName,
		}
	}

	// Add callback URLs for 3D payments - route through GoPay
	if is3D && request.CallbackURL != "" {
		// Build callback URL through GoPay (like other providers)
		gopayCallbackURL := fmt.Sprintf("%s/v1/callback/payu", p.gopayBaseURL)
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

		payuReq["successUrl"] = gopayCallbackURL
		payuReq["failureUrl"] = gopayCallbackURL
		payuReq["cancelUrl"] = gopayCallbackURL

		// Add tenant ID to webhook URL for proper tenant identification
		notificationURL := fmt.Sprintf("%s/v1/webhooks/payu", p.gopayBaseURL)
		if request.TenantID != 0 {
			notificationURL += fmt.Sprintf("?tenantId=%d", request.TenantID)
		}
		payuReq["notificationUrl"] = notificationURL
	}

	// Add customer info
	if request.Customer.Email != "" || request.Customer.PhoneNumber != "" {
		customer := make(map[string]any)
		if request.Customer.Email != "" {
			customer["email"] = request.Customer.Email
		}
		if request.Customer.PhoneNumber != "" {
			customer["phone"] = request.Customer.PhoneNumber
		}
		payuReq["customer"] = customer
	}

	// Add billing address if provided
	if request.Customer.Address != nil && request.Customer.Address.Country != "" {
		billing := map[string]any{
			"firstName": request.Customer.Name,
			"lastName":  request.Customer.Surname,
			"address":   request.Customer.Address.Address,
			"city":      request.Customer.Address.City,
			"country":   request.Customer.Address.Country,
			"zipCode":   request.Customer.Address.ZipCode,
		}
		payuReq["billingAddress"] = billing
	}

	// Add signature
	payuReq["signature"] = p.generateSignature(payuReq)

	return payuReq
}

// mapTo3DCompleteRequest maps 3D completion data to PayU format
func (p *PayUProvider) mapTo3DCompleteRequest(paymentID, conversationID string, data map[string]string) map[string]any {
	req := map[string]any{
		"merchantId":     p.merchantID,
		"paymentId":      paymentID,
		"conversationId": conversationID,
		"timestamp":      time.Now().Unix(),
	}

	// Add 3D response data
	for key, value := range data {
		req[key] = value
	}

	// Add signature
	req["signature"] = p.generateSignature(req)

	return req
}

// mapToRefundRequest maps a refund request to PayU format
func (p *PayUProvider) mapToRefundRequest(request provider.RefundRequest) map[string]any {
	payuReq := map[string]any{
		"merchantId":  p.merchantID,
		"paymentId":   request.PaymentID,
		"amount":      fmt.Sprintf("%.2f", request.RefundAmount),
		"reason":      request.Reason,
		"description": request.Description,
		"timestamp":   time.Now().Unix(),
	}

	if request.ConversationID != "" {
		payuReq["conversationId"] = request.ConversationID
	}

	if request.Currency != "" {
		payuReq["currency"] = request.Currency
	}

	// Add signature
	payuReq["signature"] = p.generateSignature(payuReq)

	return payuReq
}

// mapToPaymentResponse maps PayU response to generic payment response
func (p *PayUProvider) mapToPaymentResponse(resp PayUResponse) *provider.PaymentResponse {
	// Determine success: either status is success, or it's pending with redirect URL (3D Secure)
	isSuccess := resp.Status == statusSuccess || (resp.Status == statusPending && resp.RedirectURL != "")

	now := time.Now()
	paymentResp := &provider.PaymentResponse{
		Success:          isSuccess,
		PaymentID:        resp.PaymentID,
		TransactionID:    resp.TransactionID,
		Amount:           resp.Amount,
		Currency:         resp.Currency,
		Status:           p.mapPayUStatus(resp.Status),
		Message:          resp.Message,
		SystemTime:       &now,
		ProviderResponse: resp,
	}

	// Set error details if payment failed
	if !paymentResp.Success {
		paymentResp.ErrorCode = resp.ErrorCode
		if resp.ErrorMessage != "" {
			paymentResp.Message = resp.ErrorMessage
		}
	}

	// Set 3D redirect URL if available
	if resp.RedirectURL != "" {
		paymentResp.RedirectURL = resp.RedirectURL
	}

	return paymentResp
}

// mapToRefundResponse maps PayU refund response to generic refund response
func (p *PayUProvider) mapToRefundResponse(resp PayURefundResponse) *provider.RefundResponse {
	now := time.Now()
	refundResp := &provider.RefundResponse{
		Success:      resp.Status == statusSuccess,
		RefundID:     resp.RefundID,
		PaymentID:    resp.PaymentID,
		RefundAmount: resp.Amount,
		Status:       string(p.mapPayUStatus(resp.Status)),
		Message:      resp.Message,
		SystemTime:   &now,
		RawResponse:  resp,
	}

	// Set error details if refund failed
	if !refundResp.Success {
		refundResp.ErrorCode = resp.ErrorCode
		refundResp.Message = resp.ErrorMessage
	}

	return refundResp
}

// mapPayUStatus maps PayU status to generic status
func (p *PayUProvider) mapPayUStatus(status string) provider.PaymentStatus {
	switch status {
	case statusSuccess:
		return provider.StatusSuccessful
	case statusPending:
		return provider.StatusPending
	case statusFailed:
		return provider.StatusFailed
	case statusCancelled:
		return provider.StatusCancelled
	case statusRefunded:
		return provider.StatusRefunded
	case statusAuthorized:
		return provider.StatusProcessing
	default:
		return provider.StatusFailed
	}
}

// generateSignature generates PayU signature for authentication
func (p *PayUProvider) generateSignature(data map[string]any) string {
	// Create signature string from key data fields
	signatureData := fmt.Sprintf("%s|%v|%v|%s",
		p.merchantID,
		data["amount"],
		data["orderId"],
		p.secretKey,
	)

	hash := sha256.Sum256([]byte(signatureData))
	return hex.EncodeToString(hash[:])
}

// calculateWebhookSignature calculates webhook signature for validation
func (p *PayUProvider) calculateWebhookSignature(payload string) string {
	signatureData := p.secretKey + payload
	hash := sha256.Sum256([]byte(signatureData))
	return hex.EncodeToString(hash[:])
}

// PayUResponse represents the standard PayU API response
type PayUResponse struct {
	Status        string  `json:"status"`
	PaymentID     string  `json:"paymentId"`
	TransactionID string  `json:"transactionId"`
	OrderID       string  `json:"orderId"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Message       string  `json:"message"`
	ErrorCode     string  `json:"errorCode,omitempty"`
	ErrorMessage  string  `json:"errorMessage,omitempty"`
	RedirectURL   string  `json:"redirectUrl,omitempty"`
	Timestamp     int64   `json:"timestamp"`
}

// PayURefundResponse represents the PayU refund response
type PayURefundResponse struct {
	Status       string  `json:"status"`
	RefundID     string  `json:"refundId"`
	PaymentID    string  `json:"paymentId"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	Message      string  `json:"message"`
	ErrorCode    string  `json:"errorCode,omitempty"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
	Timestamp    int64   `json:"timestamp"`
}
