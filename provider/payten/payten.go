package payten

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// API Endpoints
	apiSandboxURL    = "https://test.merchantsafeunipay.com/msu/api/v2"
	apiProductionURL = "https://merchantsafeunipay.com/msu/api/v2"

	// 3D Secure Gateway (Hosted Page)
	api3DGatewayURL = "https://merchantsafeunipay.com/msu/3dgate"

	// Actions
	actionSale             = "SALE"
	actionPreauth          = "PREAUTH"
	actionVoid             = "VOID"
	actionRefund           = "REFUND"
	actionSession          = "SESSIONTOKEN"
	actionQueryTransaction = "QUERYTRANSACTION"

	// Session Types
	sessionTypePayment = "PAYMENTSESSION"
	sessionTypeQuery   = "QUERYOPERATIONSESSION"

	// Currency Codes
	currencyCodeTRY = "TRY"
)

// PaytenProvider implements the provider.PaymentProvider interface for Payten
type PaytenProvider struct {
	merchant         string
	merchantUser     string
	merchantPassword string
	secretKey        string
	baseURL          string
	threeDGatewayURL string
	gopayBaseURL     string
	isProduction     bool
	httpClient       *provider.ProviderHTTPClient
	logID            int64
}

// NewProvider creates a new Payten payment provider
func NewProvider() provider.PaymentProvider {
	return &PaytenProvider{}
}

// GetRequiredConfig returns the configuration fields required for Payten
func (p *PaytenProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "merchant",
			Required:    true,
			Type:        "string",
			Description: "Payten Merchant ID",
			Example:     "100200127",
			MinLength:   5,
			MaxLength:   50,
		},
		{
			Key:         "merchantUser",
			Required:    true,
			Type:        "string",
			Description: "Payten Merchant User",
			Example:     "merchant_user",
			MinLength:   3,
			MaxLength:   100,
		},
		{
			Key:         "merchantPassword",
			Required:    true,
			Type:        "string",
			Description: "Payten Merchant Password",
			Example:     "merchant_password",
			MinLength:   5,
			MaxLength:   100,
		},
		{
			Key:         "secretKey",
			Required:    true,
			Type:        "string",
			Description: "Payten Secret Key (Store Key) for hash calculation",
			Example:     "TEST1234",
			MinLength:   5,
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

// ValidateConfig validates the provided configuration against Payten requirements
func (p *PaytenProvider) ValidateConfig(config map[string]string) error {
	requiredFields := p.GetRequiredConfig(config["environment"])
	return provider.ValidateConfigFields("payten", config, requiredFields)
}

// Initialize sets up the Payten payment provider with authentication credentials
func (p *PaytenProvider) Initialize(conf map[string]string) error {
	p.merchant = conf["merchant"]
	p.merchantUser = conf["merchantUser"]
	p.merchantPassword = conf["merchantPassword"]
	p.secretKey = conf["secretKey"]

	if p.merchant == "" || p.merchantUser == "" || p.merchantPassword == "" || p.secretKey == "" {
		return errors.New("payten: merchant, merchantUser, merchantPassword and secretKey are required")
	}

	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	p.isProduction = conf["environment"] == "production"
	p.baseURL = apiSandboxURL
	if p.isProduction {
		p.baseURL = apiProductionURL
	}

	p.threeDGatewayURL = api3DGatewayURL

	p.httpClient = provider.NewProviderHTTPClient(provider.CreateHTTPClientConfig(p.baseURL, p.isProduction))

	return nil
}

// GetInstallmentCount returns the installment count for a payment
func (p *PaytenProvider) GetInstallmentCount(ctx context.Context, request provider.InstallmentInquireRequest) (provider.InstallmentInquireResponse, error) {
	return provider.InstallmentInquireResponse{}, nil
}

// GetCommission returns the commission for a payment
func (p *PaytenProvider) GetCommission(ctx context.Context, request provider.CommissionRequest) (provider.CommissionResponse, error) {
	return provider.CommissionResponse{}, nil
}

// CreatePayment makes a non-3D payment request
// Note: Payten uses Hosted Page approach, so this creates a session token
func (p *PaytenProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("payten: invalid payment request: %w", err)
	}

	// Payten uses Hosted Page (HP) approach with SESSIONTOKEN
	return p.processHostedPagePayment(ctx, request)
}

