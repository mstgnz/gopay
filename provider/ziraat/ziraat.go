package ziraat

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
	"sort"
	"strconv"
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

// ZiraatProvider implements the provider.PaymentProvider interface for Ziraat
type ZiraatProvider struct {
	merchantSafeId   string
	terminalSafeId   string
	secretKey        string
	baseURL          string
	threeDGatewayURL string
	gopayBaseURL     string
	isProduction     bool
	httpClient       *provider.ProviderHTTPClient
	logID            int64
}

// NewProvider creates a new Ziraat payment provider
func NewProvider() provider.PaymentProvider {
	return &ZiraatProvider{}
}

// GetRequiredConfig returns the configuration fields required for Ziraat
func (p *ZiraatProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "merchantSafeId",
			Required:    true,
			Type:        "string",
			Description: "Ziraat Merchant Safe ID (provided by Ziraat)",
			Example:     "2025100217305644994AAC1BF57EC29B",
			MinLength:   32,
			MaxLength:   50,
		},
		{
			Key:         "terminalSafeId",
			Required:    true,
			Type:        "string",
			Description: "Ziraat Terminal Safe ID (provided by Ziraat)",
			Example:     "202510021730564616275A2A52298FCF",
			MinLength:   32,
			MaxLength:   50,
		},
		{
			Key:         "secretKey",
			Required:    true,
			Type:        "string",
			Description: "Ziraat Security Key (provided by Ziraat)",
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

// ValidateConfig validates the provided configuration against Ziraat requirements
func (p *ZiraatProvider) ValidateConfig(config map[string]string) error {
	requiredFields := p.GetRequiredConfig(config["environment"])
	return provider.ValidateConfigFields("ziraat", config, requiredFields)
}

// Initialize sets up the Ziraat payment provider with authentication credentials
func (p *ZiraatProvider) Initialize(conf map[string]string) error {
	p.merchantSafeId = conf["merchantSafeId"]
	p.terminalSafeId = conf["terminalSafeId"]
	p.secretKey = conf["secretKey"]

	if p.merchantSafeId == "" || p.terminalSafeId == "" || p.secretKey == "" {
		return errors.New("ziraat: merchantSafeId, terminalSafeId and secretKey are required")
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
func (p *ZiraatProvider) GetInstallmentCount(ctx context.Context, request provider.InstallmentInquireRequest) (provider.InstallmentInquireResponse, error) {
	return provider.InstallmentInquireResponse{}, nil
}

// GetCommission returns the commission for a payment
func (p *ZiraatProvider) GetCommission(ctx context.Context, request provider.CommissionRequest) (provider.CommissionResponse, error) {
	return provider.CommissionResponse{}, nil
}

// CreatePayment makes a non-3D payment request
func (p *ZiraatProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("ziraat: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *ZiraatProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("ziraat: invalid 3D payment request: %w", err)
	}

	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *ZiraatProvider) Complete3DPayment(ctx context.Context, callbackState *provider.CallbackState, data map[string]string) (*provider.PaymentResponse, error) {
	p.logID = callbackState.LogID

	// Validate hash from callback
	receivedHash, ok := data["HASH"]
	if !ok {
		return nil, errors.New("ziraat: missing HASH in callback data")
	}

	// Calculate expected hash
	expectedHash, err := p.calculate3DHash(data)
	if err != nil {
		return nil, fmt.Errorf("ziraat: failed to calculate hash: %w", err)
	}

	// Verify hash
	if receivedHash != expectedHash {
		return nil, errors.New("ziraat: invalid hash in callback data")
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
func (p *ZiraatProvider) GetPaymentStatus(ctx context.Context, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	// Ziraat doesn't have a separate status inquiry endpoint in the PHP example
	// Status would need to be checked through their reporting API
	return nil, errors.New("ziraat: payment status inquiry not yet implemented")
}

// CancelPayment cancels a payment
func (p *ZiraatProvider) CancelPayment(ctx context.Context, request provider.CancelRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("ziraat: paymentID is required")
	}

	// Get original order ID from log
	originalOrderId, err := provider.GetProviderRequestFromLogWithPaymentID("ziraat", request.PaymentID, "orderId")
	if err != nil {
		return nil, fmt.Errorf("failed to get order ID: %s %w", request.PaymentID, err)
	}

	// Prepare cancel request
	ziraatReq := p.buildBaseRequest(txnCodeCancel)
	ziraatReq["order"] = map[string]any{
		"orderId": originalOrderId,
	}

	return p.sendPaymentRequest(ctx, ziraatReq)
}

// RefundPayment issues a refund for a payment
func (p *ZiraatProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("ziraat: paymentID is required for refund")
	}

	if request.RefundAmount <= 0 {
		return nil, errors.New("ziraat: refund amount must be greater than 0")
	}

	// Get original order ID from log
	originalOrderId, err := provider.GetProviderRequestFromLogWithPaymentID("ziraat", request.PaymentID, "orderId")
	if err != nil {
		return nil, fmt.Errorf("failed to get order ID: %s %w", request.PaymentID, err)
	}

	// Prepare refund request
	ziraatReq := p.buildBaseRequest(txnCodeRefund)
	ziraatReq["order"] = map[string]any{
		"orderId": originalOrderId,
	}
	ziraatReq["transaction"] = map[string]any{
		"amount":       int(request.RefundAmount * 100), // Convert to kuruş
		"currencyCode": currencyCodeTRY,
	}

	resp, err := p.sendRequest(ctx, ziraatReq)
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
func (p *ZiraatProvider) ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	// Ziraat webhook validation would go here
	return true, data, nil
}

// validatePaymentRequest validates the payment request
func (p *ZiraatProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
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
func (p *ZiraatProvider) processPayment(ctx context.Context, request provider.PaymentRequest, is3D bool) (*provider.PaymentResponse, error) {
	if is3D {
		return p.process3DPayment(ctx, request)
	}

	// Non-3D payment flow
	// Determine transaction code
	txnCode := txnCodeSale

	// Build payment request
	ziraatReq := p.buildBaseRequest(txnCode)

	// Add card information
	expireDate := request.CardInfo.ExpireMonth + request.CardInfo.ExpireYear[len(request.CardInfo.ExpireYear)-2:]
	ziraatReq["card"] = map[string]any{
		"cardNumber": request.CardInfo.CardNumber,
		"cvv2":       request.CardInfo.CVV,
		"expireDate": expireDate,
	}

	// Add reward information (empty for now)
	ziraatReq["reward"] = map[string]any{
		"ccbRewardAmount": 0,
		"pcbRewardAmount": 0,
		"xcbRewardAmount": 0,
	}

	// Add transaction information
	installCount := 1
	if request.InstallmentCount > 1 {
		installCount = request.InstallmentCount
	}

	ziraatReq["transaction"] = map[string]any{
		"amount":       int(request.Amount * 100), // Convert to kuruş
		"currencyCode": currencyCodeTRY,
		"motoInd":      0,
		"installCount": installCount,
	}

	// Generate order ID
	orderId := p.generateOrderId()
	ziraatReq["order"] = map[string]any{
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

	ziraatReq["customer"] = map[string]any{
		"emailAddress": request.Customer.Email,
		"ipAddress":    customerIP,
	}

	return p.sendPaymentRequest(ctx, ziraatReq)
}

// process3DPayment handles 3D Secure payment flow using Payten form-based approach
func (p *ZiraatProvider) process3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	// Generate order ID
	orderId := p.generateOrderId()

	// Create callback state
	state := provider.CallbackState{
		TenantID:         int(request.TenantID),
		PaymentID:        orderId,
		OriginalCallback: request.CallbackURL,
		Amount:           request.Amount,
		Currency:         request.Currency,
		LogID:            p.logID,
		Provider:         "ziraat",
		Environment:      request.Environment,
		Timestamp:        time.Now(),
		ClientIP:         request.ClientIP,
		Installment:      request.InstallmentCount,
		SessionID:        request.SessionID,
	}

	// Create short callback URL
	gopayCallbackURL, err := provider.CreateShortCallbackURL(ctx, p.gopayBaseURL, "ziraat", state)
	if err != nil {
		return nil, fmt.Errorf("failed to create callback URL: %w", err)
	}

	// Prepare 3D form parameters
	formParams := p.build3DFormParams(request, orderId, gopayCallbackURL)

	// Calculate hash for 3D form
	hash, err := p.calculate3DHash(formParams)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate 3D hash: %w", err)
	}
	formParams["HASH"] = hash

	// Generate HTML form
	html := p.generate3DSecureHTML(formParams)

	// Store form params for logging
	if reqMap, err := provider.StructToMap(formParams); err == nil {
		_ = provider.AddProviderRequestToClientRequest("ziraat", "3dFormParams", reqMap, p.logID)
	}

	now := time.Now()
	return &provider.PaymentResponse{
		Success:          true,
		Status:           provider.StatusPending,
		PaymentID:        orderId,
		Amount:           request.Amount,
		Currency:         request.Currency,
		HTML:             html,
		Message:          "3D Secure authentication required",
		SystemTime:       &now,
		ProviderResponse: formParams,
	}, nil
}

// build3DFormParams builds form parameters for 3D Secure payment
func (p *ZiraatProvider) build3DFormParams(request provider.PaymentRequest, orderId, callbackURL string) map[string]string {
	// Determine card type (1=Visa, 2=MasterCard)
	cardType := "1" // Default to Visa
	cardNumber := strings.ReplaceAll(request.CardInfo.CardNumber, " ", "")
	if len(cardNumber) > 0 {
		firstDigit := cardNumber[0]
		if firstDigit == '5' {
			cardType = "2" // MasterCard
		}
	}

	// Format amount (with 2 decimal places)
	amountStr := fmt.Sprintf("%.2f", request.Amount)

	// Generate random number
	rnd := fmt.Sprintf("%d", time.Now().UnixNano()/1e6)

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
		"clientid":                        p.merchantSafeId,
		"amount":                          amountStr,
		"okurl":                           callbackURL,
		"failUrl":                         callbackURL,
		"TranType":                        "Auth",
		"Instalment":                      "",
		"callbackUrl":                     callbackURL,
		"currency":                        "949", // TRY
		"rnd":                             rnd,
		"storetype":                       "3D_PAY",
		"hashAlgorithm":                   "ver3",
		"lang":                            "tr",
		"pan":                             request.CardInfo.CardNumber,
		"cv2":                             request.CardInfo.CVV,
		"Ecom_Payment_Card_ExpDate_Year":  expYear,
		"Ecom_Payment_Card_ExpDate_Month": request.CardInfo.ExpireMonth,
		"cardType":                        cardType,
		"BillToName":                      customerName,
		"BillToCompany":                   "",
	}

	// Add installment if specified
	if request.InstallmentCount > 1 {
		params["Instalment"] = strconv.Itoa(request.InstallmentCount)
	}

	return params
}

// calculate3DHash calculates SHA512 hash for 3D Secure form (Payten format)
func (p *ZiraatProvider) calculate3DHash(params map[string]string) (string, error) {
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

// generate3DSecureHTML generates HTML form for 3D Secure authentication
func (p *ZiraatProvider) generate3DSecureHTML(params map[string]string) string {
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

// buildBaseRequest builds the base request structure for Ziraat
func (p *ZiraatProvider) buildBaseRequest(txnCode string) map[string]any {
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
func (p *ZiraatProvider) sendPaymentRequest(ctx context.Context, requestData map[string]any) (*provider.PaymentResponse, error) {
	now := time.Now()
	resp, err := p.sendRequest(ctx, requestData)
	if err != nil {
		return nil, err
	}

	// Add provider request to client request
	if reqMap, err := provider.StructToMap(requestData); err == nil {
		_ = provider.AddProviderRequestToClientRequest("ziraat", "providerRequest", reqMap, p.logID)
	}

	// Map Ziraat response to common PaymentResponse
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

// sendRequest sends a request to Ziraat API
func (p *ZiraatProvider) sendRequest(ctx context.Context, requestData map[string]any) (map[string]any, error) {
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
func (p *ZiraatProvider) generateAuthHash(data string) string {
	h := hmac.New(sha512.New, []byte(p.secretKey))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// generateRequestDateTime generates request datetime in Ziraat format
func (p *ZiraatProvider) generateRequestDateTime() string {
	now := time.Now()
	// Format: 2006-01-02T15:04:05.000
	return now.Format("2006-01-02T15:04:05.") + fmt.Sprintf("%03d", now.Nanosecond()/1000000)
}

// generateRandomNumber generates a random hex string of specified length
func (p *ZiraatProvider) generateRandomNumber(length int) string {
	bytes := make([]byte, length/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// generateOrderId generates a unique order ID
func (p *ZiraatProvider) generateOrderId() string {
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
