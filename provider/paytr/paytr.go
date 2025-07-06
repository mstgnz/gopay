package paytr

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// API URLs
	apiSandboxURL    = "https://www.paytr.com"
	apiProductionURL = "https://www.paytr.com"

	// API Endpoints
	endpointIFrameToken     = "/odeme/api/get-token"
	endpointDirectPayment   = "/odeme"
	endpointPaymentStatus   = "/odeme/durum-sorgu"
	endpointRefund          = "/odeme/iade"
	endpointInstallmentRate = "/odeme/api/installment-rates"
	endpointBINQuery        = "/odeme/api/bin-detail"

	// PayTR Status Codes
	statusSuccess   = "success"
	statusFailed    = "failed"
	statusWaiting   = "waiting"
	statusPending   = "pending"
	statusCancelled = "cancelled"
	statusRefunded  = "refunded"

	// PayTR Error Codes
	errorCodeInsufficientFunds = "YETERSIZ_BAKIYE"
	errorCodeInvalidCard       = "GECERSIZ_KART"
	errorCodeExpiredCard       = "SURESI_GECMIS_KART"
	errorCodeFraudulent        = "SAHTEKARLIK_SUPTESI"
	errorCodeDeclined          = "KART_REDDEDILDI"
	errorCodeSystemError       = "SISTEM_HATASI"

	// Default Values
	defaultCurrency = "TL"
	defaultTimeout  = 30 * time.Second
	defaultLang     = "tr"
)

// PayTRProvider implements the provider.PaymentProvider interface for PayTR
type PayTRProvider struct {
	merchantID   string
	merchantKey  string
	merchantSalt string
	baseURL      string
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
	client       *http.Client
}

// NewProvider creates a new PayTR payment provider
func NewProvider() provider.PaymentProvider {
	return &PayTRProvider{
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Initialize sets up the PayTR payment provider with authentication credentials
func (p *PayTRProvider) Initialize(conf map[string]string) error {
	p.merchantID = conf["merchantId"]
	p.merchantKey = conf["merchantKey"]
	p.merchantSalt = conf["merchantSalt"]

	if p.merchantID == "" || p.merchantKey == "" || p.merchantSalt == "" {
		return errors.New("paytr: merchantId, merchantKey and merchantSalt are required")
	}

	// Set GoPay base URL for callbacks
	if gopayBaseURL, ok := conf["gopayBaseURL"]; ok && gopayBaseURL != "" {
		p.gopayBaseURL = gopayBaseURL
	} else {
		p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")
	}

	p.isProduction = conf["environment"] == "production"
	// PayTR uses the same base URL for both sandbox and production
	p.baseURL = apiProductionURL

	return nil
}

// CreatePayment makes a non-3D payment request (Direct API)
func (p *PayTRProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("paytr: invalid payment request: %w", err)
	}

	return p.processDirectPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process (iFrame API)
func (p *PayTRProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("paytr: invalid 3D payment request: %w", err)
	}

	return p.processIFramePayment(ctx, request)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *PayTRProvider) Complete3DPayment(ctx context.Context, paymentID, conversationID string, data map[string]string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("paytr: paymentID is required")
	}

	// For PayTR, 3D completion is handled via callback
	// We typically just need to verify the callback data and get payment status
	return p.GetPaymentStatus(ctx, paymentID)
}

// GetPaymentStatus retrieves the current status of a payment
func (p *PayTRProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("paytr: paymentID is required")
	}

	data := map[string]string{
		"merchant_id":  p.merchantID,
		"merchant_oid": paymentID,
	}

	// Generate token hash for status inquiry
	tokenHash := p.generateStatusQueryHash(paymentID)
	data["paytr_token"] = tokenHash

	response, err := p.sendRequest(ctx, endpointPaymentStatus, data)
	if err != nil {
		return nil, fmt.Errorf("paytr: payment status inquiry failed: %w", err)
	}

	return p.mapToPaymentResponse(response, paymentID), nil
}

// CancelPayment cancels a payment (PayTR handles this via refund with 0 commission)
func (p *PayTRProvider) CancelPayment(ctx context.Context, paymentID string, reason string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("paytr: paymentID is required")
	}

	// PayTR doesn't have a separate cancel endpoint
	// We need to use refund endpoint with full amount
	return nil, errors.New("paytr: payment cancellation is handled via refund operations")
}

