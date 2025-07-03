package paycell

import (
	"context"
	"crypto/md5"
	"encoding/hex"
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
	apiSandboxURL    = "https://test.paycell.com.tr"
	apiProductionURL = "https://api.paycell.com.tr"

	// API Endpoints
	endpointPayment       = "/api/payments"
	endpointPayment3D     = "/api/payments/3d"
	endpointPaymentStatus = "/api/payments/%s" // %s will be replaced with paymentId
	endpointRefund        = "/api/refunds"
	endpointCancel        = "/api/payments/%s/cancel" // %s will be replaced with paymentId

	// Paycell Status Codes
	statusSuccess    = "SUCCESS"
	statusPending    = "PENDING"
	statusWaiting    = "WAITING"
	statusFailed     = "FAILED"
	statusCancelled  = "CANCELLED"
	statusRefunded   = "REFUNDED"
	statusProcessing = "PROCESSING"

	// Paycell Error Codes
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

// PaycellProvider implements the provider.PaymentProvider interface for Paycell
type PaycellProvider struct {
	username     string
	password     string
	merchantID   string
	terminalID   string
	baseURL      string
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
	client       *http.Client
}

// NewProvider creates a new Paycell payment provider
func NewProvider() provider.PaymentProvider {
	return &PaycellProvider{
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Initialize sets up the Paycell payment provider with authentication credentials
func (p *PaycellProvider) Initialize(conf map[string]string) error {
	p.username = conf["username"]
	p.password = conf["password"]
	p.merchantID = conf["merchantId"]
	p.terminalID = conf["terminalId"]

	if p.username == "" || p.password == "" || p.merchantID == "" || p.terminalID == "" {
		return errors.New("paycell: username, password, merchantId and terminalId are required")
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
func (p *PaycellProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("paycell: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *PaycellProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("paycell: invalid 3D payment request: %w", err)
	}

	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *PaycellProvider) Complete3DPayment(ctx context.Context, paymentID, conversationID string, data map[string]string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	// For Paycell, we need to verify the 3D secure result and complete the payment
	// This is typically done by calling a completion endpoint with the callback data
	endpoint := fmt.Sprintf("/api/payments/%s/complete3d", paymentID)

	paycellReq := map[string]any{
		"paymentId":      paymentID,
		"conversationId": conversationID,
		"callbackData":   data,
		"timestamp":      time.Now().Unix(),
	}

	return p.sendRequest(ctx, endpoint, paycellReq)
}

// GetPaymentStatus retrieves the current status of a payment
func (p *PaycellProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	endpoint := fmt.Sprintf(endpointPaymentStatus, paymentID)

	// For GET requests, we don't send a body but we might need auth headers
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to create request: %w", err)
	}

	p.addAuthHeaders(req, "")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paycell: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to read response: %w", err)
	}

	var paycellResp PaycellResponse
	if err := json.Unmarshal(body, &paycellResp); err != nil {
		return nil, fmt.Errorf("paycell: failed to parse response: %w", err)
	}

	return p.mapToPaymentResponse(paycellResp), nil
}

// CancelPayment cancels a payment
func (p *PaycellProvider) CancelPayment(ctx context.Context, paymentID, reason string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	endpoint := fmt.Sprintf(endpointCancel, paymentID)

	paycellReq := map[string]any{
		"paymentId":  paymentID,
		"reason":     reason,
		"timestamp":  time.Now().Unix(),
		"merchantId": p.merchantID,
		"terminalId": p.terminalID,
	}

	return p.sendRequest(ctx, endpoint, paycellReq)
}

// RefundPayment issues a refund for a payment
func (p *PaycellProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	refundAmount := request.RefundAmount
	if refundAmount <= 0 {
		// If no specific amount, this is a full refund
		// We'll need to get the original payment amount
		paymentResp, err := p.GetPaymentStatus(ctx, request.PaymentID)
		if err != nil {
			return nil, fmt.Errorf("paycell: failed to get payment for refund: %w", err)
		}
		refundAmount = paymentResp.Amount
	}

	paycellReq := map[string]any{
		"paymentId":      request.PaymentID,
		"refundAmount":   int64(refundAmount * 100), // Convert to cents
		"reason":         request.Reason,
		"description":    request.Description,
		"currency":       request.Currency,
		"conversationId": request.ConversationID,
		"timestamp":      time.Now().Unix(),
		"merchantId":     p.merchantID,
		"terminalId":     p.terminalID,
	}

	response, err := p.sendRequest(ctx, endpointRefund, paycellReq)
	if err != nil {
		return nil, err
	}

	// Convert PaymentResponse to RefundResponse
	return &provider.RefundResponse{
		Success:      response.Success,
		RefundID:     response.TransactionID, // Use transaction ID as refund ID
		PaymentID:    request.PaymentID,
		Status:       string(response.Status),
		RefundAmount: refundAmount,
		Message:      response.Message,
	}, nil
}

// ValidateWebhook validates an incoming webhook notification
func (p *PaycellProvider) ValidateWebhook(ctx context.Context, data, headers map[string]string) (bool, map[string]string, error) {
	// Paycell webhook validation typically involves checking a signature
	signature := headers["X-Paycell-Signature"]
	if signature == "" {
		return false, nil, errors.New("paycell: missing webhook signature")
	}

	// Calculate expected signature
	rawData, _ := json.Marshal(data)
	expectedSignature := p.generateSignature(string(rawData))

	if signature != expectedSignature {
		return false, nil, errors.New("paycell: invalid webhook signature")
	}

	// Extract payment information from webhook data
	result := map[string]string{
		"paymentId":     data["paymentId"],
		"status":        data["status"],
		"transactionId": data["transactionId"],
		"amount":        data["amount"],
		"currency":      data["currency"],
	}

	return true, result, nil
}

// validatePaymentRequest validates the payment request
func (p *PaycellProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
	if request.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}

	if request.Currency == "" {
		return errors.New("currency is required")
	}

	if request.Customer.Email == "" {
		return errors.New("customer email is required")
	}

	if request.Customer.Name == "" {
		return errors.New("customer name is required")
	}

	if request.Customer.Surname == "" {
		return errors.New("customer surname is required")
	}

	if request.CardInfo.CardNumber == "" {
		return errors.New("card number is required")
	}

	if request.CardInfo.CVV == "" {
		return errors.New("CVV is required")
	}

	if request.CardInfo.ExpireMonth == "" {
		return errors.New("expire month is required")
	}

	if request.CardInfo.ExpireYear == "" {
		return errors.New("expire year is required")
	}

	if is3D && request.CallbackURL == "" {
		return errors.New("callback URL is required for 3D payments")
	}

	return nil
}

// processPayment processes a payment request
func (p *PaycellProvider) processPayment(ctx context.Context, request provider.PaymentRequest, is3D bool) (*provider.PaymentResponse, error) {
	var endpoint string
	if is3D {
		endpoint = endpointPayment3D
	} else {
		endpoint = endpointPayment
	}

	paycellReq := p.mapToPaycellRequest(request, is3D)
	return p.sendRequest(ctx, endpoint, paycellReq)
}

// mapToPaycellRequest converts a standard payment request to Paycell format
func (p *PaycellProvider) mapToPaycellRequest(request provider.PaymentRequest, is3D bool) map[string]any {
	// Generate unique payment ID
	paymentID := "paycell_" + uuid.New().String()

	paycellReq := map[string]any{
		"paymentId":      paymentID,
		"merchantId":     p.merchantID,
		"terminalId":     p.terminalID,
		"amount":         int64(request.Amount * 100), // Convert to cents
		"currency":       request.Currency,
		"description":    request.Description,
		"conversationId": request.ConversationID,
		"timestamp":      time.Now().Unix(),

		// Customer information
		"customer": map[string]any{
			"name":    request.Customer.Name,
			"surname": request.Customer.Surname,
			"email":   request.Customer.Email,
			"phone":   request.Customer.PhoneNumber,
			"address": map[string]any{
				"country": request.Customer.Address.Country,
				"city":    request.Customer.Address.City,
				"address": request.Customer.Address.Address,
				"zipCode": request.Customer.Address.ZipCode,
			},
		},

		// Card information
		"card": map[string]any{
			"cardNumber":     request.CardInfo.CardNumber,
			"expireMonth":    request.CardInfo.ExpireMonth,
			"expireYear":     request.CardInfo.ExpireYear,
			"cvv":            request.CardInfo.CVV,
			"cardHolderName": request.CardInfo.CardHolderName,
		},
	}

	// Add 3D secure specific fields
	if is3D {
		callbackURL := request.CallbackURL
		if callbackURL == "" {
			// Use GoPay's callback URL if not provided
			callbackURL = fmt.Sprintf("%s/v1/callback/paycell", p.gopayBaseURL)
			// Add tenant ID to callback URL for proper tenant identification
			if request.TenantID != "" {
				callbackURL += fmt.Sprintf("?tenantId=%s", request.TenantID)
			}
		} else {
			// Build GoPay's own callback URL with user's original callback URL as parameter
			gopayCallbackURL := fmt.Sprintf("%s/v1/callback/paycell?originalCallbackUrl=%s", p.gopayBaseURL, callbackURL)
			if request.TenantID != "" {
				gopayCallbackURL += fmt.Sprintf("&tenantId=%s", request.TenantID)
			}
			callbackURL = gopayCallbackURL
		}

		paycellReq["callbackUrl"] = callbackURL
		paycellReq["secure3d"] = true
	}

	// Add items if provided
	if len(request.Items) > 0 {
		var items []map[string]any
		for _, item := range request.Items {
			items = append(items, map[string]any{
				"name":     item.Name,
				"price":    int64(item.Price * 100), // Convert to cents
				"quantity": item.Quantity,
			})
		}
		paycellReq["items"] = items
	}

	return paycellReq
}

// mapToPaymentResponse converts Paycell response to standard payment response
func (p *PaycellProvider) mapToPaymentResponse(paycellResp PaycellResponse) *provider.PaymentResponse {
	response := &provider.PaymentResponse{
		Success:          paycellResp.Success,
		PaymentID:        paycellResp.PaymentID,
		TransactionID:    paycellResp.TransactionID,
		Amount:           float64(paycellResp.Amount) / 100, // Convert from cents
		Currency:         paycellResp.Currency,
		Message:          paycellResp.Message,
		ErrorCode:        paycellResp.ErrorCode,
		SystemTime:       time.Now(),
		ProviderResponse: paycellResp,
	}

	// Map Paycell status to standard status
	switch paycellResp.Status {
	case statusSuccess:
		response.Status = provider.StatusSuccessful
		response.Success = true
	case statusPending, statusWaiting, statusProcessing:
		response.Status = provider.StatusPending
	case statusFailed:
		response.Status = provider.StatusFailed
		response.Success = false
	case statusCancelled:
		response.Status = provider.StatusCancelled
		response.Success = true
	case statusRefunded:
		response.Status = provider.StatusRefunded
		response.Success = true
	default:
		response.Status = provider.StatusFailed
		response.Success = false
	}

	// Add 3D secure information if present
	if paycellResp.RedirectURL != "" {
		response.RedirectURL = paycellResp.RedirectURL
		response.Status = provider.StatusPending
	}

	if paycellResp.HTML != "" {
		response.HTML = paycellResp.HTML
	}

	return response
}

// sendRequest sends a request to Paycell API
func (p *PaycellProvider) sendRequest(ctx context.Context, endpoint string, data map[string]any) (*provider.PaymentResponse, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+endpoint, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	p.addAuthHeaders(req, string(jsonData))

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paycell: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to read response: %w", err)
	}

	var paycellResp PaycellResponse
	if err := json.Unmarshal(body, &paycellResp); err != nil {
		return nil, fmt.Errorf("paycell: failed to parse response: %w", err)
	}

	return p.mapToPaymentResponse(paycellResp), nil
}

// addAuthHeaders adds authentication headers to the request
func (p *PaycellProvider) addAuthHeaders(req *http.Request, body string) {
	// Paycell typically uses username/password or signature-based auth
	// For this implementation, we'll use a simple signature approach
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := p.generateAuthSignature(req.Method, req.URL.Path, body, timestamp)

	req.Header.Set("X-Paycell-Username", p.username)
	req.Header.Set("X-Paycell-Timestamp", timestamp)
	req.Header.Set("X-Paycell-Signature", signature)
}

// generateAuthSignature generates authentication signature
func (p *PaycellProvider) generateAuthSignature(method, path, body, timestamp string) string {
	// Create signature string: METHOD|PATH|BODY|TIMESTAMP|PASSWORD
	data := method + "|" + path + "|" + body + "|" + timestamp + "|" + p.password
	return p.generateSignature(data)
}

// generateSignature generates MD5 signature for Paycell
func (p *PaycellProvider) generateSignature(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// PaycellResponse represents a response from Paycell API
type PaycellResponse struct {
	Success       bool   `json:"success"`
	Status        string `json:"status"`
	PaymentID     string `json:"paymentId"`
	TransactionID string `json:"transactionId"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Message       string `json:"message"`
	ErrorCode     string `json:"errorCode"`
	RedirectURL   string `json:"redirectUrl,omitempty"`
	HTML          string `json:"html,omitempty"`
}
