package ozanpay

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// API URLs - Updated to match documentation
	apiSandboxURL    = "https://api-sandbox.ozan.com/api/v3"
	apiProductionURL = "https://api.ozan.com/api/v3"

	// API Endpoints - Fixed to match documentation
	endpointPurchase = "/purchase"
	endpointRefund   = "/refund"
	endpointStatus   = "/status"
	endpointCancel   = "/cancel"
	endpointCapture  = "/capture"

	// OzanPay Status Codes - Updated to match documentation
	statusApproved  = "APPROVED"
	statusPending   = "PENDING"
	statusWaiting   = "WAITING"
	statusDeclined  = "DECLINED"
	statusFailed    = "FAILED"
	statusCancelled = "CANCELLED"
	statusRefunded  = "REFUNDED"

	// Default Values
	defaultTimeout = 30 * time.Second
)

// OzanPayProvider implements the provider.PaymentProvider interface for OzanPay
type OzanPayProvider struct {
	apiKey       string
	secretKey    string // Used for checksum verification
	providerKey  string // Provider API Key from OzanPay
	baseURL      string
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
	client       *http.Client
	logID        int64
}

// NewProvider creates a new OzanPay payment provider
func NewProvider() provider.PaymentProvider {
	return &OzanPayProvider{}
}

// GetRequiredConfig returns the configuration fields required for OzanPay
func (p *OzanPayProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "apiKey",
			Required:    true,
			Type:        "string",
			Description: "OzanPay API Key (provided by OzanPay)",
			Example:     "OZANPAY_API_KEY_123",
			MinLength:   10,
			MaxLength:   100,
		},
		{
			Key:         "secretKey",
			Required:    true,
			Type:        "string",
			Description: "OzanPay Secret Key (provided by OzanPay)",
			Example:     "OZANPAY_SECRET_KEY_456",
			MinLength:   10,
			MaxLength:   100,
		},
		{
			Key:         "merchantId",
			Required:    true,
			Type:        "string",
			Description: "OzanPay Merchant ID (provided by OzanPay)",
			Example:     "MERCHANT123456",
			MinLength:   5,
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

// ValidateConfig validates the provided configuration against OzanPay requirements
func (p *OzanPayProvider) ValidateConfig(config map[string]string) error {
	requiredFields := p.GetRequiredConfig(config["environment"])
	return provider.ValidateConfigFields("ozanpay", config, requiredFields)
}

// Initialize sets up the OzanPay payment provider with authentication credentials
func (p *OzanPayProvider) Initialize(conf map[string]string) error {
	p.apiKey = conf["apiKey"]
	p.secretKey = conf["secretKey"]
	p.providerKey = conf["providerKey"]

	if p.apiKey == "" {
		return errors.New("ozanpay: apiKey is required")
	}

	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	p.isProduction = conf["environment"] == "production"
	if p.isProduction {
		p.baseURL = apiProductionURL
		// Production environment - use secure TLS
		p.client = &http.Client{
			Timeout: defaultTimeout,
		}
	} else {
		p.baseURL = apiSandboxURL
		// Sandbox environment - skip TLS verification for test endpoints
		p.client = &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}

	return nil
}

// GetInstallmentCount returns the installment count for a payment
func (p *OzanPayProvider) GetInstallmentCount(ctx context.Context, request provider.InstallmentInquireRequest) (provider.InstallmentInquireResponse, error) {
	return provider.InstallmentInquireResponse{}, nil
}

// GetCommission returns the commission for a payment
func (p *OzanPayProvider) GetCommission(ctx context.Context, request provider.CommissionRequest) (provider.CommissionResponse, error) {
	return provider.CommissionResponse{}, nil
}

// CreatePayment makes a non-3D payment request
func (p *OzanPayProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("ozanpay: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *OzanPayProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("ozanpay: invalid 3D payment request: %w", err)
	}

	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *OzanPayProvider) Complete3DPayment(ctx context.Context, callbackState *provider.CallbackState, data map[string]string) (*provider.PaymentResponse, error) {
	if callbackState.PaymentID == "" {
		return nil, errors.New("ozanpay: paymentID is required for 3D completion")
	}

	// For OzanPay, 3D completion is handled via status check
	// The payment should be completed automatically after 3D authentication
	return p.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: callbackState.PaymentID})
}

