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
func (p *PaytenProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("payten: invalid payment request: %w", err)
	}

	return p.processDirectPostNon3D(ctx, request)
}

// Create3DPayment starts a 3D secure payment process
func (p *PaytenProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("payten: invalid 3D payment request: %w", err)
	}

	return p.processDirectPost3D(ctx, request)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *PaytenProvider) Complete3DPayment(ctx context.Context, callbackState *provider.CallbackState, data map[string]string) (*provider.PaymentResponse, error) {
	p.logID = callbackState.LogID

	// Validate hash from callback
	receivedHash, ok := data["HASH"]
	if !ok {
		return nil, errors.New("payten: missing HASH in callback data")
	}

	// Calculate expected hash
	expectedHash, err := p.calculateHash(data)
	if err != nil {
		return nil, fmt.Errorf("payten: failed to calculate hash: %w", err)
	}

	// Verify hash
	if receivedHash != expectedHash {
		return nil, errors.New("payten: invalid hash in callback data")
	}

	// Extract payment status from callback
	mdStatus, _ := data["mdStatus"]
	responseCode, _ := data["Response"]
	errorMsg, _ := data["ErrMsg"]
	transactionId, _ := data["TransId"]
	orderId, _ := data["oid"]

	// Determine success based on mdStatus and Response
	// mdStatus: 1,2,3,4 = success, others = failure
	// Response: "Approved" = success
	success := (mdStatus == "1" || mdStatus == "2" || mdStatus == "3" || mdStatus == "4") && responseCode == "Approved"

	now := time.Now()
	response := &provider.PaymentResponse{
		Success:          success,
		PaymentID:        orderId,
		TransactionID:    transactionId,
		Amount:           callbackState.Amount,
		Currency:         callbackState.Currency,
		SystemTime:       &now,
		ProviderResponse: data,
		RedirectURL:      callbackState.OriginalCallback,
	}

	if success {
		response.Status = provider.StatusSuccessful
		response.Message = "3D payment completed successfully"
	} else {
		response.Status = provider.StatusFailed
		response.ErrorCode = responseCode
		if errorMsg != "" {
			response.Message = errorMsg
		} else {
			response.Message = "3D payment failed"
		}
	}

	return response, nil
}

// GetPaymentStatus retrieves the current status of a payment
func (p *PaytenProvider) GetPaymentStatus(ctx context.Context, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	// Payten doesn't have a separate status inquiry endpoint for Direct Post
	// Status would need to be checked through their reporting API
	return nil, errors.New("payten: payment status inquiry not yet implemented")
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

// processDirectPostNon3D handles Direct Post Non-3D payment
func (p *PaytenProvider) processDirectPostNon3D(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	// Generate merchant payment ID
	merchantPaymentID := request.ID
	if merchantPaymentID == "" {
		merchantPaymentID = fmt.Sprintf("PAYTEN-%d", time.Now().UnixNano())
	}

	// Build form data with card information
	formData := p.buildSaleRequest(request, merchantPaymentID, false)

	// Send request
	resp, err := p.sendFormRequest(ctx, formData)
	if err != nil {
		return nil, err
	}

	// Store request for logging
	if reqMap, err := provider.StructToMap(formData); err == nil {
		_ = provider.AddProviderRequestToClientRequest("payten", "providerRequest", reqMap, p.logID)
	}

	return p.mapToPaymentResponse(resp, merchantPaymentID)
}

// processDirectPost3D handles Direct Post 3D payment
func (p *PaytenProvider) processDirectPost3D(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
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

	// Build 3D form parameters with card information
	formParams := p.buildSaleRequest(request, merchantPaymentID, true)
	formParams["RETURNURL"] = gopayCallbackURL
	formParams["FAILURL"] = gopayCallbackURL

	// Calculate hash for 3D form
	hash, err := p.calculateHash(formParams)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate 3D hash: %w", err)
	}
	formParams["HASH"] = hash

	// Generate HTML form
	html := p.generate3DSecureHTML(formParams)

	// Store form params for logging
	if reqMap, err := provider.StructToMap(formParams); err == nil {
		_ = provider.AddProviderRequestToClientRequest("payten", "3dFormParams", reqMap, p.logID)
	}

	now := time.Now()
	return &provider.PaymentResponse{
		Success:          true,
		Status:           provider.StatusPending,
		PaymentID:        merchantPaymentID,
		Amount:           request.Amount,
		Currency:         request.Currency,
		HTML:             html,
		Message:          "3D Secure authentication required",
		SystemTime:       &now,
		ProviderResponse: formParams,
	}, nil
}

// buildSaleRequest builds form parameters for SALE action (Direct Post)
func (p *PaytenProvider) buildSaleRequest(request provider.PaymentRequest, merchantPaymentID string, is3D bool) map[string]string {
	// Format amount (with 2 decimal places)
	amountStr := fmt.Sprintf("%.2f", request.Amount)

	// Get year (last 2 digits)
	expYear := request.CardInfo.ExpireYear
	if len(expYear) > 2 {
		expYear = expYear[len(expYear)-2:]
	}

	// Build customer name
	customerName := strings.TrimSpace(request.CardInfo.CardHolderName)
	if customerName == "" {
		customerName = fmt.Sprintf("%s %s", request.Customer.Name, request.Customer.Surname)
	}

	params := map[string]string{
		"ACTION":                          actionSale,
		"MERCHANT":                        p.merchant,
		"MERCHANTUSER":                    p.merchantUser,
		"MERCHANTPASSWORD":                p.merchantPassword,
		"MERCHANTPAYMENTID":               merchantPaymentID,
		"AMOUNT":                          amountStr,
		"CURRENCY":                        currencyCodeTRY,
		"CUSTOMEREMAIL":                   request.Customer.Email,
		"CUSTOMERNAME":                    customerName,
		"CUSTOMERPHONE":                   request.Customer.PhoneNumber,
		"PAN":                             request.CardInfo.CardNumber,
		"CVV2":                            request.CardInfo.CVV,
		"ECOM_PAYMENT_CARD_EXPDATE_MONTH": request.CardInfo.ExpireMonth,
		"ECOM_PAYMENT_CARD_EXPDATE_YEAR":  expYear,
		"LANG":                            "tr",
		"STORE_TYPE":                      "3D_PAY",
		"HASHALGORITHM":                   "ver3",
	}

	// Add installment if specified
	if request.InstallmentCount > 1 {
		params["SETINSTALLMENT"] = fmt.Sprintf("%d", request.InstallmentCount)
	}

	// Add customer IP if available
	if request.ClientIP != "" {
		params["CUSTOMERIPADDRESS"] = request.ClientIP
	}

	return params
}

// sendFormRequest sends a form request to Payten API (with hash calculation)
func (p *PaytenProvider) sendFormRequest(ctx context.Context, formData map[string]string) (map[string]any, error) {
	// Calculate hash
	hash, err := p.calculateHash(formData)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}
	formData["HASH"] = hash

	// Send POST request (application/x-www-form-urlencoded for Direct Post)
	httpReq := &provider.HTTPRequest{
		Method:   "POST",
		Endpoint: p.baseURL,
		Body:     formData,
		Headers: map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
			"Accept":       "application/json",
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
