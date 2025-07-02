package ozanpay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mstgnz/gopay/provider"
)

const (
	// API URLs
	apiSandboxURL    = "https://sandbox-api.ozan.com"
	apiProductionURL = "https://api.ozan.com"

	// API Endpoints
	endpointPayment       = "/api/v1/payments"
	endpointRefund        = "/api/v1/refunds"
	endpointPaymentStatus = "/api/v1/payments/%s" // %s will be replaced with paymentId

	// OzanPay Status Codes
	statusApproved   = "APPROVED"
	statusAuthorized = "AUTHORIZED"
	statusPending    = "PENDING"
	statusProcessing = "PROCESSING"
	statusDeclined   = "DECLINED"
	statusFailed     = "FAILED"
	statusCancelled  = "CANCELLED"
	statusRefunded   = "REFUNDED"

	// OzanPay Error Codes
	errorCodeInsufficientFunds = "INSUFFICIENT_FUNDS"
	errorCodeInvalidCard       = "INVALID_CARD"
	errorCodeExpiredCard       = "EXPIRED_CARD"
	errorCodeFraudulent        = "FRAUDULENT_TRANSACTION"
	errorCodeDeclined          = "CARD_DECLINED"

	// Default Values
	defaultTimeout = 30 * time.Second
)

// OzanPayProvider implements the provider.PaymentProvider interface for OzanPay
type OzanPayProvider struct {
	apiKey       string
	secretKey    string
	merchantID   string
	baseURL      string
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
	client       *http.Client
}

// NewProvider creates a new OzanPay payment provider
func NewProvider() provider.PaymentProvider {
	return &OzanPayProvider{
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Initialize sets up the OzanPay payment provider with authentication credentials
func (p *OzanPayProvider) Initialize(config map[string]string) error {
	p.apiKey = config["apiKey"]
	p.secretKey = config["secretKey"]
	p.merchantID = config["merchantId"]

	if p.apiKey == "" || p.secretKey == "" || p.merchantID == "" {
		return errors.New("ozanpay: apiKey, secretKey and merchantId are required")
	}

	// Set GoPay base URL for callbacks
	if gopayBaseURL, ok := config["gopayBaseURL"]; ok && gopayBaseURL != "" {
		p.gopayBaseURL = gopayBaseURL
	} else {
		p.gopayBaseURL = "http://localhost:9999" // Default fallback
	}

	p.isProduction = config["environment"] == "production"
	if p.isProduction {
		p.baseURL = apiProductionURL
	} else {
		p.baseURL = apiSandboxURL
	}

	return nil
}

// CreatePayment makes a non-3D payment request
func (p *OzanPayProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("ozanpay: invalid payment request: %w", err)
	}

	// OzanPay doesn't differentiate between 3D and non-3D in the initial API call
	// Instead it decides based on the card and the payment amount
	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *OzanPayProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("ozanpay: invalid 3D payment request: %w", err)
	}

	// Force 3D secure for this payment
	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *OzanPayProvider) Complete3DPayment(ctx context.Context, paymentID string, conversationID string, data map[string]string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("ozanpay: paymentID is required for 3D completion")
	}

	// In OzanPay, we don't need to complete the 3D payment with a separate call
	// The payment status should be checked instead
	return p.GetPaymentStatus(ctx, paymentID)
}

// GetPaymentStatus retrieves the current status of a payment
func (p *OzanPayProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("ozanpay: paymentID is required")
	}

	// Format the endpoint with the payment ID
	endpoint := fmt.Sprintf(endpointPaymentStatus, paymentID)

	// Send request to get payment status
	response, err := p.sendRequest(ctx, endpoint, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	// Map OzanPay response to our common PaymentResponse
	return p.mapToPaymentResponse(response)
}

