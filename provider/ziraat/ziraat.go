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
	apiSandboxURL    = "https://torus-stage-ziraat.asseco-see.com.tr/fim/api"
	apiProductionURL = "https://stage-ziraat.asseco-see.com.tr/fim/api"

	// 3D Post URL
	api3DSandboxURL    = "https://torus-stage-ziraat.asseco-see.com.tr/fim/est3Dgate"
	api3DProductionURL = "https://sanalpos2.ziraatbank.com.tr/fim/est3Dgate"

	// Transaction Codes
	txnCodeSale   = "1000" // Direct sale
	txnCodeCancel = "2000" // Cancel/void
	txnCodeRefund = "2100" // Refund

	// Currency Codes
	currencyCodeTRY = 949

	// Default version
	apiVersion = "1.00"
)

// ZiraatProvider implements the provider.PaymentProvider interface for Ziraat
type ZiraatProvider struct {
	username                string
	password                string
	storeKey                string
	baseURL                 string
	threeDPostURL           string
	gopayBaseURL            string
	isProduction            bool
	httpClient              *provider.ProviderHTTPClient
	paymentManagementClient *provider.ProviderHTTPClient
	logID                   int64
}

// NewProvider creates a new Ziraat payment provider
func NewProvider() provider.PaymentProvider {
	return &ZiraatProvider{}
}

