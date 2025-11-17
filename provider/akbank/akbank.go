package akbank

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// API Endpoints
	apiSandboxPaymentAPIURL    = "https://apipre.akbank.com/api/v1/payment/virtualpos/transaction/process"
	apiProductionPaymentAPIURL = "https://api.akbank.com/api/v1/payment/virtualpos/transaction/process"

	// Transaction Codes
	txnCodeSale   = "1000" // Direct sale
	txnCode3D     = "3000" // 3D Secure sale
	txnCodeCancel = "2000" // Cancel/void
	txnCodeRefund = "2100" // Refund

	// Currency Codes
	currencyCodeTRY = 949

	// Default version
	apiVersion = "1.00"
)

// AkbankProvider implements the provider.PaymentProvider interface for Akbank
type AkbankProvider struct {
	merchantSafeId string
	terminalSafeId string
	secretKey      string
	baseURL        string
	gopayBaseURL   string
	isProduction   bool
	httpClient     *provider.ProviderHTTPClient
	logID          int64
}

// NewProvider creates a new Akbank payment provider
func NewProvider() provider.PaymentProvider {
	return &AkbankProvider{}
}

// GetRequiredConfig returns the configuration fields required for Akbank
func (p *AkbankProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "merchantSafeId",
			Required:    true,
			Type:        "string",
			Description: "Akbank Merchant Safe ID (provided by Akbank)",
			Example:     "2025100217305644994AAC1BF57EC29B",
			MinLength:   32,
			MaxLength:   50,
		},
		{
			Key:         "terminalSafeId",
			Required:    true,
			Type:        "string",
			Description: "Akbank Terminal Safe ID (provided by Akbank)",
			Example:     "202510021730564616275A2A52298FCF",
			MinLength:   32,
			MaxLength:   50,
		},
		{
			Key:         "secretKey",
			Required:    true,
			Type:        "string",
			Description: "Akbank Security Key (provided by Akbank)",
			Example:     "323032353130303231373330353634343135315f763737353873315f3176383731723331723572377367315f333572386733383132377431315f377267313532",
			MinLength:   50,
			MaxLength:   200,
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

// ValidateConfig validates the provided configuration against Akbank requirements
func (p *AkbankProvider) ValidateConfig(config map[string]string) error {
	requiredFields := p.GetRequiredConfig(config["environment"])
	return provider.ValidateConfigFields("akbank", config, requiredFields)
}

// Initialize sets up the Akbank payment provider with authentication credentials
func (p *AkbankProvider) Initialize(conf map[string]string) error {
	p.merchantSafeId = conf["merchantSafeId"]
	p.terminalSafeId = conf["terminalSafeId"]
	p.secretKey = conf["secretKey"]

	if p.merchantSafeId == "" || p.terminalSafeId == "" || p.secretKey == "" {
		return errors.New("akbank: merchantSafeId, terminalSafeId and secretKey are required")
	}

	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	p.isProduction = conf["environment"] == "production"
	if p.isProduction {
		p.baseURL = apiProductionPaymentAPIURL
	} else {
		p.baseURL = apiSandboxPaymentAPIURL
	}

	p.httpClient = provider.NewProviderHTTPClient(provider.CreateHTTPClientConfig(p.baseURL, p.isProduction))

	return nil
}

// GetInstallmentCount returns the installment count for a payment
func (p *AkbankProvider) GetInstallmentCount(ctx context.Context, request provider.InstallmentInquireRequest) (provider.InstallmentInquireResponse, error) {
	return provider.InstallmentInquireResponse{}, nil
}

// GetCommission returns the commission for a payment
func (p *AkbankProvider) GetCommission(ctx context.Context, request provider.CommissionRequest) (provider.CommissionResponse, error) {
	return provider.CommissionResponse{}, nil
}

// CreatePayment makes a non-3D payment request
func (p *AkbankProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("akbank: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *AkbankProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("akbank: invalid 3D payment request: %w", err)
	}

	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *AkbankProvider) Complete3DPayment(ctx context.Context, callbackState *provider.CallbackState, data map[string]string) (*provider.PaymentResponse, error) {
	// For Akbank, 3D completion is handled differently
	// This would need implementation based on Akbank's 3D secure flow
	return nil, errors.New("akbank: 3D completion not yet implemented")
}

// GetPaymentStatus retrieves the current status of a payment
func (p *AkbankProvider) GetPaymentStatus(ctx context.Context, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	// Akbank doesn't have a separate status inquiry endpoint in the PHP example
	// Status would need to be checked through their reporting API
	return nil, errors.New("akbank: payment status inquiry not yet implemented")
}

// CancelPayment cancels a payment
func (p *AkbankProvider) CancelPayment(ctx context.Context, request provider.CancelRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("akbank: paymentID is required")
	}

	// Get original order ID from log
	originalOrderId, err := provider.GetProviderRequestFromLogWithPaymentID("akbank", request.PaymentID, "orderId")
	if err != nil {
		return nil, fmt.Errorf("failed to get order ID: %s %w", request.PaymentID, err)
	}

	// Prepare cancel request
	akbankReq := p.buildBaseRequest(txnCodeCancel)
	akbankReq["order"] = map[string]any{
		"orderId": originalOrderId,
	}

	return p.sendPaymentRequest(ctx, akbankReq)
}