// Create3DPayment starts a 3D secure payment process
// Note: Payten uses Hosted Page approach, so this creates a session token
func (p *PaytenProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("payten: invalid 3D payment request: %w", err)
	}

	// Payten uses Hosted Page (HP) approach with SESSIONTOKEN
	return p.processHostedPagePayment(ctx, request)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *PaytenProvider) Complete3DPayment(ctx context.Context, callbackState *provider.CallbackState, data map[string]string) (*provider.PaymentResponse, error) {
	p.logID = callbackState.LogID

	// Payten Hosted Page returns sessionToken in callback
	sessionToken, ok := data["sessionToken"]
	if !ok {
		// Fallback: try alternative field names
		sessionToken, _ = data["session_token"]
		if sessionToken == "" {
			sessionToken, _ = data["SESSIONTOKEN"]
		}
	}

	if sessionToken == "" {
		return nil, errors.New("payten: missing sessionToken in callback data")
	}

	// Query transaction using session token
	transactionResp, err := p.queryTransaction(ctx, sessionToken)
	if err != nil {
		return nil, fmt.Errorf("payten: failed to query transaction: %w", err)
	}

	// Extract transaction details
	transactionList, ok := transactionResp["transactionList"].([]any)
	if !ok || len(transactionList) == 0 {
		return nil, errors.New("payten: no transaction found in response")
	}

	transaction, ok := transactionList[0].(map[string]any)
	if !ok {
		return nil, errors.New("payten: invalid transaction format")
	}

	transactionId, _ := transaction["transactionId"].(string)
	responseCode, _ := transactionResp["responseCode"].(string)
	success := responseCode == "00"

	now := time.Now()
	response := &provider.PaymentResponse{
		Success:          success,
		PaymentID:        callbackState.PaymentID,
		TransactionID:    transactionId,
		Amount:           callbackState.Amount,
		Currency:         callbackState.Currency,
		SystemTime:       &now,
		ProviderResponse: transactionResp,
		RedirectURL:      callbackState.OriginalCallback,
	}

	if success {
		response.Status = provider.StatusSuccessful
		response.Message = "Payment completed successfully"
		if msg, ok := transactionResp["responseMsg"].(string); ok {
			response.Message = msg
		}
	} else {
		response.Status = provider.StatusFailed
		response.ErrorCode = responseCode
		if msg, ok := transactionResp["errorMsg"].(string); ok {
			response.Message = msg
		} else {
			response.Message = "Payment failed"
		}
	}

	return response, nil
}

// GetPaymentStatus retrieves the current status of a payment
func (p *PaytenProvider) GetPaymentStatus(ctx context.Context, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	// Get session token from log
	sessionToken, err := provider.GetProviderRequestFromLogWithPaymentID("payten", request.PaymentID, "sessionToken")
	if err != nil {
		return nil, fmt.Errorf("failed to get session token: %s %w", request.PaymentID, err)
	}

	// Query transaction
	transactionResp, err := p.queryTransaction(ctx, sessionToken)
	if err != nil {
		return nil, fmt.Errorf("payten: failed to query transaction: %w", err)
	}

	return p.mapToPaymentResponse(transactionResp, request.PaymentID)
}

// CancelPayment cancels a payment
func (p *PaytenProvider) CancelPayment(ctx context.Context, request provider.CancelRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("payten: paymentID is required")
	}

	// Get original transaction details from log
	originalOrderId, err := provider.GetProviderRequestFromLogWithPaymentID("payten", request.PaymentID, "MERCHANTPAYMENTID")
	if err != nil {
		return nil, fmt.Errorf("failed to get order ID: %s %w", request.PaymentID, err)
	}

	// Build void request
	formData := p.buildVoidRequest(originalOrderId)

	// Send request
	resp, err := p.sendMultipartRequest(ctx, formData)
	if err != nil {
		return nil, err
	}

	return p.mapToPaymentResponse(resp, request.PaymentID)
}

