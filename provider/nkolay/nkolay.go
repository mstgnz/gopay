package nkolay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// API URLs
	apiSandboxURL    = "https://sandbox-api.nkolay.com"
	apiProductionURL = "https://api.nkolay.com"

	// API Endpoints
	endpointPayment       = "/api/v1/payments"
	endpointPayment3D     = "/api/v1/payments/3d"
	endpointPaymentStatus = "/api/v1/payments/%s" // %s will be replaced with paymentId
	endpointRefund        = "/api/v1/refunds"
	endpointCancel        = "/api/v1/payments/%s/cancel" // %s will be replaced with paymentId

	// Nkolay Status Codes
	statusSuccess    = "SUCCESS"
	statusPending    = "PENDING"
	statusWaiting    = "WAITING"
	statusFailed     = "FAILED"
	statusCancelled  = "CANCELLED"
	statusRefunded   = "REFUNDED"
	statusProcessing = "PROCESSING"

	// Nkolay Error Codes
	errorCodeInsufficientFunds = "INSUFFICIENT_FUNDS"
	errorCodeInvalidCard       = "INVALID_CARD"
	errorCodeExpiredCard       = "EXPIRED_CARD"
	errorCodeFraudulent        = "FRAUDULENT_TRANSACTION"
	errorCodeDeclined          = "CARD_DECLINED"
	errorCodeSystemError       = "SYSTEM_ERROR"

	// Default Values
	defaultCurrency = "TRY"
	defaultTimeout  = 30 * time.Second
)

// NkolayProvider implements the provider.PaymentProvider interface for Nkolay
type NkolayProvider struct {
	apiKey       string
	secretKey    string
	merchantID   string
	baseURL      string
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
	client       *http.Client
}

// NewProvider creates a new Nkolay payment provider
func NewProvider() provider.PaymentProvider {
	return &NkolayProvider{
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Initialize sets up the Nkolay payment provider with authentication credentials
func (p *NkolayProvider) Initialize(conf map[string]string) error {
	p.apiKey = conf["apiKey"]
	p.secretKey = conf["secretKey"]
	p.merchantID = conf["merchantId"]

	if p.apiKey == "" || p.secretKey == "" || p.merchantID == "" {
		return errors.New("nkolay: apiKey, secretKey and merchantId are required")
	}

	// Set GoPay base URL for callbacks
	if gopayBaseURL, ok := conf["gopayBaseURL"]; ok && gopayBaseURL != "" {
		p.gopayBaseURL = gopayBaseURL
	} else {
		p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")
	}

	p.isProduction = conf["environment"] == "production"
	if p.isProduction {
		p.baseURL = apiProductionURL
	} else {
		p.baseURL = apiSandboxURL
	}

	return nil
}

// CreatePayment makes a non-3D payment request
func (p *NkolayProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("nkolay: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *NkolayProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("nkolay: invalid 3D payment request: %w", err)
	}

	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *NkolayProvider) Complete3DPayment(ctx context.Context, paymentID, conversationID string, data map[string]string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	// For Nkolay, we need to verify the 3D secure result and complete the payment
	endpoint := fmt.Sprintf("/api/v1/payments/%s/complete3d", paymentID)

	nkolayReq := map[string]any{
		"paymentId":      paymentID,
		"conversationId": conversationID,
		"callbackData":   data,
		"timestamp":      time.Now().Unix(),
		"merchantId":     p.merchantID,
	}

	return p.sendRequest(ctx, endpoint, nkolayReq)
}

// GetPaymentStatus retrieves the current status of a payment
func (p *NkolayProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	endpoint := fmt.Sprintf(endpointPaymentStatus, paymentID)

	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to create request: %w", err)
	}

	p.addAuthHeaders(req, "")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nkolay: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to read response: %w", err)
	}

	var nkolayResp NkolayResponse
	if err := json.Unmarshal(body, &nkolayResp); err != nil {
		return nil, fmt.Errorf("nkolay: failed to parse response: %w", err)
	}

	return p.mapToPaymentResponse(nkolayResp), nil
}

// CancelPayment cancels a payment
func (p *NkolayProvider) CancelPayment(ctx context.Context, paymentID, reason string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	endpoint := fmt.Sprintf(endpointCancel, paymentID)

	nkolayReq := map[string]any{
		"paymentId":  paymentID,
		"reason":     reason,
		"timestamp":  time.Now().Unix(),
		"merchantId": p.merchantID,
	}

	return p.sendRequest(ctx, endpoint, nkolayReq)
}