// RefundPayment issues a refund for a payment
func (p *AkbankProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("akbank: paymentID is required for refund")
	}

	if request.RefundAmount <= 0 {
		return nil, errors.New("akbank: refund amount must be greater than 0")
	}

	// Get original order ID from log
	originalOrderId, err := provider.GetProviderRequestFromLogWithPaymentID("akbank", request.PaymentID, "orderId")
	if err != nil {
		return nil, fmt.Errorf("failed to get order ID: %s %w", request.PaymentID, err)
	}

	// Prepare refund request
	akbankReq := p.buildBaseRequest(txnCodeRefund)
	akbankReq["order"] = map[string]any{
		"orderId": originalOrderId,
	}
	akbankReq["transaction"] = map[string]any{
		"amount":       int(request.RefundAmount * 100), // Convert to kuruş
		"currencyCode": currencyCodeTRY,
	}

	resp, err := p.sendRequest(ctx, akbankReq)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	success := resp["respCode"] == "0000" || resp["respCode"] == "00"

	refundResp := &provider.RefundResponse{
		Success:      success,
		PaymentID:    request.PaymentID,
		RefundAmount: request.RefundAmount,
		SystemTime:   &now,
		RawResponse:  resp,
	}

	if success {
		refundResp.Status = "success"
		refundResp.Message = "Refund successful"
		if refundID, ok := resp["transactionId"].(string); ok {
			refundResp.RefundID = refundID
		}
	} else {
		refundResp.Status = "failed"
		refundResp.ErrorCode = fmt.Sprintf("%v", resp["respCode"])
		refundResp.Message = fmt.Sprintf("%v", resp["respText"])
	}

	return refundResp, nil
}

// ValidateWebhook validates an incoming webhook notification
func (p *AkbankProvider) ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	// Akbank webhook validation would go here
	return true, data, nil
}