// GetPaymentStatus retrieves the current status of a payment
func (p *OzanPayProvider) GetPaymentStatus(ctx context.Context, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("ozanpay: paymentID is required")
	}

	// Prepare status request according to documentation
	statusData := map[string]any{
		"apiKey":        p.apiKey,
		"transactionId": request.PaymentID,
	}

	// Send request to get payment status
	response, err := p.sendRequest(ctx, endpointStatus, http.MethodPost, statusData)
	if err != nil {
		return nil, err
	}

	// Map OzanPay response to our common PaymentResponse
	return p.mapToPaymentResponse(response)
}

// CancelPayment cancels a payment using OzanPay's cancel endpoint
func (p *OzanPayProvider) CancelPayment(ctx context.Context, request provider.CancelRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("ozanpay: paymentID is required")
	}

	// Prepare cancel request according to documentation
	cancelData := map[string]any{
		"apiKey":        p.apiKey,
		"transactionId": request.PaymentID,
	}

	// Send cancel request
	response, err := p.sendRequest(ctx, endpointCancel, http.MethodPost, cancelData)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	// Convert response to payment response
	paymentResp := &provider.PaymentResponse{
		Success:          response["status"] == statusApproved,
		Status:           provider.StatusCancelled,
		PaymentID:        request.PaymentID,
		ProviderResponse: response,
		SystemTime:       &now,
	}

	if response["status"] == statusApproved {
		paymentResp.Message = "Payment cancelled successfully"
	} else {
		paymentResp.Success = false
		if errMsg, ok := response["message"].(string); ok {
			paymentResp.Message = errMsg
		} else {
			paymentResp.Message = "Cancellation failed"
		}
		if errCode, ok := response["code"].(string); ok {
			paymentResp.ErrorCode = errCode
		}
	}

	return paymentResp, nil
}

// RefundPayment issues a refund for a payment
func (p *OzanPayProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	now := time.Now()
	if request.PaymentID == "" {
		return nil, errors.New("ozanpay: paymentID is required for refund")
	}

	// Prepare refund request data according to documentation
	refundData := map[string]any{
		"apiKey":        p.apiKey,
		"transactionId": request.PaymentID,
		"referenceNo":   request.PaymentID,                 // Use payment ID as reference
		"amount":        int64(request.RefundAmount * 100), // Convert to minor units (cents)
		"currency":      request.Currency,
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
		SystemTime:   &now,
		RawResponse:  response,
	}

	// Extract refund ID (transaction ID from response)
	if transactionID, ok := response["transactionId"].(string); ok {
		refundResp.RefundID = transactionID
	}

	// Handle error responses
	if response["status"] != statusApproved {
		refundResp.Success = false
		if errMsg, ok := response["message"].(string); ok && errMsg != "" {
			refundResp.Message = errMsg
		} else {
			refundResp.Message = "Refund failed"
		}
		if errCode, ok := response["code"].(string); ok && errCode != "" {
			refundResp.ErrorCode = errCode
		}
	} else {
		refundResp.Status = "success"
		refundResp.Message = "Refund successful"
	}

	return refundResp, nil
}

