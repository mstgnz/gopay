package nkolay

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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
	endpointPaymentInstallments = "/Vpos/Payment/PaymentInstallments"
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

// GetRequiredConfig returns the configuration fields required for Nkolay
func (p *NkolayProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "sx",
			Required:    false,
			Type:        "string",
			Description: "Nkolay SX token for payment operations (optional, uses test value if not provided)",
			Example:     "118591467|bScbGDYCtPf7SS1N...",
			MinLength:   10,
			MaxLength:   500,
		},
		{
			Key:         "sxList",
			Required:    false,
			Type:        "string",
			Description: "Nkolay SX token for listing operations (optional, uses test value if not provided)",
			Example:     "118591467|bScbGDYCtPf7SS1N...|3hJpHVF2cqvcCZ4q6F7rcA==",
			MinLength:   10,
			MaxLength:   500,
		},
		{
			Key:         "sxCancel",
			Required:    false,
			Type:        "string",
			Description: "Nkolay SX token for cancel/refund operations (optional, uses test value if not provided)",
			Example:     "118591467|bScbGDYCtPf7SS1N...|yDUZaCk6rsoHZJWI3d471A/+TJA7C81X",
			MinLength:   10,
			MaxLength:   500,
		},
		{
			Key:         "secretKey",
			Required:    false,
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
	// For Nkolay, 3D completion happens automatically via callback
	// This method will validate the callback data and return status

	// Extract payment status from callback data
	status := data["status"]
	if status == "" {
		status = data["State"]
	}

	response := &provider.PaymentResponse{
		PaymentID:        paymentID,
		TransactionID:    data["referenceCode"],
		Success:          status == statusSuccess,
		Message:          data["message"],
		SystemTime:       timePtr(time.Now()),
		ProviderResponse: data,
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
func (p *NkolayProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	// Use list API to get payment status
	today := time.Now()
	formData := map[string]string{
		"sx":            p.sxList,
		"startDate":     today.AddDate(0, 0, -1).Format("02.01.2006"), // Yesterday
		"endDate":       today.Format("02.01.2006"),                   // Today
		"clientRefCode": paymentID,
	}

	// Generate hash: sx+startDate+endDate+clientRefCode+secretkey
	hashData := formData["sx"] + formData["startDate"] + formData["endDate"] + formData["clientRefCode"] + p.secretKey
	formData["hashData"] = p.generateSHA1Hash(hashData)

	responseBody, err := p.sendFormRequest(ctx, endpointPaymentList, formData)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to get payment status: %w", err)
	}

	// Parse response (Nkolay returns XML/HTML format)
	// For now, return a basic response - would need XML parsing for full implementation
	return &provider.PaymentResponse{
		PaymentID:  paymentID,
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
func (p *NkolayProvider) CancelPayment(ctx context.Context, paymentID, reason string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("nkolay: paymentID is required")
	}

	formData := map[string]string{
		"sx":            p.sxCancel,
		"referenceCode": paymentID,
		"type":          "cancel",
		"trxDate":       time.Now().Format("2006.01.02"),
	}

	// Generate hash: sx+referenceCode+type+trxDate+secretkey
	hashData := formData["sx"] + formData["referenceCode"] + formData["type"] + formData["trxDate"] + p.secretKey
	formData["hashData"] = p.generateSHA1Hash(hashData)

	responseBody, err := p.sendFormRequest(ctx, endpointCancelRefund, formData)
	if err != nil {
		return nil, fmt.Errorf("nkolay: failed to cancel payment: %w", err)
	}

	return &provider.PaymentResponse{
		PaymentID:  paymentID,
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

	refundAmount := request.RefundAmount
	if refundAmount <= 0 {
		return nil, errors.New("nkolay: refund amount must be greater than 0")
	}

	formData := map[string]string{
		"sx":            p.sxCancel,
		"referenceCode": request.PaymentID,
		"type":          "refund",
		"trxDate":       time.Now().Format("2006.01.02"),
		"amount":        fmt.Sprintf("%.2f", refundAmount),
	}

	// Generate hash: sx+referenceCode+type+amount+trxDate+secretkey
	hashData := formData["sx"] + formData["referenceCode"] + formData["type"] + formData["amount"] + formData["trxDate"] + p.secretKey
	formData["hashData"] = p.generateSHA1Hash(hashData)

	responseBody, err := p.sendFormRequest(ctx, endpointCancelRefund, formData)
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

	// Generate timestamp
	rnd := time.Now().Format("02-01-2006 15:04:05")

	formData := map[string]string{
		"sx":              p.sx,
		"clientRefCode":   clientRefCode,
		"amount":          fmt.Sprintf("%.2f", request.Amount),
		"installmentNo":   "1", // Default to 1 installment
		"cardHolderName":  request.CardInfo.CardHolderName,
		"month":           request.CardInfo.ExpireMonth,
		"year":            request.CardInfo.ExpireYear,
		"cvv":             request.CardInfo.CVV,
		"cardNumber":      request.CardInfo.CardNumber,
		"transactionType": "SALES",
		"rnd":             rnd,
		"environment":     "API",
		"currencyNumber":  currencyCodeTRY,
	}

	// Add 3D settings
	if use3D {
		formData["use3D"] = "true"
		// Build callback URLs through GoPay
		successUrl := fmt.Sprintf("%s/v1/callback/nkolay", p.gopayBaseURL)
		failUrl := fmt.Sprintf("%s/v1/callback/nkolay", p.gopayBaseURL)

		if request.CallbackURL != "" {
			successUrl += "?originalCallbackUrl=" + url.QueryEscape(request.CallbackURL) + "&status=success"
			failUrl += "?originalCallbackUrl=" + url.QueryEscape(request.CallbackURL) + "&status=failed"
		}

		formData["successUrl"] = successUrl
		formData["failUrl"] = failUrl
	}

	// Add optional fields
	if request.Customer.Name != "" {
		formData["namesurname"] = request.Customer.Name + " " + request.Customer.Surname
	}
	if request.Customer.Email != "" {
		formData["email"] = request.Customer.Email
	}
	if request.Customer.PhoneNumber != "" {
		// Remove country code and + sign
		phone := strings.ReplaceAll(request.Customer.PhoneNumber, "+90", "")
		phone = strings.ReplaceAll(phone, "+", "")
		formData["phone"] = phone
	}
	if request.Description != "" {
		formData["description"] = request.Description
	}

	// Generate hash according to Nkolay documentation
	// Hash format varies by endpoint, for payment it's specific fields + secret key
	hashString := p.generatePaymentHash(formData)
	formData["hashData"] = hashString

	responseBody, err := p.sendFormRequest(ctx, endpointPayment, formData)
	if err != nil {
		return nil, fmt.Errorf("nkolay: payment request failed: %w", err)
	}

	return p.parsePaymentResponse(responseBody, clientRefCode, request.Amount)
}

// generatePaymentHash generates the payment hash according to Nkolay specs
func (p *NkolayProvider) generatePaymentHash(formData map[string]string) string {
	// According to Nkolay docs: specific fields + secret key
	// This is a simplified version - real implementation would need exact field order
	hashInput := formData["sx"] + formData["clientRefCode"] + formData["amount"] + formData["rnd"] + p.secretKey
	return p.generateSHA1Hash(hashInput)
}

// generateSHA1Hash generates SHA1 hash and encodes it in base64
func (p *NkolayProvider) generateSHA1Hash(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// sendFormRequest sends form-data request to Nkolay API
func (p *NkolayProvider) sendFormRequest(ctx context.Context, endpoint string, formData map[string]string) ([]byte, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add form fields
	for key, value := range formData {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("failed to write form field %s: %w", key, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close form writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+endpoint, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return responseBody, nil
}

// parsePaymentResponse parses Nkolay payment response
func (p *NkolayProvider) parsePaymentResponse(responseBody []byte, paymentID string, amount float64) (*provider.PaymentResponse, error) {
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

		// Check for 3D Secure HTML form
		if htmlStr, ok := htmlString.(string); ok && htmlStr != "" && strings.Contains(htmlStr, "form") {
			response.Success = true
			response.Status = provider.StatusPending
			response.Message = "3D Secure authentication required"
			response.HTML = htmlStr

			// Extract redirect URL from form action
			if actionStart := strings.Index(htmlStr, `action="`); actionStart != -1 {
				actionStart += 8 // len(`action="`)
				if actionEnd := strings.Index(htmlStr[actionStart:], `"`); actionEnd != -1 {
					response.RedirectURL = htmlStr[actionStart : actionStart+actionEnd]
				}
			}
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

		// Extract redirect URL from form action
		if actionStart := strings.Index(responseStr, `action="`); actionStart != -1 {
			actionStart += 8 // len(`action="`)
			if actionEnd := strings.Index(responseStr[actionStart:], `"`); actionEnd != -1 {
				response.RedirectURL = responseStr[actionStart : actionStart+actionEnd]
			}
		}
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

// timePtr returns a pointer to the given time
func timePtr(t time.Time) *time.Time {
	return &t
}
