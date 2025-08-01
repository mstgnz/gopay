package nkolay

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// Real Nkolay API URLs from postman collection
	apiSandboxURL    = "https://paynkolaytest.nkolayislem.com.tr"
	apiProductionURL = "https://paynkolay.nkolayislem.com.tr"

	// Real API Endpoints from postman collection
	endpointPayment             = "/Vpos/v1/Payment"
	endpointPaymentInstallments = "/Vpos/Payment/GetMerchandInformation"
	endpointPaymentForm         = "/Vpos/Payment/Payment"
	endpointCancelRefund        = "/Vpos/v1/CancelRefundPayment"
	endpointPartialRefund       = "/Vpos/Payment/PartialRefundPayment"
	endpointPaymentList         = "/Vpos/Payment/PaymentList"

	// Test credentials provided
	testSx        = "118591467|bScbGDYCtPf7SS1N6PQ6/+58rFhW1WpsWINqvkJFaJlu6bMH2tgPKDQtjeA5vClpzJP24uA0vx7OX53cP3SgUspa4EvYix+1C3aXe++8glUvu9Oyyj3v300p5NP7ro/9K57Zcw=="
	testSxList    = "118591467|bScbGDYCtPf7SS1N6PQ6/+58rFhW1WpsWINqvkJFaJlu6bMH2tgPKDQtjeA5vClpzJP24uA0vx7OX53cP3SgUspa4EvYix+1C3aXe++8glUvu9Oyyj3v300p5NP7ro/9K57Zcw==|3hJpHVF2cqvcCZ4q6F7rcA=="
	testSxCancel  = "118591467|bScbGDYCtPf7SS1N6PQ6/+58rFhW1WpsWINqvkJFaJlu6bMH2tgPKDQtjeA5vClpzJP24uA0vx7OX53cP3SgUspa4EvYix+1C3aXe++8glUvu9Oyyj3v300p5NP7ro/9K57Zcw==|yDUZaCk6rsoHZJWI3d471A/+TJA7C81X"
	testSecretKey = "_YckdxUbv4vrnMUZ6VQsr"

	// Response Status Values from postman
	statusSuccess   = "SUCCESS"
	statusFailed    = "FAILED"
	statusPending   = "PENDING"
	statusCancelled = "CANCELLED"
	statusRefunded  = "REFUNDED"

	// Default Values
	defaultCurrency = "TRY"
	currencyCodeTRY = "949"
	defaultTimeout  = 30 * time.Second
)

// NkolayProvider implements the provider.PaymentProvider interface for Nkolay
type NkolayProvider struct {
	sx           string // Test token provided by Nkolay
	sxList       string // Token for listing operations
	sxCancel     string // Token for cancel/refund operations
	secretKey    string // Merchant secret key
	baseURL      string
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
	httpClient   *provider.ProviderHTTPClient
	logID        int64
}

// NewProvider creates a new Nkolay payment provider
func NewProvider() provider.PaymentProvider {
	return &NkolayProvider{}
}