// validatePaymentRequest validates the payment request
func (p *AkbankProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
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

// processPayment handles the main payment processing logic
func (p *AkbankProvider) processPayment(ctx context.Context, request provider.PaymentRequest, is3D bool) (*provider.PaymentResponse, error) {
	// Determine transaction code
	txnCode := txnCodeSale
	if is3D {
		txnCode = txnCode3D
	}

	// Build payment request
	akbankReq := p.buildBaseRequest(txnCode)

	// Add card information
	expireDate := request.CardInfo.ExpireMonth + request.CardInfo.ExpireYear[len(request.CardInfo.ExpireYear)-2:]
	akbankReq["card"] = map[string]any{
		"cardNumber": request.CardInfo.CardNumber,
		"cvv2":       request.CardInfo.CVV,
		"expireDate": expireDate,
	}

	// Add reward information (empty for now)
	akbankReq["reward"] = map[string]any{
		"ccbRewardAmount": 0,
		"pcbRewardAmount": 0,
		"xcbRewardAmount": 0,
	}

	// Add transaction information
	installCount := 1
	if request.InstallmentCount > 1 {
		installCount = request.InstallmentCount
	}

	akbankReq["transaction"] = map[string]any{
		"amount":       int(request.Amount * 100), // Convert to kuruş
		"currencyCode": currencyCodeTRY,
		"motoInd":      0,
		"installCount": installCount,
	}

	// Generate order ID
	orderId := p.generateOrderId()
	akbankReq["order"] = map[string]any{
		"orderId": orderId,
	}

	// Add customer information
	customerIP := request.Customer.IPAddress
	if customerIP == "" {
		customerIP = request.ClientIP
	}
	if customerIP == "" {
		customerIP = "127.0.0.1"
	}

	akbankReq["customer"] = map[string]any{
		"emailAddress": request.Customer.Email,
		"ipAddress":    customerIP,
	}

	return p.sendPaymentRequest(ctx, akbankReq)
}

// buildBaseRequest builds the base request structure for Akbank
func (p *AkbankProvider) buildBaseRequest(txnCode string) map[string]any {
	return map[string]any{
		"version":         apiVersion,
		"txnCode":         txnCode,
		"requestDateTime": p.generateRequestDateTime(),
		"randomNumber":    p.generateRandomNumber(128),
		"terminal": map[string]any{
			"merchantSafeId": p.merchantSafeId,
			"terminalSafeId": p.terminalSafeId,
		},
	}
}

// sendPaymentRequest sends payment request and maps to PaymentResponse
func (p *AkbankProvider) sendPaymentRequest(ctx context.Context, requestData map[string]any) (*provider.PaymentResponse, error) {
	now := time.Now()
	resp, err := p.sendRequest(ctx, requestData)
	if err != nil {
		return nil, err
	}

	// Add provider request to client request
	_ = provider.AddProviderRequestToClientRequest("akbank", "providerRequest", requestData, p.logID)

	// Map Akbank response to common PaymentResponse
	paymentResp := &provider.PaymentResponse{
		SystemTime:       &now,
		ProviderResponse: resp,
	}

	// Check response code
	respCode, _ := resp["respCode"].(string)
	success := respCode == "0000" || respCode == "00"
	paymentResp.Success = success

	if success {
		paymentResp.Status = provider.StatusSuccessful
		paymentResp.Message = "Payment successful"

		// Extract payment ID
		if transactionId, ok := resp["transactionId"].(string); ok {
			paymentResp.PaymentID = transactionId
			paymentResp.TransactionID = transactionId
		}

		// Extract order ID
		if orderId, ok := resp["orderId"].(string); ok {
			paymentResp.OrderID = orderId
		}

		// Extract amount
		if order, ok := resp["order"].(map[string]any); ok {
			if orderId, ok := order["orderId"].(string); ok {
				paymentResp.OrderID = orderId
			}
		}
	} else {
		paymentResp.Status = provider.StatusFailed
		paymentResp.ErrorCode = respCode
		if respText, ok := resp["respText"].(string); ok {
			paymentResp.Message = respText
		} else {
			paymentResp.Message = "Payment failed"
		}
	}

	return paymentResp, nil
}

// sendRequest sends a request to Akbank API
func (p *AkbankProvider) sendRequest(ctx context.Context, requestData map[string]any) (map[string]any, error) {
	// Convert request data to JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Calculate auth hash
	authHash := p.generateAuthHash(string(jsonData))

	// Use new HTTP client
	httpReq := &provider.HTTPRequest{
		Method:   "POST",
		Endpoint: p.baseURL,
		Body:     requestData,
		Headers: map[string]string{
			"auth-hash":    authHash,
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
	}

	resp, err := p.httpClient.SendJSON(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Parse response as JSON
	var responseData map[string]any
	if err := p.httpClient.ParseJSONResponse(resp, &responseData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return responseData, nil
}

// generateAuthHash generates HMAC-SHA512 hash for authentication
func (p *AkbankProvider) generateAuthHash(data string) string {
	h := hmac.New(sha512.New, []byte(p.secretKey))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// generateRequestDateTime generates request datetime in Akbank format
func (p *AkbankProvider) generateRequestDateTime() string {
	now := time.Now()
	// Format: 2006-01-02T15:04:05.000
	return now.Format("2006-01-02T15:04:05.") + fmt.Sprintf("%03d", now.Nanosecond()/1000000)
}

// generateRandomNumber generates a random hex string of specified length
func (p *AkbankProvider) generateRandomNumber(length int) string {
	bytes := make([]byte, length/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// generateOrderId generates a unique order ID
func (p *AkbankProvider) generateOrderId() string {
	now := time.Now()
	// Format: YY + MONTH_NAME + DAY_NAME + seconds + instant_seconds
	// Example from PHP: 25NOVEMBERSUN5258
	year := now.Format("06")
	month := strings.ToUpper(now.Format("January"))
	day := strings.ToUpper(now.Format("Monday"))
	seconds := now.Format("05")
	instant := now.Format("0504")

	return year + month + day + seconds + instant
}