// ValidateWebhook validates an incoming webhook notification using OzanPay checksum verification
func (p *OzanPayProvider) ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	// Check for checksum in the data (OzanPay sends checksum as part of response data)
	checksumFromOzan, ok := data["checksum"]
	if !ok {
		return false, nil, errors.New("missing checksum in webhook data")
	}

	// Extract required fields for checksum verification according to documentation
	referenceNo, ok := data["referenceNo"]
	if !ok {
		return false, nil, errors.New("missing referenceNo in webhook data")
	}

	amount, ok := data["amount"]
	if !ok {
		return false, nil, errors.New("missing amount in webhook data")
	}

	currency, ok := data["currency"]
	if !ok {
		return false, nil, errors.New("missing currency in webhook data")
	}

	status, ok := data["status"]
	if !ok {
		return false, nil, errors.New("missing status in webhook data")
	}

	message, ok := data["message"]
	if !ok {
		return false, nil, errors.New("missing message in webhook data")
	}

	code, ok := data["code"]
	if !ok {
		return false, nil, errors.New("missing code in webhook data")
	}

	// Generate checksum according to OzanPay documentation
	// toString = referenceNo + amount + currency + status + message + code + secretKey
	toString := referenceNo + amount + currency + status + message + code + p.secretKey

	// Calculate SHA256 hash
	hash := sha256.Sum256([]byte(toString))
	expectedChecksum := hex.EncodeToString(hash[:])

	// Compare checksums
	if checksumFromOzan != expectedChecksum {
		return false, nil, errors.New("invalid webhook checksum")
	}

	// Extract payment details
	result := make(map[string]string)
	if transactionID, ok := data["transactionId"]; ok {
		result["paymentId"] = transactionID
	}
	result["status"] = status
	result["referenceNo"] = referenceNo
	result["amount"] = amount
	result["currency"] = currency

	return true, result, nil
}

// validatePaymentRequest validates the payment request
func (p *OzanPayProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
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
	response, err := p.sendRequest(ctx, endpointPurchase, http.MethodPost, paymentData)
	if err != nil {
		return nil, err
	}

	// add provider request to client request
	_ = provider.AddProviderRequestToClientRequest("ozanpay", "providerRequest", paymentData, p.logID)

	// Map response to common format
	return p.mapToPaymentResponse(response)
}