// GetRequiredConfig returns the configuration fields required for Nkolay
func (p *NkolayProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "sx",
			Required:    true,
			Type:        "string",
			Description: "Nkolay SX token for payment operations (optional, uses test value if not provided)",
			Example:     "118591467|bScbGDYCtPf7SS1N...",
			MinLength:   10,
			MaxLength:   500,
		},
		{
			Key:         "sxList",
			Required:    true,
			Type:        "string",
			Description: "Nkolay SX token for listing operations (optional, uses test value if not provided)",
			Example:     "118591467|bScbGDYCtPf7SS1N...|3hJpHVF2cqvcCZ4q6F7rcA==",
			MinLength:   10,
			MaxLength:   500,
		},
		{
			Key:         "sxCancel",
			Required:    true,
			Type:        "string",
			Description: "Nkolay SX token for cancel/refund operations (optional, uses test value if not provided)",
			Example:     "118591467|bScbGDYCtPf7SS1N...|yDUZaCk6rsoHZJWI3d471A/+TJA7C81X",
			MinLength:   10,
			MaxLength:   500,
		},
		{
			Key:         "secretKey",
			Required:    true,
			Type:        "string",
			Description: "Nkolay Secret Key (optional, uses test value if not provided)",
			Example:     "_YckdxUbv4vrnMUZ6VQsr",
			MinLength:   5,
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

// ValidateConfig validates the provided configuration against Nkolay requirements
func (p *NkolayProvider) ValidateConfig(config map[string]string) error {
	requiredFields := p.GetRequiredConfig(config["environment"])
	return provider.ValidateConfigFields("nkolay", config, requiredFields)
}

// Initialize sets up the Nkolay payment provider with authentication credentials
func (p *NkolayProvider) Initialize(conf map[string]string) error {
	// For real API, use provided credentials. For testing, use test values
	if sx := conf["sx"]; sx != "" {
		p.sx = sx
	} else {
		p.sx = testSx // Use test sx if not provided
	}

	if sxList := conf["sxList"]; sxList != "" {
		p.sxList = sxList
	} else {
		p.sxList = testSxList
	}

	if sxCancel := conf["sxCancel"]; sxCancel != "" {
		p.sxCancel = sxCancel
	} else {
		p.sxCancel = testSxCancel
	}

	if secretKey := conf["secretKey"]; secretKey != "" {
		p.secretKey = secretKey
	} else {
		p.secretKey = testSecretKey
	}

	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	p.isProduction = conf["environment"] == "production"
	if p.isProduction {
		p.baseURL = apiProductionURL
	} else {
		p.baseURL = apiSandboxURL
	}

	p.httpClient = provider.NewProviderHTTPClient(provider.CreateHTTPClientConfig(p.baseURL, p.isProduction))

	return nil
}

// GetInstallmentCount returns the installment count for a payment
func (p *NkolayProvider) GetInstallmentCount(ctx context.Context, request provider.InstallmentInquireRequest) (provider.InstallmentInquireResponse, error) {
	formData := map[string]string{
		"sx":     p.sx,
		"amount": fmt.Sprintf("%.2f", request.Amount),
	}

	responseBody, err := p.doNkolayFormRequest(ctx, endpointPaymentInstallments, formData)
	if err != nil {
		return provider.InstallmentInquireResponse{}, fmt.Errorf("nkolay: failed to get installment count: %w", err)
	}

	// Parse response as map first
	var rawResponse map[string]any
	if err := json.Unmarshal(responseBody, &rawResponse); err != nil {
		return provider.InstallmentInquireResponse{}, fmt.Errorf("nkolay: failed to unmarshal installment count response: %w", err)
	}

	// Initialize response structure
	response := provider.InstallmentInquireResponse{
		Amount:       request.Amount,
		Message:      "Installment options retrieved successfully",
		Installments: make(map[string][]provider.InstallmentInfo),
	}

	// Extract commission list from response
	commissionList, ok := rawResponse["COMMISSION_LIST"].([]any)
	if !ok || len(commissionList) == 0 {
		return provider.InstallmentInquireResponse{}, fmt.Errorf("nkolay: no commission list found in response")
	}

	// Process each commission entry (bank/card type)
	for _, entry := range commissionList {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		// Get card type code (PARAF, AXESS, etc.)
		cardType, ok := entryMap["CODE"].(string)
		if !ok {
			continue
		}

		// Get installment data array
		dataArray, ok := entryMap["DATA"].([]any)
		if !ok {
			continue
		}

		// Process installment options for this card type
		var installmentInfos []provider.InstallmentInfo
		for _, dataEntry := range dataArray {
			dataMap, ok := dataEntry.(map[string]any)
			if !ok {
				continue
			}

			// Extract installment number
			installmentNum, ok := dataMap["INSTALLMENT"].(float64)
			if !ok {
				continue
			}

			// Extract commission rate
			commission, ok := dataMap["MERCHANT_COMMISSION"].(float64)
			if !ok {
				continue
			}

			installmentInfos = append(installmentInfos, provider.InstallmentInfo{
				Installment: int(installmentNum),
				Commission:  commission,
			})
		}

		// Add to response if we have valid data
		if len(installmentInfos) > 0 {
			response.Installments[cardType] = installmentInfos
		}
	}

	return response, nil
}

// GetCommission returns the commission for a payment
func (p *NkolayProvider) GetCommission(ctx context.Context, request provider.CommissionRequest) (provider.CommissionResponse, error) {
	return provider.CommissionResponse{}, nil
}

// CreatePayment makes a non-3D payment request
func (p *NkolayProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("nkolay: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *NkolayProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("nkolay: invalid 3D payment request: %w", err)
	}

	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *NkolayProvider) Complete3DPayment(ctx context.Context, callbackState *provider.CallbackState, data map[string]string) (*provider.PaymentResponse, error) {
	p.logID = callbackState.LogID

	status := data["status"]

	response := &provider.PaymentResponse{
		PaymentID:        callbackState.PaymentID,
		TransactionID:    callbackState.PaymentID,
		Success:          status == statusSuccess,
		Message:          "3D payment completed successfully",
		SystemTime:       timePtr(time.Now()),
		ProviderResponse: data,
		Amount:           callbackState.Amount,
		Currency:         callbackState.Currency,
		RedirectURL:      callbackState.OriginalCallback,
	}

	// Map status
	switch status {
	case statusSuccess:
		response.Status = provider.StatusSuccessful
		response.Message = "3D payment completed successfully"
	case statusFailed:
		response.Status = provider.StatusFailed
		response.Message = "3D payment failed"
	default:
		response.Status = provider.StatusPending
		response.Message = "3D payment pending"
	}

	// Parse amount if available
	if amountStr := data["amount"]; amountStr != "" {
		if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
			response.Amount = amount
		}
	}

	return response, nil
}