// CancelPayment cancels a payment
func (p *OzanPayProvider) CancelPayment(ctx context.Context, paymentID string, reason string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("ozanpay: paymentID is required")
	}

	// OzanPay doesn't have a specific cancel endpoint, it's handled through the refund endpoint
	// so we'll treat this as a full refund
	refundReq := provider.RefundRequest{
		PaymentID: paymentID,
		Reason:    reason,
	}

	// Get payment details first to determine the amount
	paymentResp, err := p.GetPaymentStatus(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	// Set the refund amount to the full payment amount
	refundReq.RefundAmount = paymentResp.Amount
	refundReq.Currency = paymentResp.Currency

	// Process the refund
	refundResp, err := p.RefundPayment(ctx, refundReq)
	if err != nil {
		return nil, err
	}

	// Convert the refund response to a payment response
	paymentResp = &provider.PaymentResponse{
		Success:    refundResp.Success,
		Status:     provider.StatusCancelled,
		Message:    refundResp.Message,
		ErrorCode:  refundResp.ErrorCode,
		PaymentID:  paymentID,
		Amount:     refundResp.RefundAmount,
		SystemTime: refundResp.SystemTime,
	}

	return paymentResp, nil
}

// RefundPayment issues a refund for a payment
func (p *OzanPayProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("ozanpay: paymentID is required for refund")
	}

	// Prepare refund request data for OzanPay
	refundData := map[string]any{
		"parentId":    request.PaymentID,
		"amount":      int64(request.RefundAmount * 100), // Convert to minor units
		"description": request.Description,
		"reason":      request.Reason,
		"metadata":    fmt.Sprintf(`{"conversationId":"%s"}`, request.ConversationID),
	}

	if request.ConversationID == "" {
		refundData["metadata"] = fmt.Sprintf(`{"conversationId":"%s"}`, uuid.New().String())
	}

	// Send refund request
	response, err := p.sendRequest(ctx, endpointRefund, http.MethodPost, refundData)
	if err != nil {
		return nil, err
	}

	// Process response
	refundResp := &provider.RefundResponse{
		Success:      response["status"] == statusApproved,
		PaymentID:    request.PaymentID,
		RefundAmount: request.RefundAmount,
		SystemTime:   time.Now(),
		RawResponse:  response,
	}

	// Extract refund ID
	if refundID, ok := response["id"].(string); ok {
		refundResp.RefundID = refundID
	}

	// Handle error responses
	if response["status"] != statusApproved {
		refundResp.Success = false
		if errMsg, ok := response["errorMessage"].(string); ok && errMsg != "" {
			refundResp.Message = errMsg
		} else {
			refundResp.Message = "Refund failed"
		}
		if errCode, ok := response["errorCode"].(string); ok && errCode != "" {
			refundResp.ErrorCode = errCode
		}
	} else {
		refundResp.Status = "success"
		refundResp.Message = "Refund successful"
	}

	return refundResp, nil
}

// ValidateWebhook validates an incoming webhook notification
func (p *OzanPayProvider) ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	// Check for signature header
	signature, ok := headers["X-Ozan-Signature"]
	if !ok {
		return false, nil, errors.New("missing X-Ozan-Signature header")
	}

	// Get the raw payload as JSON string
	rawJson, err := json.Marshal(data)
	if err != nil {
		return false, nil, fmt.Errorf("failed to marshal webhook data: %w", err)
	}

	// Calculate expected signature
	h := hmac.New(sha256.New, []byte(p.secretKey))
	h.Write(rawJson)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	// Compare signatures
	if signature != expectedSignature {
		return false, nil, errors.New("invalid webhook signature")
	}

	// Extract payment details
	result := make(map[string]string)
	if paymentID, ok := data["id"]; ok {
		result["paymentId"] = paymentID
	}
	if status, ok := data["status"]; ok {
		result["status"] = status
	}

	return true, result, nil
}