// RefundPayment issues a refund for a payment
func (p *NkolayProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	nkolayReq := map[string]any{
		"paymentId":   request.PaymentID,
		"amount":      request.RefundAmount,
		"reason":      request.Reason,
		"description": request.Description,
		"currency":    request.Currency,
		"timestamp":   time.Now().Unix(),
		"merchantId":  p.merchantID,
	}

	if request.Currency == "" {
		nkolayReq["currency"] = defaultCurrency
	}

	if request.ConversationID != "" {
		nkolayReq["conversationId"] = request.ConversationID
	} else {
		nkolayReq["conversationId"] = uuid.New().String()
	}

	response, err := p.sendRequest(ctx, endpointRefund, nkolayReq)
	if err != nil {
		return nil, err
	}

	// Convert payment response to refund response
	refundResp := &provider.RefundResponse{
		Success:      response.Success,
		RefundID:     response.TransactionID,
		PaymentID:    request.PaymentID,
		Status:       string(response.Status),
		RefundAmount: request.RefundAmount,
		Message:      response.Message,
		ErrorCode:    response.ErrorCode,
		SystemTime:   response.SystemTime,
	}

	return refundResp, nil
}

// ValidateWebhook validates an incoming webhook notification
func (p *NkolayProvider) ValidateWebhook(ctx context.Context, data, headers map[string]string) (bool, map[string]string, error) {
	// Get the signature from headers
	signature := headers["X-Nkolay-Signature"]
	if signature == "" {
		return false, nil, errors.New("nkolay: missing signature header")
	}

	// Get the timestamp to prevent replay attacks
	timestamp := headers["X-Nkolay-Timestamp"]
	if timestamp == "" {
		return false, nil, errors.New("nkolay: missing timestamp header")
	}

	// Reconstruct the payload for signature verification
	payload := ""
	for key, value := range data {
		payload += key + "=" + value + "&"
	}
	payload = strings.TrimSuffix(payload, "&")

	// Calculate expected signature
	expectedSignature := p.generateSignature(payload + timestamp)

	// Compare signatures
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return false, nil, errors.New("nkolay: invalid webhook signature")
	}

	// Extract payment information from webhook data
	webhookData := make(map[string]string)
	for key, value := range data {
		webhookData[key] = value
	}

	return true, webhookData, nil
}

// validatePaymentRequest validates the payment request
func (p *NkolayProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
	if request.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}

	if request.Currency == "" {
		return errors.New("currency is required")
	}

	if request.Customer.Email == "" {
		return errors.New("customer email is required")
	}

	if request.CardInfo.CardNumber == "" {
		return errors.New("card number is required")
	}

	if request.CardInfo.ExpireMonth == "" || request.CardInfo.ExpireYear == "" {
		return errors.New("card expiry month and year are required")
	}

	if request.CardInfo.CVV == "" {
		return errors.New("card CVV is required")
	}

	if is3D && request.CallbackURL == "" {
		return errors.New("callback URL is required for 3D payments")
	}

	return nil
}

// processPayment handles both 3D and non-3D payments
func (p *NkolayProvider) processPayment(ctx context.Context, request provider.PaymentRequest, is3D bool) (*provider.PaymentResponse, error) {
	endpoint := endpointPayment
	if is3D {
		endpoint = endpointPayment3D
	}

	nkolayReq := p.mapToNkolayRequest(request, is3D)
	return p.sendRequest(ctx, endpoint, nkolayReq)
}

// mapToNkolayRequest converts a PaymentRequest to Nkolay's request format
func (p *NkolayProvider) mapToNkolayRequest(request provider.PaymentRequest, is3D bool) map[string]any {
	// Generate unique IDs if not provided
	conversationID := request.ConversationID
	if conversationID == "" {
		conversationID = uuid.New().String()
	}

	paymentID := request.ID
	if paymentID == "" {
		paymentID = uuid.New().String()
	}

	nkolayReq := map[string]any{
		"merchantId":     p.merchantID,
		"paymentId":      paymentID,
		"conversationId": conversationID,
		"amount":         request.Amount,
		"currency":       request.Currency,
		"description":    request.Description,
		"timestamp":      time.Now().Unix(),
		"customer": map[string]any{
			"id":          request.Customer.ID,
			"name":        request.Customer.Name,
			"surname":     request.Customer.Surname,
			"email":       request.Customer.Email,
			"phoneNumber": request.Customer.PhoneNumber,
			"ipAddress":   request.Customer.IPAddress,
		},
		"card": map[string]any{
			"cardHolderName": request.CardInfo.CardHolderName,
			"cardNumber":     request.CardInfo.CardNumber,
			"expireMonth":    request.CardInfo.ExpireMonth,
			"expireYear":     request.CardInfo.ExpireYear,
			"cvv":            request.CardInfo.CVV,
		},
	}

	// Add customer address if provided
	if request.Customer.Address.Address != "" {
		nkolayReq["customer"].(map[string]any)["address"] = map[string]any{
			"city":        request.Customer.Address.City,
			"country":     request.Customer.Address.Country,
			"address":     request.Customer.Address.Address,
			"zipCode":     request.Customer.Address.ZipCode,
			"description": request.Customer.Address.Description,
		}
	}

	// Add items if provided
	if len(request.Items) > 0 {
		items := make([]map[string]any, len(request.Items))
		for i, item := range request.Items {
			items[i] = map[string]any{
				"id":          item.ID,
				"name":        item.Name,
				"description": item.Description,
				"category":    item.Category,
				"price":       item.Price,
				"quantity":    item.Quantity,
			}
		}
		nkolayReq["items"] = items
	}

	// Add 3D specific fields
	if is3D {
		callbackURL := request.CallbackURL
		if callbackURL == "" {
			callbackURL = p.gopayBaseURL + "/v1/callback/nkolay"
			// Add tenant ID to callback URL for proper tenant identification
			if request.TenantID != "" {
				callbackURL += fmt.Sprintf("?tenantId=%s", request.TenantID)
			}
		} else {
			// Build GoPay's own callback URL with user's original callback URL as parameter
			gopayCallbackURL := fmt.Sprintf("%s/v1/callback/nkolay?originalCallbackUrl=%s", p.gopayBaseURL, callbackURL)
			if request.TenantID != "" {
				gopayCallbackURL += fmt.Sprintf("&tenantId=%s", request.TenantID)
			}
			callbackURL = gopayCallbackURL
		}
		nkolayReq["callbackUrl"] = callbackURL
		nkolayReq["use3D"] = true
	}

	// Add installment count if specified
	if request.InstallmentCount > 0 {
		nkolayReq["installmentCount"] = request.InstallmentCount
	}

	// Add metadata if provided
	if request.MetaData != "" {
		nkolayReq["metaData"] = request.MetaData
	}

	return nkolayReq
}