// GetPaymentStatus retrieves the current status of a payment
func (p *NkolayProvider) GetPaymentStatus(ctx context.Context, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	// Use list API to get payment status
	today := time.Now()
	formData := map[string]string{
		"sx":            p.sxList,
		"startDate":     today.AddDate(0, 0, -1).Format("02.01.2006"), // Yesterday
		"endDate":       today.Format("02.01.2006"),                   // Today
		"clientRefCode": request.PaymentID,
	}

	// Generate hash: sx+startDate+endDate+clientRefCode+secretkey
	input := formData["sx"] + formData["startDate"] + formData["endDate"] + formData["clientRefCode"] + p.secretKey
	formData["hashData"] = p.generateSHA1Hash(input)

	responseBody, err := p.doNkolayFormRequest(ctx, endpointPaymentList, formData)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to get payment status: %w", err)
	}

	// Parse response (Nkolay returns XML/HTML format)
	// For now, return a basic response - would need XML parsing for full implementation
	return &provider.PaymentResponse{
		PaymentID:  request.PaymentID,
		Success:    strings.Contains(string(responseBody), "SUCCESS"),
		Status:     provider.StatusPending,
		Message:    "Status check completed",
		SystemTime: timePtr(time.Now()),
		ProviderResponse: map[string]any{
			"raw_response": string(responseBody),
		},
	}, nil
}

// CancelPayment cancels a payment (same day cancellation)
func (p *NkolayProvider) CancelPayment(ctx context.Context, request provider.CancelRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	systemTime, err := provider.GetProviderRequestFromLogWithPaymentID("nkolay", request.PaymentID, "systemTime")
	if err != nil {
		return nil, fmt.Errorf("failed to get systemTime: %s %w", request.PaymentID, err)
	}

	// Convert systemTime from "2025-07-23T11:30:21.163704+03" to "2025.07.23" format
	trxDate, err := p.formatDateForNkolay(systemTime)
	if err != nil {
		return nil, fmt.Errorf("failed to format trxDate: %w", err)
	}

	formData := map[string]string{
		"sx":            p.sxCancel,
		"referenceCode": request.PaymentID,
		"type":          "cancel",
		"trxDate":       trxDate,
		"resultUrl":     "json",
	}

	// Generate hash: sx+referenceCode+type+trxDate+secretkey
	input := formData["sx"] + formData["referenceCode"] + formData["type"] + formData["trxDate"] + p.secretKey
	formData["hashData"] = p.generateSHA1Hash(input)

	responseBody, err := p.doNkolayFormRequest(ctx, endpointCancelRefund, formData)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to cancel payment: %w", err)
	}

	return &provider.PaymentResponse{
		PaymentID:  request.PaymentID,
		Success:    strings.Contains(string(responseBody), "SUCCESS"),
		Status:     provider.StatusCancelled,
		Message:    "Payment cancellation processed",
		SystemTime: timePtr(time.Now()),
		ProviderResponse: map[string]any{
			"raw_response": string(responseBody),
		},
	}, nil
}