// validatePaymentRequest validates the payment request
func (p *OzanPayProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
	if request.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}

	if request.Currency == "" {
		return errors.New("currency is required")
	}

	if request.Customer.Email == "" {
		return errors.New("customer email is required")
	}

	if request.Customer.Name == "" || request.Customer.Surname == "" {
		return errors.New("customer name and surname are required")
	}

	if request.CardInfo.CardNumber == "" {
		return errors.New("card number is required")
	}

	if request.CardInfo.CVV == "" {
		return errors.New("CVV is required")
	}

	if request.CardInfo.ExpireMonth == "" || request.CardInfo.ExpireYear == "" {
		return errors.New("card expiration month and year are required")
	}

	if is3D && request.CallbackURL == "" {
		return errors.New("callback URL is required for 3D secure payments")
	}

	return nil
}

// Helper method to process a payment
func (p *OzanPayProvider) processPayment(ctx context.Context, request provider.PaymentRequest, force3D bool) (*provider.PaymentResponse, error) {
	// Map request to OzanPay format
	paymentData := p.mapToOzanPayRequest(request, force3D)

	// Send payment request
	response, err := p.sendRequest(ctx, endpointPayment, http.MethodPost, paymentData)
	if err != nil {
		return nil, err
	}

	// Map response to common format
	return p.mapToPaymentResponse(response)
}

// Helper method to map our common request to OzanPay format
func (p *OzanPayProvider) mapToOzanPayRequest(request provider.PaymentRequest, force3D bool) map[string]any {
	// Format amount - OzanPay expects amount in minor units (cents, pennies, etc.)
	amountInMinorUnits := int64(request.Amount * 100)

	// Build payment request
	paymentReq := map[string]any{
		"merchantId":    p.merchantID,
		"amount":        amountInMinorUnits,
		"currency":      request.Currency,
		"paymentMethod": "CREDIT_CARD",
		"description":   request.Description,
		"metadata":      request.MetaData,
		"initiatedBy":   "CUSTOMER",
	}

	// Add card details
	paymentReq["card"] = map[string]any{
		"holderName":  request.CardInfo.CardHolderName,
		"number":      request.CardInfo.CardNumber,
		"expireMonth": request.CardInfo.ExpireMonth,
		"expireYear":  request.CardInfo.ExpireYear,
		"cvv":         request.CardInfo.CVV,
	}

	// Add customer details
	paymentReq["customer"] = map[string]any{
		"id":        request.Customer.ID,
		"firstName": request.Customer.Name,
		"lastName":  request.Customer.Surname,
		"email":     request.Customer.Email,
		"phone":     request.Customer.PhoneNumber,
		"ipAddress": request.Customer.IPAddress,
	}

	// Add billing address
	if request.Customer.Address.City != "" {
		paymentReq["billingAddress"] = map[string]any{
			"country":    request.Customer.Address.Country,
			"city":       request.Customer.Address.City,
			"address":    request.Customer.Address.Address,
			"postalCode": request.Customer.Address.ZipCode,
		}
	}

	// Add 3D secure settings if needed
	if force3D || request.Use3D {
		secure3DSettings := map[string]any{
			"enabled": true,
		}

		// Build GoPay's own callback URL with user's original callback URL as parameter
		if request.CallbackURL != "" {
			gopayCallbackURL := fmt.Sprintf("%s/v1/callback/ozanpay?originalCallbackUrl=%s",
				p.gopayBaseURL,
				request.CallbackURL)
			secure3DSettings["returnUrl"] = gopayCallbackURL
		} else {
			// If no callback URL provided, use GoPay's callback without redirect
			secure3DSettings["returnUrl"] = fmt.Sprintf("%s/v1/callback/ozanpay", p.gopayBaseURL)
		}

		paymentReq["secure3d"] = secure3DSettings
	}

	// Add items if available
	if len(request.Items) > 0 {
		items := make([]map[string]any, len(request.Items))
		for i, item := range request.Items {
			items[i] = map[string]any{
				"name":        item.Name,
				"description": item.Description,
				"quantity":    item.Quantity,
				"price":       int64(item.Price * 100), // Convert to minor units
			}
		}
		paymentReq["items"] = items
	}

	return paymentReq
}