// mapToPaymentResponse converts a Nkolay response to PaymentResponse
func (p *NkolayProvider) mapToPaymentResponse(nkolayResp NkolayResponse) *provider.PaymentResponse {
	status := provider.StatusFailed
	switch nkolayResp.Status {
	case statusSuccess:
		status = provider.StatusSuccessful
	case statusPending:
		status = provider.StatusPending
	case statusWaiting:
		status = provider.StatusProcessing
	case statusFailed:
		status = provider.StatusFailed
	case statusCancelled:
		status = provider.StatusCancelled
	case statusRefunded:
		status = provider.StatusRefunded
	case statusProcessing:
		status = provider.StatusProcessing
	}

	return &provider.PaymentResponse{
		Success:          nkolayResp.Success,
		Status:           status,
		Message:          nkolayResp.Message,
		ErrorCode:        nkolayResp.ErrorCode,
		TransactionID:    nkolayResp.TransactionID,
		PaymentID:        nkolayResp.PaymentID,
		Amount:           nkolayResp.Amount,
		Currency:         nkolayResp.Currency,
		RedirectURL:      nkolayResp.RedirectURL,
		HTML:             nkolayResp.HTML,
		SystemTime:       time.Now(),
		FraudStatus:      nkolayResp.FraudStatus,
		ProviderResponse: nkolayResp,
	}
}

// sendRequest sends a request to Nkolay API
func (p *NkolayProvider) sendRequest(ctx context.Context, endpoint string, data map[string]any) (*provider.PaymentResponse, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+endpoint, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	p.addAuthHeaders(req, string(jsonData))

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nkolay: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to read response: %w", err)
	}

	var nkolayResp NkolayResponse
	if err := json.Unmarshal(body, &nkolayResp); err != nil {
		return nil, fmt.Errorf("nkolay: failed to parse response: %w", err)
	}

	return p.mapToPaymentResponse(nkolayResp), nil
}

// addAuthHeaders adds authentication headers to the request
func (p *NkolayProvider) addAuthHeaders(req *http.Request, body string) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := p.generateAuthSignature(req.Method, req.URL.Path, body, timestamp)

	req.Header.Set("X-Nkolay-ApiKey", p.apiKey)
	req.Header.Set("X-Nkolay-Timestamp", timestamp)
	req.Header.Set("X-Nkolay-Signature", signature)
	req.Header.Set("X-Nkolay-MerchantId", p.merchantID)
}

// generateAuthSignature generates the authentication signature for requests
func (p *NkolayProvider) generateAuthSignature(method, path, body, timestamp string) string {
	data := method + path + body + timestamp + p.merchantID
	return p.generateSignature(data)
}

// generateSignature generates HMAC-SHA256 signature
func (p *NkolayProvider) generateSignature(data string) string {
	h := hmac.New(sha256.New, []byte(p.secretKey))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// NkolayResponse represents the response structure from Nkolay API
type NkolayResponse struct {
	Success       bool    `json:"success"`
	Status        string  `json:"status"`
	PaymentID     string  `json:"paymentId"`
	TransactionID string  `json:"transactionId"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Message       string  `json:"message"`
	ErrorCode     string  `json:"errorCode"`
	RedirectURL   string  `json:"redirectUrl,omitempty"`
	HTML          string  `json:"html,omitempty"`
	FraudStatus   int     `json:"fraudStatus,omitempty"`
	Timestamp     int64   `json:"timestamp"`
}