// RefundPayment issues a refund for a payment
func (p *NkolayProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	systemTime, err := provider.GetProviderRequestFromLogWithPaymentID("nkolay", request.PaymentID, "systemTime")
	if err != nil {
		return nil, fmt.Errorf("failed to get systemTime: %s %w", request.PaymentID, err)
	}

	// Convert systemTime from "2025-07-23T11:30:21.163704+03" to "2025.07.23" format
	trxDate, err := p.formatDateForNkolay(systemTime)
	if err != nil {
		return nil, fmt.Errorf("failed to format trxDate: %w", err)
	}

	refundAmount := request.RefundAmount
	if refundAmount <= 0 {
		return nil, errors.New("nkolay: refund amount must be greater than 0")
	}

	formData := map[string]string{
		"sx":            p.sxCancel,
		"referenceCode": request.PaymentID,
		"type":          "refund",
		"trxDate":       trxDate,
		"amount":        fmt.Sprintf("%.2f", refundAmount),
		"resultUrl":     "json",
	}

	// Generate hash: sx+referenceCode+type+amount+trxDate+secretkey
	input := formData["sx"] + formData["referenceCode"] + formData["type"] + formData["amount"] + formData["trxDate"] + p.secretKey
	formData["hashData"] = p.generateSHA1Hash(input)

	responseBody, err := p.doNkolayFormRequest(ctx, endpointCancelRefund, formData)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to refund payment: %w", err)
	}

	return &provider.RefundResponse{
		Success:      strings.Contains(string(responseBody), "SUCCESS"),
		RefundID:     fmt.Sprintf("refund_%s_%d", request.PaymentID, time.Now().Unix()),
		PaymentID:    request.PaymentID,
		RefundAmount: refundAmount,
		Status:       "processed",
		Message:      "Refund processed",
		SystemTime:   timePtr(time.Now()),
		RawResponse: map[string]any{
			"raw_response": string(responseBody),
		},
	}, nil
}

// ValidateWebhook validates incoming webhook notifications
func (p *NkolayProvider) ValidateWebhook(ctx context.Context, data, headers map[string]string) (bool, map[string]string, error) {
	// Nkolay sends POST callbacks to success/fail URLs
	// Basic validation - in real implementation would verify signature
	if referenceCode := data["referenceCode"]; referenceCode != "" {
		return true, data, nil
	}

	return false, nil, errors.New("nkolay: invalid webhook data")
}

// validatePaymentRequest validates the payment request
func (p *NkolayProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
	if request.TenantID == 0 {
		return errors.New("tenantID is required")
	}

	if request.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}

	if request.Currency == "" {
		return errors.New("currency is required")
	}

	if request.Customer.Name == "" || request.Customer.Surname == "" {
		return errors.New("customer name and surname are required")
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
		return errors.New("expiry date is required")
	}

	if is3D && request.CallbackURL == "" {
		return errors.New("callback URL is required for 3D payments")
	}

	return nil
}