// RefundPayment issues a refund for a payment
func (p *PaytenProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("payten: paymentID is required for refund")
	}

	if request.RefundAmount <= 0 {
		return nil, errors.New("payten: refund amount must be greater than 0")
	}

	// Get original transaction details from log
	originalOrderId, err := provider.GetProviderRequestFromLogWithPaymentID("payten", request.PaymentID, "MERCHANTPAYMENTID")
	if err != nil {
		return nil, fmt.Errorf("failed to get order ID: %s %w", request.PaymentID, err)
	}

	// Build refund request
	formData := p.buildRefundRequest(originalOrderId, request.RefundAmount)

	// Send request
	resp, err := p.sendMultipartRequest(ctx, formData)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	responseCode, _ := resp["responseCode"].(string)
	success := responseCode == "00"

	refundResp := &provider.RefundResponse{
		Success:      success,
		PaymentID:    request.PaymentID,
		RefundAmount: request.RefundAmount,
		SystemTime:   &now,
		RawResponse:  resp,
	}

	if success {
		refundResp.Status = "success"
		if msg, ok := resp["responseMsg"].(string); ok {
			refundResp.Message = msg
		}
		if transactionId, ok := resp["transactionId"].(string); ok {
			refundResp.RefundID = transactionId
		}
	} else {
		refundResp.Status = "failed"
		refundResp.ErrorCode = responseCode
		if msg, ok := resp["responseMsg"].(string); ok {
			refundResp.Message = msg
		}
	}

	return refundResp, nil
}

// ValidateWebhook validates an incoming webhook notification
func (p *PaytenProvider) ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	// Payten webhook validation would go here
	return true, data, nil
}

// validatePaymentRequest validates the payment request
func (p *PaytenProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
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

// processHostedPagePayment handles Hosted Page payment using SESSIONTOKEN
func (p *PaytenProvider) processHostedPagePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	// Generate merchant payment ID
	merchantPaymentID := request.ID
	if merchantPaymentID == "" {
		merchantPaymentID = fmt.Sprintf("PAYTEN-%d", time.Now().UnixNano())
	}

	// Create callback state
	state := provider.CallbackState{
		TenantID:         int(request.TenantID),
		PaymentID:        merchantPaymentID,
		OriginalCallback: request.CallbackURL,
		Amount:           request.Amount,
		Currency:         request.Currency,
		LogID:            p.logID,
		Provider:         "payten",
		Environment:      request.Environment,
		Timestamp:        time.Now(),
		ClientIP:         request.ClientIP,
		Installment:      request.InstallmentCount,
		SessionID:        request.SessionID,
	}

	// Create short callback URL
	gopayCallbackURL, err := provider.CreateShortCallbackURL(ctx, p.gopayBaseURL, "payten", state)
	if err != nil {
		return nil, fmt.Errorf("failed to create callback URL: %w", err)
	}

	// Build SESSIONTOKEN request
	formData := p.buildSessionTokenRequest(request, merchantPaymentID, gopayCallbackURL)

	// Send multipart form request (no hash needed for SESSIONTOKEN)
	resp, err := p.sendMultipartRequest(ctx, formData)
	if err != nil {
		return nil, err
	}

	// Store request for logging
	if reqMap, err := provider.StructToMap(formData); err == nil {
		_ = provider.AddProviderRequestToClientRequest("payten", "sessionTokenRequest", reqMap, p.logID)
	}

	// Check response
	responseCode, _ := resp["responseCode"].(string)
	if responseCode != "00" {
		errorMsg, _ := resp["errorMsg"].(string)
		return nil, fmt.Errorf("payten: failed to create session token: %s", errorMsg)
	}

	// Extract session token
	sessionToken, ok := resp["sessionToken"].(string)
	if !ok {
		return nil, errors.New("payten: sessionToken not found in response")
	}

	// Store session token for later use
	if reqMap, err := provider.StructToMap(map[string]string{"sessionToken": sessionToken}); err == nil {
		_ = provider.AddProviderRequestToClientRequest("payten", "sessionToken", reqMap, p.logID)
	}

	// Generate redirect URL to Payten Hosted Page
	redirectURL := fmt.Sprintf("%s?sessionToken=%s", p.threeDGatewayURL, sessionToken)

	now := time.Now()
	return &provider.PaymentResponse{
		Success:          true,
		Status:           provider.StatusPending,
		PaymentID:        merchantPaymentID,
		Amount:           request.Amount,
		Currency:         request.Currency,
		RedirectURL:      redirectURL,
		Message:          "Redirect to Payten Hosted Page",
		SystemTime:       &now,
		ProviderResponse: resp,
	}, nil
}