// RefundPayment issues a refund for a payment
func (p *PayTRProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("paytr: paymentID is required for refund")
	}

	if request.RefundAmount <= 0 {
		return nil, errors.New("paytr: refund amount must be greater than 0")
	}

	// Convert amount to PayTR format (multiply by 100 for kuruş)
	refundAmountKurus := int64(request.RefundAmount * 100)

	data := map[string]string{
		"merchant_id":   p.merchantID,
		"merchant_oid":  request.PaymentID,
		"return_amount": strconv.FormatInt(refundAmountKurus, 10),
		"reference_no":  uuid.New().String(),
	}

	if request.Reason != "" {
		data["reason"] = request.Reason
	}

	// Generate token hash for refund
	tokenHash := p.generateRefundHash(request.PaymentID, refundAmountKurus)
	data["paytr_token"] = tokenHash

	response, err := p.sendRequest(ctx, endpointRefund, data)
	if err != nil {
		return nil, fmt.Errorf("paytr: refund failed: %w", err)
	}

	return p.mapToRefundResponse(response, request), nil
}

// ValidateWebhook validates an incoming webhook notification
func (p *PayTRProvider) ValidateWebhook(ctx context.Context, data, headers map[string]string) (bool, map[string]string, error) {
	// PayTR webhook validation using merchant_oid and hash
	merchantOid, ok := data["merchant_oid"]
	if !ok {
		return false, nil, errors.New("paytr: missing merchant_oid in webhook data")
	}

	status, ok := data["status"]
	if !ok {
		return false, nil, errors.New("paytr: missing status in webhook data")
	}

	totalAmount, ok := data["total_amount"]
	if !ok {
		return false, nil, errors.New("paytr: missing total_amount in webhook data")
	}

	hash, ok := data["hash"]
	if !ok {
		return false, nil, errors.New("paytr: missing hash in webhook data")
	}

	// Calculate expected hash
	expectedHash := p.generateWebhookHash(merchantOid, status, totalAmount)

	// Compare hashes
	if hash != expectedHash {
		return false, nil, errors.New("paytr: invalid webhook hash")
	}

	// Extract payment information
	result := map[string]string{
		"paymentId":     merchantOid,
		"status":        status,
		"totalAmount":   totalAmount,
		"transactionId": data["payment_id"], // PayTR's internal payment ID
	}

	// Add optional fields if present
	if errorMessage, ok := data["failed_reason_msg"]; ok {
		result["errorMessage"] = errorMessage
	}

	if errorCode, ok := data["failed_reason_code"]; ok {
		result["errorCode"] = errorCode
	}

	return true, result, nil
}

// processDirectPayment handles direct payment (Non-3D)
func (p *PayTRProvider) processDirectPayment(ctx context.Context, request provider.PaymentRequest, force3D bool) (*provider.PaymentResponse, error) {
	// PayTR Direct API requires different implementation
	// For now, we'll redirect to 3D secure as PayTR primarily uses iFrame
	return p.processIFramePayment(ctx, request)
}

// processIFramePayment handles iFrame payment (with 3D secure support)
func (p *PayTRProvider) processIFramePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	// Convert amount to PayTR format (multiply by 100 for kuruş)
	amountInKurus := int64(request.Amount * 100)

	// Generate merchant order ID if not provided
	merchantOid := request.ID
	if merchantOid == "" {
		merchantOid = uuid.New().String()
	}

	// Build payment data
	data := map[string]string{
		"merchant_id":       p.merchantID,
		"user_ip":           request.ClientIP,
		"merchant_oid":      merchantOid,
		"email":             request.Customer.Email,
		"payment_amount":    strconv.FormatInt(amountInKurus, 10),
		"currency":          p.getCurrency(request.Currency),
		"test_mode":         p.getTestMode(),
		"non_3d":            "0", // Force 3D secure
		"merchant_ok_url":   p.getCallbackURL(request.CallbackURL, "success", request.TenantID),
		"merchant_fail_url": p.getCallbackURL(request.CallbackURL, "fail", request.TenantID),
		"user_name":         fmt.Sprintf("%s %s", request.Customer.Name, request.Customer.Surname),
		"user_phone":        request.Customer.PhoneNumber,
	}

	// Add user basket (required by PayTR)
	userBasket := p.buildUserBasket(request.Items, request.Amount)
	data["user_basket"] = userBasket

	// Add user address if available
	if request.Customer.Address.Address != "" {
		data["user_address"] = fmt.Sprintf("%s, %s, %s", request.Customer.Address.Address, request.Customer.Address.City, request.Customer.Address.Country)
	}

	// Add installment if specified
	if request.InstallmentCount > 1 {
		data["installment_count"] = strconv.Itoa(request.InstallmentCount)
	}

	// Generate token hash
	tokenHash := p.generateTokenHash(data)
	data["paytr_token"] = tokenHash

	// Get iFrame token
	response, err := p.sendRequest(ctx, endpointIFrameToken, data)
	if err != nil {
		return nil, fmt.Errorf("paytr: failed to get iframe token: %w", err)
	}

	return p.mapToIFrameResponse(response, merchantOid), nil
}