// processPayment handles both regular and 3D payment processing
func (p *NkolayProvider) processPayment(ctx context.Context, request provider.PaymentRequest, use3D bool) (*provider.PaymentResponse, error) {
	// Generate unique reference code
	clientRefCode := fmt.Sprintf("gopay_%d", time.Now().UnixNano())

	formData := map[string]string{
		"sx":              p.sx,
		"clientRefCode":   clientRefCode,
		"amount":          fmt.Sprintf("%.2f", request.Amount),
		"transactionType": "SALES",
		"rnd":             time.Now().Format("02-01-2006 15:04:05"),
		"instalments":     strconv.Itoa(request.InstallmentCount),
		"installmentNo":   strconv.Itoa(request.InstallmentCount),
		"ECOMM_PLATFORM":  "GOPAY",
	}

	if request.InstallmentCount > 0 {
		// get installment count from nkolay
		installmentCount, err := p.GetInstallmentCount(ctx, provider.InstallmentInquireRequest{
			Amount: request.Amount,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get installment count: %w", err)
		}
		// ana tutarı + ( ana tutar * komisyon oranı /100)
		// find installment count in installmentCount.Installments["OTHERS"]
		for _, installment := range installmentCount.Installments["OTHERS"] {
			if installment.Installment == request.InstallmentCount {
				request.Amount = request.Amount + (request.Amount * installment.Commission / 100)
				break
			}
		}
	}

	// Add 3D settings
	stateId := ""
	if use3D {
		formData["use3D"] = "true"
		// Build callback URLs through GoPay

		state := provider.CallbackState{
			PaymentID:        request.ID,
			TenantID:         request.TenantID,
			Amount:           request.Amount,
			Currency:         request.Currency,
			LogID:            request.LogID,
			Provider:         "nkolay",
			Environment:      request.Environment,
			Timestamp:        time.Now(),
			OriginalCallback: request.CallbackURL,
			ClientIP:         request.ClientIP,
		}

		gopayCallbackURL, err := provider.CreateShortCallbackURL(ctx, p.gopayBaseURL, "nkolay", state)
		if err != nil {
			return nil, fmt.Errorf("failed to create short callback URL: %w", err)
		}

		// Extract state ID from the callback URL
		if parsedURL, err := url.Parse(gopayCallbackURL); err == nil {
			stateId = parsedURL.Query().Get("state")
		}

		formData["successUrl"] = gopayCallbackURL + "&status=SUCCESS"
		formData["failUrl"] = gopayCallbackURL + "&status=FAILED"

	}

	if request.CardInfo.CardHolderName != "" {
		formData["cardHolderName"] = request.CardInfo.CardHolderName
	}
	if request.CardInfo.ExpireMonth != "" {
		formData["month"] = request.CardInfo.ExpireMonth
	}
	if request.CardInfo.ExpireYear != "" {
		formData["year"] = request.CardInfo.ExpireYear
	}
	if request.CardInfo.CVV != "" {
		formData["cvv"] = request.CardInfo.CVV
	}
	if request.CardInfo.CardNumber != "" {
		formData["cardNumber"] = request.CardInfo.CardNumber
	}

	// Generate hash according to Nkolay documentation
	// Hash format varies by endpoint, for payment it's specific fields + secret key
	input := formData["sx"] + formData["clientRefCode"] + formData["amount"] + formData["successUrl"] + formData["failUrl"] + formData["rnd"] + p.secretKey
	formData["hashData"] = p.generateSHA1Hash(input)

	responseBody, err := p.doNkolayFormRequest(ctx, endpointPayment, formData)
	if err != nil {
		return nil, fmt.Errorf("nkolay: payment request failed: %w", err)
	}

	// add provider request to client request
	_ = provider.AddProviderRequestToClientRequest("nkolay", "providerRequest", formData, p.logID)

	return p.parsePaymentResponse(responseBody, clientRefCode, request.Amount, stateId)
}

// generateSHA1Hash generates SHA1 hash and encodes it in base64 (Nkolay official format)
func (p *NkolayProvider) generateSHA1Hash(input string) string {

	// PHP equivalent: base64_encode(pack('H*', sha1($hashstr)))
	// This means: SHA1 -> hex string -> binary -> base64
	h := sha1.New()
	h.Write([]byte(input))
	hexHash := fmt.Sprintf("%x", h.Sum(nil)) // Get hex string

	// Convert hex string to binary (like PHP's pack('H*', ...))
	binaryData := make([]byte, len(hexHash)/2)
	for i := 0; i < len(hexHash); i += 2 {
		val, _ := strconv.ParseUint(hexHash[i:i+2], 16, 8)
		binaryData[i/2] = byte(val)
	}

	return base64.StdEncoding.EncodeToString(binaryData)
}

// doNkolayFormRequest is a helper to send multipart/form-data requests to Nkolay API
func (p *NkolayProvider) doNkolayFormRequest(ctx context.Context, endpoint string, formData map[string]string) ([]byte, error) {
	httpReq := &provider.HTTPRequest{
		Method:   "POST",
		Endpoint: endpoint,
		FormData: formData,
		Headers:  map[string]string{"Accept": "application/json, text/html"},
	}
	resp, err := p.httpClient.SendForm(ctx, httpReq)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// parsePaymentResponse parses Nkolay payment response
func (p *NkolayProvider) parsePaymentResponse(responseBody []byte, paymentID string, amount float64, stateId string) (*provider.PaymentResponse, error) {
	response := &provider.PaymentResponse{
		PaymentID:  paymentID,
		Amount:     amount,
		Currency:   defaultCurrency,
		SystemTime: timePtr(time.Now()),
		ProviderResponse: map[string]any{
			"raw_response": string(responseBody),
		},
	}

	responseStr := string(responseBody)

	// Try to parse as JSON first
	var jsonResponse map[string]any
	if err := json.Unmarshal(responseBody, &jsonResponse); err == nil {
		// JSON response - extract values
		responseCode := jsonResponse["RESPONSE_CODE"]
		responseData := jsonResponse["RESPONSE_DATA"]
		authCode := jsonResponse["AUTH_CODE"]
		referenceCode := jsonResponse["REFERENCE_CODE"]
		errorMessage := jsonResponse["ERROR_MESSAGE"]
		htmlString := jsonResponse["HTML_STRING"]

		// Update payment ID to use reference code if available
		if refCode, ok := referenceCode.(string); ok && refCode != "" {
			response.PaymentID = refCode
			// nkolay işlemlerinde referans kodu sonradan geldiği için 3d complete işlemlerine akarılamıyor.
			// bunu çözmek için burada atmamız gerek onun içinde callbacks state güncellememiz gerekiyor.
			if stateId != "" {
				_ = provider.UpdateCallbackState(context.Background(), stateId, refCode)
			}
		}

		// Store additional data in provider response
		if providerResp, ok := response.ProviderResponse.(map[string]any); ok {
			if authCode != nil {
				providerResp["auth_code"] = authCode
			}
			if referenceCode != nil {
				providerResp["reference_code"] = referenceCode
			}
		}

		// Check for 3D Secure HTML form in BANK_REQUEST_MESSAGE
		if bankRequestMessage, ok := jsonResponse["BANK_REQUEST_MESSAGE"].(string); ok && bankRequestMessage != "" && strings.Contains(bankRequestMessage, "form") {
			response.Success = true
			response.Status = provider.StatusPending
			response.Message = "3D Secure authentication required"

			// Clean HTML for client use
			cleanHTML := p.cleanHTMLForClient(bankRequestMessage)
			response.HTML = cleanHTML

			return response, nil
		}

		// Also check HTML_STRING as fallback
		if htmlStr, ok := htmlString.(string); ok && htmlStr != "" && strings.Contains(htmlStr, "form") {
			response.Success = true
			response.Status = provider.StatusPending
			response.Message = "3D Secure authentication required"
			response.HTML = htmlStr

			return response, nil
		}

		// Check response code for success
		if code, ok := responseCode.(float64); ok {
			switch int(code) {
			case 2: // Success response code for Nkolay
				response.Success = true
				response.Status = provider.StatusSuccessful
				if responseData != nil {
					response.Message = fmt.Sprintf("%v", responseData)
				} else {
					response.Message = "Payment successful"
				}
			case 0, 1, 3, 4, 5: // Various error codes
				response.Success = false
				response.Status = provider.StatusFailed
				if errorMessage != nil && errorMessage != "" {
					response.Message = fmt.Sprintf("%v", errorMessage)
				} else if responseData != nil {
					response.Message = fmt.Sprintf("%v", responseData)
				} else {
					response.Message = "Payment failed"
				}

				// Set error code based on response code
				switch int(code) {
				case 0:
					response.ErrorCode = "GENERAL_ERROR"
				case 1:
					response.ErrorCode = "INVALID_REQUEST"
				case 3:
					response.ErrorCode = "INSUFFICIENT_FUNDS"
				case 4:
					response.ErrorCode = "INVALID_CARD"
				case 5:
					response.ErrorCode = "DECLINED"
				default:
					response.ErrorCode = "PAYMENT_FAILED"
				}
			default:
				response.Success = false
				response.Status = provider.StatusFailed
				response.Message = "Unknown response code"
				response.ErrorCode = "UNKNOWN_RESPONSE"
			}
		} else {
			// No response code found, check for other indicators
			if responseData != nil && strings.Contains(strings.ToUpper(fmt.Sprintf("%v", responseData)), "BAŞARILI") {
				response.Success = true
				response.Status = provider.StatusSuccessful
				response.Message = fmt.Sprintf("%v", responseData)
			} else {
				response.Success = false
				response.Status = provider.StatusFailed
				response.Message = "Invalid response format"
				response.ErrorCode = "INVALID_RESPONSE"
			}
		}

		return response, nil
	}

	// Fallback to HTML/text parsing for non-JSON responses
	if strings.Contains(responseStr, "form") && strings.Contains(responseStr, "action") {
		// 3D Secure form returned
		response.Success = true
		response.Status = provider.StatusPending
		response.Message = "3D Secure authentication required"
		response.HTML = responseStr
	} else if strings.Contains(responseStr, "SUCCESS") || strings.Contains(responseStr, "APPROVED") {
		// Payment successful
		response.Success = true
		response.Status = provider.StatusSuccessful
		response.Message = "Payment successful"
	} else if strings.Contains(responseStr, "FAILED") || strings.Contains(responseStr, "ERROR") {
		// Payment failed
		response.Success = false
		response.Status = provider.StatusFailed
		response.Message = "Payment failed"

		// Extract error details
		if strings.Contains(responseStr, "INSUFFICIENT") {
			response.ErrorCode = "INSUFFICIENT_FUNDS"
		} else if strings.Contains(responseStr, "INVALID") {
			response.ErrorCode = "INVALID_CARD"
		} else {
			response.ErrorCode = "PAYMENT_FAILED"
		}
	} else {
		// Unknown response
		response.Success = false
		response.Status = provider.StatusFailed
		response.Message = "Unknown response from Nkolay"
		response.ErrorCode = "UNKNOWN_RESPONSE"
	}

	return response, nil
}

// formatDateForNkolay converts systemTime from "2025-07-23T11:30:21.163704+03" to "2025.07.23" format
func (p *NkolayProvider) formatDateForNkolay(systemTime string) (string, error) {
	// Parse the systemTime which is in format "2025-07-23T11:30:21.163704+03"
	// We want to extract just the date part and format as "2025.07.23"

	// Find the first 'T' to split date and time
	datepart := systemTime
	if tIndex := strings.Index(systemTime, "T"); tIndex != -1 {
		datepart = systemTime[:tIndex]
	}

	// Parse the date part "2025-07-23"
	parsedTime, err := time.Parse("2006-01-02", datepart)
	if err != nil {
		return "", fmt.Errorf("failed to parse date %s: %w", datepart, err)
	}

	// Format as "2025.07.23"
	return parsedTime.Format("2006.01.02"), nil
}

// timePtr returns a pointer to the given time
func timePtr(t time.Time) *time.Time {
	return &t
}

// cleanHTMLForClient cleans HTML by removing escape characters and formatting properly
func (p *NkolayProvider) cleanHTMLForClient(htmlStr string) string {
	// Remove common escape characters
	cleanHTML := strings.ReplaceAll(htmlStr, "\\r", "")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\\n", "")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\\t", "")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\r", "")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\n", "")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\t", "")

	// Remove JSON escape characters
	cleanHTML = strings.ReplaceAll(cleanHTML, "\\\"", "\"")
	cleanHTML = strings.ReplaceAll(cleanHTML, "\\/", "/")

	// Fix JavaScript onload attribute quotation issue
	// Replace: onload=document.forms["form"].submit()
	// With: onload="document.forms['form'].submit()"
	cleanHTML = strings.ReplaceAll(cleanHTML, `onload=document.forms["form"].submit()`, `onload="document.forms['form'].submit()"`)

	// Remove extra spaces between tags and attributes
	onloadRegex := regexp.MustCompile(`>\s*<`)
	cleanHTML = onloadRegex.ReplaceAllString(cleanHTML, "><")

	// Clean script tag formatting
	cleanHTML = strings.ReplaceAll(cleanHTML, ">    var ", "> var ")
	cleanHTML = strings.ReplaceAll(cleanHTML, ";    ", "; ")

	return cleanHTML
}