// queryTransaction queries transaction status using session token
func (p *PaytenProvider) queryTransaction(ctx context.Context, sessionToken string) (map[string]any, error) {
	formData := map[string]string{
		"ACTION":       actionQueryTransaction,
		"SESSIONTOKEN": sessionToken,
	}

	resp, err := p.sendMultipartRequest(ctx, formData)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// buildSessionTokenRequest builds form parameters for SESSIONTOKEN action
func (p *PaytenProvider) buildSessionTokenRequest(request provider.PaymentRequest, merchantPaymentID, returnURL string) map[string]string {
	// Format amount (with 2 decimal places)
	amountStr := fmt.Sprintf("%.2f", request.Amount)

	// Build customer ID (use customer email or ID)
	customerID := request.Customer.ID
	if customerID == "" {
		customerID = request.Customer.Email
	}
	if customerID == "" {
		customerID = fmt.Sprintf("CUSTOMER-%d", time.Now().UnixNano())
	}

	params := map[string]string{
		"ACTION":            actionSession,
		"MERCHANT":          p.merchant,
		"MERCHANTUSER":      p.merchantUser,
		"MERCHANTPASSWORD":  p.merchantPassword,
		"CUSTOMER":          customerID,
		"SESSIONTYPE":       sessionTypePayment,
		"MERCHANTPAYMENTID": merchantPaymentID,
		"AMOUNT":            amountStr,
		"CURRENCY":          currencyCodeTRY,
		"RETURNURL":         returnURL,
		"EXTRA[SaveCard]":   "NO",
	}

	// Add customer email if available
	if request.Customer.Email != "" {
		params["CUSTOMEREMAIL"] = request.Customer.Email
	}

	// Add customer name if available
	if request.Customer.Name != "" || request.Customer.Surname != "" {
		customerName := strings.TrimSpace(fmt.Sprintf("%s %s", request.Customer.Name, request.Customer.Surname))
		if customerName != "" {
			params["CUSTOMERNAME"] = customerName
		}
	}

	// Add customer phone if available
	if request.Customer.PhoneNumber != "" {
		params["CUSTOMERPHONE"] = request.Customer.PhoneNumber
	}

	return params
}

// buildVoidRequest builds form parameters for VOID action
func (p *PaytenProvider) buildVoidRequest(merchantPaymentID string) map[string]string {
	return map[string]string{
		"ACTION":            actionVoid,
		"MERCHANT":          p.merchant,
		"MERCHANTUSER":      p.merchantUser,
		"MERCHANTPASSWORD":  p.merchantPassword,
		"MERCHANTPAYMENTID": merchantPaymentID,
	}
}

// buildRefundRequest builds form parameters for REFUND action
func (p *PaytenProvider) buildRefundRequest(merchantPaymentID string, refundAmount float64) map[string]string {
	amountStr := fmt.Sprintf("%.2f", refundAmount)
	return map[string]string{
		"ACTION":            actionRefund,
		"MERCHANT":          p.merchant,
		"MERCHANTUSER":      p.merchantUser,
		"MERCHANTPASSWORD":  p.merchantPassword,
		"MERCHANTPAYMENTID": merchantPaymentID,
		"AMOUNT":            amountStr,
		"CURRENCY":          currencyCodeTRY,
	}
}

// calculateHash calculates SHA512 hash for Payten form (ver3 format)
func (p *PaytenProvider) calculateHash(params map[string]string) (string, error) {
	// Get sorted parameter keys (case-insensitive)
	keys := make([]string, 0, len(params))
	for k := range params {
		lowerKey := strings.ToLower(k)
		// Skip hash and encoding parameters
		if lowerKey != "hash" && lowerKey != "encoding" {
			keys = append(keys, k)
		}
	}

	// Sort keys case-insensitively
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToLower(keys[i]) < strings.ToLower(keys[j])
	})

	// Build hash string
	var hashVal strings.Builder
	for _, key := range keys {
		value := params[key]
		// Escape | and \ characters
		escapedValue := strings.ReplaceAll(value, "\\", "\\\\")
		escapedValue = strings.ReplaceAll(escapedValue, "|", "\\|")
		hashVal.WriteString(escapedValue)
		hashVal.WriteString("|")
	}

	// Add secret key (also escaped)
	escapedSecretKey := strings.ReplaceAll(p.secretKey, "\\", "\\\\")
	escapedSecretKey = strings.ReplaceAll(escapedSecretKey, "|", "\\|")
	hashVal.WriteString(escapedSecretKey)

	// Calculate SHA512 hash
	hashBytes := sha512.Sum512([]byte(hashVal.String()))

	// Convert to hex string (like PHP hash('sha512', ...))
	hexHash := hex.EncodeToString(hashBytes[:])

	// Convert hex string back to bytes (like PHP pack('H*', ...))
	hashBytesFromHex := make([]byte, len(hexHash)/2)
	_, err := hex.Decode(hashBytesFromHex, []byte(hexHash))
	if err != nil {
		return "", fmt.Errorf("failed to decode hex hash: %w", err)
	}

	// Base64 encode (like PHP base64_encode(...))
	hashBase64 := base64.StdEncoding.EncodeToString(hashBytesFromHex)

	return hashBase64, nil
}