// Helper method to map OzanPay response to our common format
func (p *OzanPayProvider) mapToPaymentResponse(response map[string]any) (*provider.PaymentResponse, error) {
	paymentResp := &provider.PaymentResponse{
		ProviderResponse: response,
		SystemTime:       time.Now(),
	}

	// Extract payment ID
	if id, ok := response["id"].(string); ok {
		paymentResp.PaymentID = id
	}

	// Extract transaction ID if available
	if txnID, ok := response["transactionId"].(string); ok {
		paymentResp.TransactionID = txnID
	}

	// Extract amount and convert from minor units to standard units
	if amount, ok := response["amount"].(float64); ok {
		paymentResp.Amount = amount / 100
	}

	// Extract currency
	if currency, ok := response["currency"].(string); ok {
		paymentResp.Currency = currency
	}

	// Extract status
	if status, ok := response["status"].(string); ok {
		paymentResp.Success = (status == statusApproved || status == statusAuthorized)

		// Map OzanPay status to our common status
		switch status {
		case statusApproved, statusAuthorized:
			paymentResp.Status = provider.StatusSuccessful
			paymentResp.Message = "Payment successful"
		case statusPending:
			paymentResp.Status = provider.StatusPending
			paymentResp.Message = "Payment pending"
		case statusProcessing:
			paymentResp.Status = provider.StatusProcessing
			paymentResp.Message = "Payment processing"
		case statusDeclined, statusFailed:
			paymentResp.Status = provider.StatusFailed
			paymentResp.Message = "Payment failed"
		case statusCancelled:
			paymentResp.Status = provider.StatusCancelled
			paymentResp.Message = "Payment cancelled"
		case statusRefunded:
			paymentResp.Status = provider.StatusRefunded
			paymentResp.Message = "Payment refunded"
		default:
			paymentResp.Status = provider.StatusPending
			paymentResp.Message = "Payment status unknown"
		}
	}

	// Extract error message if available
	if errMsg, ok := response["errorMessage"].(string); ok && errMsg != "" {
		paymentResp.Success = false
		paymentResp.Message = errMsg
	}

	// Extract error code if available
	if errCode, ok := response["errorCode"].(string); ok && errCode != "" {
		paymentResp.Success = false
		paymentResp.ErrorCode = errCode
	}

	// Extract redirect URL for 3D secure payments
	if redirectURL, ok := response["redirectUrl"].(string); ok && redirectURL != "" {
		paymentResp.RedirectURL = redirectURL
		paymentResp.Status = provider.StatusPending
	}

	return paymentResp, nil
}

// Helper method to send requests to OzanPay API
func (p *OzanPayProvider) sendRequest(ctx context.Context, endpoint string, method string, requestData any) (map[string]any, error) {
	var body io.Reader
	var jsonData []byte
	var err error

	// Prepare request body if data is provided
	if requestData != nil {
		jsonData, err = json.Marshal(requestData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		body = strings.NewReader(string(jsonData))
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Set authentication headers
	timestamp := time.Now().UTC().Format(time.RFC3339)
	req.Header.Set("X-Ozan-Merchant", p.merchantID)
	req.Header.Set("X-Ozan-Timestamp", timestamp)

	// Calculate and set signature
	var dataToSign string
	if method == http.MethodGet {
		dataToSign = method + endpoint + timestamp
	} else {
		dataToSign = method + endpoint + timestamp + string(jsonData)
	}

	signature := p.generateSignature(dataToSign)
	req.Header.Set("X-Ozan-Signature", signature)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle non-success HTTP status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error: %d, response: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var responseData map[string]any
	if err := json.Unmarshal(respBody, &responseData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return responseData, nil
}

// Helper method to generate HMAC signature for OzanPay
func (p *OzanPayProvider) generateSignature(data string) string {
	h := hmac.New(sha256.New, []byte(p.secretKey))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