// validatePaymentRequest validates the payment request
func (p *PayTRProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
	if request.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}

	if request.Customer.Email == "" {
		return errors.New("customer email is required")
	}

	if request.Customer.Name == "" || request.Customer.Surname == "" {
		return errors.New("customer name and surname are required")
	}

	if request.ClientIP == "" {
		return errors.New("client IP is required")
	}

	if is3D && request.CallbackURL == "" {
		return errors.New("callback URL is required for 3D payments")
	}

	return nil
}

// Helper methods

func (p *PayTRProvider) getCurrency(currency string) string {
	if currency == "" {
		return defaultCurrency
	}
	// PayTR supports TL, USD, EUR
	switch strings.ToUpper(currency) {
	case "TRY", "TL":
		return "TL"
	case "USD":
		return "USD"
	case "EUR":
		return "EUR"
	default:
		return defaultCurrency
	}
}

func (p *PayTRProvider) getTestMode() string {
	if p.isProduction {
		return "0"
	}
	return "1"
}

func (p *PayTRProvider) getCallbackURL(originalURL, status, tenantID string) string {
	if originalURL != "" {
		callbackURL := fmt.Sprintf("%s/v1/callback/paytr?originalCallbackUrl=%s&status=%s", p.gopayBaseURL, url.QueryEscape(originalURL), status)
		if tenantID != "" {
			callbackURL += fmt.Sprintf("&tenantId=%s", tenantID)
		}
		return callbackURL
	}
	callbackURL := fmt.Sprintf("%s/v1/callback/paytr?status=%s", p.gopayBaseURL, status)
	if tenantID != "" {
		callbackURL += fmt.Sprintf("&tenantId=%s", tenantID)
	}
	return callbackURL
}

func (p *PayTRProvider) buildUserBasket(items []provider.Item, totalAmount float64) string {
	if len(items) == 0 {
		// Create a default basket item
		return fmt.Sprintf(`[["Payment","%s","1"]]`, strconv.FormatFloat(totalAmount, 'f', 2, 64))
	}

	basket := make([][]string, 0, len(items))
	for _, item := range items {
		basket = append(basket, []string{
			item.Name,
			strconv.FormatFloat(item.Price, 'f', 2, 64),
			strconv.Itoa(item.Quantity),
		})
	}

	jsonData, _ := json.Marshal(basket)
	return string(jsonData)
}

// Hash generation methods

func (p *PayTRProvider) generateTokenHash(data map[string]string) string {
	// PayTR token hash: merchant_id + user_ip + merchant_oid + email + payment_amount + user_basket + no_installment + max_installment + currency + test_mode + merchant_salt
	hashStr := data["merchant_id"] + data["user_ip"] + data["merchant_oid"] + data["email"] +
		data["payment_amount"] + data["user_basket"] + "0" + "0" + data["currency"] + data["test_mode"] + p.merchantSalt

	return p.generateMD5Hash(hashStr)
}

func (p *PayTRProvider) generateStatusQueryHash(merchantOid string) string {
	// PayTR status query hash: merchant_id + merchant_oid + merchant_salt
	hashStr := p.merchantID + merchantOid + p.merchantSalt
	return p.generateMD5Hash(hashStr)
}

func (p *PayTRProvider) generateRefundHash(merchantOid string, refundAmount int64) string {
	// PayTR refund hash: merchant_id + merchant_oid + return_amount + merchant_salt
	hashStr := p.merchantID + merchantOid + strconv.FormatInt(refundAmount, 10) + p.merchantSalt
	return p.generateMD5Hash(hashStr)
}