// Helper method to map our common request to OzanPay format according to documentation
func (p *OzanPayProvider) mapToOzanPayRequest(request provider.PaymentRequest, force3D bool) map[string]any {
	// Format amount - OzanPay expects amount in minor units (cents)
	amountInMinorUnits := int64(request.Amount * 100)

	// Build payment request according to OzanPay API documentation
	paymentReq := map[string]any{
		"apiKey":           p.apiKey,
		"amount":           amountInMinorUnits,
		"currency":         request.Currency,
		"number":           request.CardInfo.CardNumber,
		"expiryMonth":      request.CardInfo.ExpireMonth,
		"expiryYear":       request.CardInfo.ExpireYear,
		"cvv":              request.CardInfo.CVV,
		"referenceNo":      generateReferenceNo(),
		"description":      request.Description,
		"billingFirstName": request.Customer.Name,
		"billingLastName":  request.Customer.Surname,
		"email":            request.Customer.Email,
	}

	// Add address fields if address is provided
	if request.Customer.Address != nil {
		paymentReq["billingAddress1"] = request.Customer.Address.Address
		paymentReq["billingCountry"] = request.Customer.Address.Country
		paymentReq["billingCity"] = request.Customer.Address.City
		paymentReq["billingPostcode"] = request.Customer.Address.ZipCode
	} else {
		// Provide default address values if not provided
		paymentReq["billingAddress1"] = "N/A"
		paymentReq["billingCountry"] = "TR"
		paymentReq["billingCity"] = "Istanbul"
		paymentReq["billingPostcode"] = "34000"
	}

	// Add provider key if available
	if p.providerKey != "" {
		paymentReq["providerKey"] = p.providerKey
	}

	// Add optional phone number
	if request.Customer.PhoneNumber != "" {
		paymentReq["billingPhone"] = request.Customer.PhoneNumber
	}

	// Add optional company (not available in Customer struct, use empty string)

	// Add 3D secure settings
	if force3D || request.Use3D {
		paymentReq["is3d"] = true

		// Build return URL with GoPay callback
		if request.CallbackURL != "" {
			gopayCallbackURL := fmt.Sprintf("%s/v1/callback/ozanpay?originalCallbackUrl=%s",
				p.gopayBaseURL,
				request.CallbackURL)
			// Add tenant ID to callback URL for proper tenant identification
			if request.TenantID != 0 {
				gopayCallbackURL += fmt.Sprintf("&tenantId=%d", request.TenantID)
			}
			paymentReq["returnUrl"] = gopayCallbackURL
		} else {
			// If no callback URL provided, use GoPay's callback without redirect
			gopayCallbackURL := fmt.Sprintf("%s/v1/callback/ozanpay", p.gopayBaseURL)
			if request.TenantID != 0 {
				gopayCallbackURL += fmt.Sprintf("?tenantId=%d", request.TenantID)
			}
			paymentReq["returnUrl"] = gopayCallbackURL
		}
	} else {
		paymentReq["is3d"] = false
	}

	// Add customer IP if available
	if request.Customer.IPAddress != "" {
		paymentReq["customerIp"] = request.Customer.IPAddress
	}

	// Add customer user agent if available
	if request.ClientUserAgent != "" {
		paymentReq["customerUserAgent"] = request.ClientUserAgent
	}

	// Add browser info for 3D secure (required for 3D payments)
	if force3D || request.Use3D {
		browserInfo := map[string]any{
			"language":     "en-US", // Default value
			"colorDepth":   24,      // Default value
			"screenHeight": 900,     // Default value
			"screenWidth":  1440,    // Default value
			"screenTZ":     "-180",  // Default value
			"javaEnabled":  false,   // Default value
			"acceptHeader": "/",     // Default value
		}

		paymentReq["browserInfo"] = browserInfo
	}

	// Add basket items (required according to documentation)
	if len(request.Items) > 0 {
		basketItems := make([]map[string]any, len(request.Items))
		for i, item := range request.Items {
			basketItems[i] = map[string]any{
				"name":        item.Name,
				"description": item.Description,
				"category":    getItemCategory(item), // Default or from metadata
				"extraField":  "",                    // Optional field
				"quantity":    item.Quantity,
				"unitPrice":   int64(item.Price * 100), // Price in minor units
			}
		}
		paymentReq["basketItems"] = basketItems
	} else {
		// Create default basket item if none provided (required for OzanPay)
		defaultItem := map[string]any{
			"name":        "Payment",
			"description": request.Description,
			"category":    "General",
			"extraField":  "",
			"quantity":    1,
			"unitPrice":   amountInMinorUnits,
		}
		paymentReq["basketItems"] = []map[string]any{defaultItem}
	}

	return paymentReq
}

// Helper function to generate reference number
func generateReferenceNo() string {
	return fmt.Sprintf("gopay-%d-%s", time.Now().Unix(), uuid.New().String()[:8])
}

// Helper function to get item category
func getItemCategory(item provider.Item) string {
	// Use the Category field from Item struct if available
	if item.Category != "" {
		return item.Category
	}
	return "General" // Default category
}

// Helper method to map OzanPay response to our common format
func (p *OzanPayProvider) mapToPaymentResponse(response map[string]any) (*provider.PaymentResponse, error) {
	now := time.Now()
	paymentResp := &provider.PaymentResponse{
		ProviderResponse: response,
		SystemTime:       &now,
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
		paymentResp.Success = (status == statusApproved)

		// Map OzanPay status to our common status according to documentation
		switch status {
		case statusApproved:
			paymentResp.Status = provider.StatusSuccessful
			paymentResp.Message = "Payment successful"
		case statusPending, statusWaiting:
			paymentResp.Status = provider.StatusPending
			paymentResp.Message = "Payment pending"
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
	if errMsg, ok := response["message"].(string); ok && errMsg != "" {
		paymentResp.Success = false
		paymentResp.Message = errMsg
	}

	// Extract error code if available
	if errCode, ok := response["code"].(string); ok && errCode != "" {
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

	// Set headers according to OzanPay documentation
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Note: OzanPay authentication is handled via apiKey parameter in the request body,
	// not through headers. No special authentication headers needed.

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