// sendMultipartRequest sends a multipart/form-data request to Payten API
func (p *PaytenProvider) sendMultipartRequest(ctx context.Context, formData map[string]string) (map[string]any, error) {
	// Payten uses multipart/form-data (not application/x-www-form-urlencoded)
	// Set Content-Type header to indicate multipart
	httpReq := &provider.HTTPRequest{
		Method:   "POST",
		Endpoint: p.baseURL,
		FormData: formData,
		Headers: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "multipart/form-data",
		},
	}

	resp, err := p.httpClient.SendForm(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Parse JSON response
	var responseData map[string]any
	if err := p.httpClient.ParseJSONResponse(resp, &responseData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return responseData, nil
}

// mapToPaymentResponse maps Payten response to common PaymentResponse
func (p *PaytenProvider) mapToPaymentResponse(resp map[string]any, paymentID string) (*provider.PaymentResponse, error) {
	now := time.Now()
	paymentResp := &provider.PaymentResponse{
		SystemTime:       &now,
		ProviderResponse: resp,
		PaymentID:        paymentID,
	}

	// Check response code
	responseCode, _ := resp["responseCode"].(string)
	success := responseCode == "00"
	paymentResp.Success = success

	if success {
		paymentResp.Status = provider.StatusSuccessful
		paymentResp.Message = "Payment successful"
		if msg, ok := resp["responseMsg"].(string); ok {
			paymentResp.Message = msg
		}

		// Extract transaction ID
		if transactionId, ok := resp["transactionId"].(string); ok {
			paymentResp.TransactionID = transactionId
		}
	} else {
		paymentResp.Status = provider.StatusFailed
		paymentResp.ErrorCode = responseCode
		if msg, ok := resp["responseMsg"].(string); ok {
			paymentResp.Message = msg
		} else {
			paymentResp.Message = "Payment failed"
		}
	}

	return paymentResp, nil
}

// generate3DSecureHTML generates HTML form for 3D Secure authentication
func (p *PaytenProvider) generate3DSecureHTML(params map[string]string) string {
	var formFields strings.Builder
	for key, value := range params {
		formFields.WriteString(fmt.Sprintf(`<input type="hidden" name="%s" value="%s" />`, key, value))
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>3D Secure Authentication</title>
	<meta charset="utf-8">
	<meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
</head>
<body onload="document.threeDForm.submit();">
	<div style="text-align: center; margin-top: 50px;">
		<p>Ödeme işleminiz 3D güvenlik sayfasına yönlendiriliyor...</p>
		<p>Payment is being redirected to 3D secure page...</p>
	</div>
	<form name="threeDForm" method="POST" action="%s">
		%s
	</form>
</body>
</html>`, p.threeDGatewayURL, formFields.String())

	return html
}