// GetRequiredConfig returns the configuration fields required for Ziraat
func (p *ZiraatProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "username",
			Required:    true,
			Type:        "string",
			Description: "Ziraat Username (provided by Ziraat)",
			Example:     "test",
			MinLength:   3,
			MaxLength:   50,
		},
		{
			Key:         "password",
			Required:    true,
			Type:        "string",
			Description: "Ziraat Password (provided by Ziraat)",
			Example:     "test",
			MinLength:   3,
			MaxLength:   50,
		},
		{
			Key:         "storeKey",
			Required:    true,
			Type:        "string",
			Description: "Ziraat Store Key (provided by Ziraat)",
			Example:     "test",
			MinLength:   3,
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

// ValidateConfig validates the provided configuration against Ziraat requirements
func (p *ZiraatProvider) ValidateConfig(config map[string]string) error {
	requiredFields := p.GetRequiredConfig(config["environment"])
	return provider.ValidateConfigFields("ziraat", config, requiredFields)
}

// Initialize sets up the Ziraat payment provider with authentication credentials
func (p *ZiraatProvider) Initialize(conf map[string]string) error {
	p.username = conf["username"]
	p.password = conf["password"]
	p.storeKey = conf["storeKey"]

	if p.username == "" || p.password == "" || p.storeKey == "" {
		return errors.New("ziraat: username, password and storeKey are required")
	}

	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	p.isProduction = conf["environment"] == "production"
	p.baseURL = apiSandboxURL
	if p.isProduction {
		p.baseURL = apiProductionURL
	}

	p.threeDPostURL = api3DSandboxURL
	if p.isProduction {
		p.threeDPostURL = api3DProductionURL
	}

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

// CreatePayment makes a payment request (Ziraat always uses 3D Secure)
func (p *ZiraatProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	// Ziraat only supports 3D Secure payments, so always use 3D flow
	return p.Create3DPayment(ctx, request)
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

	// Log received callback data for debugging
	if len(data) == 0 {
		return nil, errors.New("ziraat: no callback data received")
	}

	// Log callback data for tracking
	if reqMap, err := provider.StructToMap(data); err == nil {
		_ = provider.AddProviderRequestToClientRequest("ziraat", "callbackData", reqMap, p.logID)
	}

	// Extract payment status from callback
	// Note: Ziraat callback does not include HASH, only Response and ErrorCode
	// Check status parameter first (from okurl/failUrl), then fallback to Response/ErrorCode
	status, _ := data["status"]
	responseCode, _ := data["Response"]
	errorCode, _ := data["ErrorCode"]
	errorMsg, _ := data["ErrMsg"]
	transactionId, _ := data["traceId"]
	orderId, _ := data["paymentId"]
	procReturnCode, _ := data["ProcReturnCode"]

	// Determine success based on status parameter (from URL) or Response/ErrorCode
	var success bool
	if status != "" {
		// Status parameter from okurl/failUrl
		success = status == "SUCCESS"
	} else {
		// Fallback to Response and ProcReturnCode
		// Response: "Approved" = success, "Declined" = failure
		// ProcReturnCode: "00" = success, others = failure
		success = responseCode == "Approved" && (procReturnCode == "00" || procReturnCode == "")
	}

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

	// Set status and message based on status parameter or Response/ErrorCode
	if status != "" {
		// Status parameter from URL
		switch status {
		case "SUCCESS":
			response.Status = provider.StatusSuccessful
			response.Message = "3D payment completed successfully"
		case "FAILED":
			response.Status = provider.StatusFailed
			if errorMsg != "" {
				response.Message = errorMsg
			} else {
				response.Message = "3D payment failed"
			}
			if errorCode != "" {
				response.ErrorCode = errorCode
			} else if procReturnCode != "" {
				response.ErrorCode = procReturnCode
			} else if responseCode != "" {
				response.ErrorCode = responseCode
			}
		default:
			response.Status = provider.StatusPending
			response.Message = "3D payment pending"
		}
	} else {
		// Fallback to Response/ErrorCode (original Ziraat callback logic)
		if success {
			response.Status = provider.StatusSuccessful
			response.Message = "3D payment completed successfully"
		} else {
			response.Status = provider.StatusFailed
			if errorCode != "" {
				response.ErrorCode = errorCode
			} else if procReturnCode != "" {
				response.ErrorCode = procReturnCode
			} else {
				response.ErrorCode = responseCode
			}
			if errorMsg != "" {
				response.Message = errorMsg
			} else {
				response.Message = "3D payment failed"
			}
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

	// Create short callback URL (state will be stored in DB)
	gopayCallbackURL, err := provider.CreateShortCallbackURL(ctx, p.gopayBaseURL, "ziraat", state)
	if err != nil {
		return nil, fmt.Errorf("failed to create callback URL: %w", err)
	}

	// Prepare 3D form parameters
	formParams := p.build3DFormParams(request, gopayCallbackURL)

	// Calculate hash for 3D form
	hash, err := p.calculate3DHash(formParams)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate 3D hash: %w", err)
	}
	formParams["hash"] = hash

	// Log hash calculation for tracking
	if reqMap, err := provider.StructToMap(formParams); err == nil {
		_ = provider.AddProviderRequestToClientRequest("ziraat", "3DRequestForm", reqMap, p.logID)
	}

	// Generate HTML form
	html := p.generate3DSecureHTML(formParams)

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
func (p *ZiraatProvider) build3DFormParams(request provider.PaymentRequest, callbackURL string) map[string]string {
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

	// Generate random number (like PHP microtime() - returns float with microseconds)
	rnd := fmt.Sprintf("%.6f", float64(time.Now().UnixNano())/1e9)

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

	// Build form parameters (password and storekey should NOT be in form, only used for hash calculation)
	// okurl and failUrl include status parameter
	// callbackURL already has ?state=xxx, so we add &status=SUCCESS/FAILED
	okURL := callbackURL + "&status=SUCCESS"
	failURL := callbackURL + "&status=FAILED"

	params := map[string]string{
		"clientid":                        p.username,
		"amount":                          amountStr,
		"okurl":                           okURL,
		"failUrl":                         failURL,
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
		"refreshtime":                     "5", // Optional: refresh time in seconds
	}

	// Add installment if specified (only if > 1)
	if request.InstallmentCount > 1 {
		params["Instalment"] = strconv.Itoa(request.InstallmentCount)
	}

	return params
}

// calculate3DHash calculates SHA512 hash for 3D Secure form (Ziraat format)
// Based on GenericVer3RequestHashHandler.jsp: sorts params case-insensitively (toUpperCase), escapes values, adds storeKey
// JSP: MessageDigest.getInstance("SHA-512") -> Base64.encodeBase64(digest) (no hex conversion)
func (p *ZiraatProvider) calculate3DHash(params map[string]string) (string, error) {
	// Get sorted parameter keys (case-insensitive, like JSP TreeMap with toUpperCase comparator)
	keys := make([]string, 0, len(params))
	for k := range params {
		lowerKey := strings.ToLower(k)
		// Skip hash, encoding, and storekey parameters (storekey added separately)
		if lowerKey != "hash" && lowerKey != "encoding" && lowerKey != "storekey" {
			keys = append(keys, k)
		}
	}

	// Sort keys case-insensitively using toUpperCase comparison (like JSP: str1.toUpperCase().compareTo(str2.toUpperCase()))
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToUpper(keys[i]) < strings.ToUpper(keys[j])
	})

	// Build hash string (like JSP: hashval3 += escapedValue + "|")
	var hashVal strings.Builder
	for _, key := range keys {
		value := params[key]
		// Escape | and \ characters (like JSP: replace("\\", "\\\\").replace("|", "\\|"))
		escapedValue := strings.ReplaceAll(value, "\\", "\\\\")
		escapedValue = strings.ReplaceAll(escapedValue, "|", "\\|")
		hashVal.WriteString(escapedValue)
		hashVal.WriteString("|")
	}

	// Add storeKey at the end (like JSP: hashval3 += escapedStoreKey, no "|" after storeKey)
	escapedStoreKey := strings.ReplaceAll(p.storeKey, "\\", "\\\\")
	escapedStoreKey = strings.ReplaceAll(escapedStoreKey, "|", "\\|")
	hashVal.WriteString(escapedStoreKey)

	// Calculate SHA512 hash (like JSP: MessageDigest.getInstance("SHA-512"))
	hashBytes := sha512.Sum512([]byte(hashVal.String()))

	// Base64 encode directly (like JSP: Base64.encodeBase64(messageDigest.digest()))
	// JSP does NOT convert to hex first, it directly Base64 encodes the digest bytes
	hashBase64 := base64.StdEncoding.EncodeToString(hashBytes[:])

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
</html>`, p.threeDPostURL, formFields.String())

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
			"username": p.username,
			"password": p.password,
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
	h := hmac.New(sha512.New, []byte(p.storeKey))
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