func (p *PayTRProvider) generateWebhookHash(merchantOid, status, totalAmount string) string {
	// PayTR webhook hash: merchant_oid + merchant_salt + status + total_amount
	hashStr := merchantOid + p.merchantSalt + status + totalAmount
	return p.generateMD5Hash(hashStr)
}

func (p *PayTRProvider) generateMD5Hash(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Response mapping methods

func (p *PayTRProvider) mapToIFrameResponse(response map[string]any, merchantOid string) *provider.PaymentResponse {
	now := time.Now()
	paymentResp := &provider.PaymentResponse{
		PaymentID:        merchantOid,
		SystemTime:       &now,
		ProviderResponse: response,
	}

	if status, ok := response["status"].(string); ok {
		if status == "success" {
			if token, ok := response["token"].(string); ok {
				// PayTR returns a token that should be used to display the iframe
				paymentResp.Success = true
				paymentResp.Status = provider.StatusPending
				paymentResp.HTML = fmt.Sprintf(`<iframe src="https://www.paytr.com/odeme/guvenlik/%s" id="paytriframe" frameborder="0" scrolling="no" style="width: 100%%; height: 400px;"></iframe>`, token)
				paymentResp.RedirectURL = fmt.Sprintf("https://www.paytr.com/odeme/guvenlik/%s", token)
			}
		} else {
			paymentResp.Success = false
			paymentResp.Status = provider.StatusFailed
			if reason, ok := response["reason"].(string); ok {
				paymentResp.Message = reason
			}
		}
	}

	return paymentResp
}

func (p *PayTRProvider) mapToPaymentResponse(response map[string]any, paymentID string) *provider.PaymentResponse {
	now := time.Now()
	paymentResp := &provider.PaymentResponse{
		PaymentID:        paymentID,
		SystemTime:       &now,
		ProviderResponse: response,
	}

	if status, ok := response["status"].(string); ok {
		switch status {
		case "success":
			paymentResp.Success = true
			paymentResp.Status = provider.StatusSuccessful
		case "failed":
			paymentResp.Success = false
			paymentResp.Status = provider.StatusFailed
		case "waiting":
			paymentResp.Status = provider.StatusPending
		default:
			paymentResp.Status = provider.StatusFailed
		}
	}

	if amount, ok := response["payment_amount"].(string); ok {
		if amountFloat, err := strconv.ParseFloat(amount, 64); err == nil {
			paymentResp.Amount = amountFloat / 100 // Convert from kuruş to TL
		}
	}

	if currency, ok := response["currency"].(string); ok {
		paymentResp.Currency = currency
	}

	if message, ok := response["failed_reason_msg"].(string); ok {
		paymentResp.Message = message
	}

	if errorCode, ok := response["failed_reason_code"].(string); ok {
		paymentResp.ErrorCode = errorCode
	}

	if transactionID, ok := response["payment_id"].(string); ok {
		paymentResp.TransactionID = transactionID
	}

	return paymentResp
}

func (p *PayTRProvider) mapToRefundResponse(response map[string]any, request provider.RefundRequest) *provider.RefundResponse {
	now := time.Now()
	refundResp := &provider.RefundResponse{
		PaymentID:   request.PaymentID,
		SystemTime:  &now,
		RawResponse: response,
	}

	if status, ok := response["status"].(string); ok {
		refundResp.Success = status == "success"
		refundResp.Status = status

		if status == "success" {
			refundResp.RefundAmount = request.RefundAmount
		}
	}

	if refundID, ok := response["ref_id"].(string); ok {
		refundResp.RefundID = refundID
	}

	if message, ok := response["reason"].(string); ok {
		refundResp.Message = message
	}

	if errorCode, ok := response["err_no"].(string); ok && errorCode != "" {
		refundResp.ErrorCode = errorCode
		refundResp.Success = false
	}

	return refundResp
}

// sendRequest sends HTTP request to PayTR API
func (p *PayTRProvider) sendRequest(ctx context.Context, endpoint string, data map[string]string) (map[string]any, error) {
	// Convert data to URL-encoded form
	formData := url.Values{}
	for key, value := range data {
		formData.Set(key, value)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "GoPay/1.0")

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle non-success HTTP status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error: %d, response: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var responseData map[string]any
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return responseData, nil
}
