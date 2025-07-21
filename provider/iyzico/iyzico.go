package iyzico

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// API URLs
	apiSandboxURL    = "https://sandbox-api.iyzipay.com"
	apiProductionURL = "https://api.iyzipay.com"

	// API Endpoints
	endpointPayment    = "/payment/auth"
	endpoint3DInit     = "/payment/3dsecure/initialize"
	endpoint3DComplete = "/payment/3dsecure/auth"
	endpointCancel     = "/payment/cancel"
	endpointRefund     = "/payment/refund"
	endpointRetrieve   = "/payment/detail"

	// İyzico Status Codes
	statusSuccess = "success"
	statusFailure = "failure"

	// İyzico Error Codes
	errorCodeNotEnoughMoney      = "5006"
	errorCodeInvalidCard         = "5007"
	errorCodeFraudulent          = "5208"
	errorCodeInsufficientBalance = "5053"

	// Default Values
	defaultLocale         = "tr"
	defaultIdentityNumber = "74300864791" // Test identity number
	defaultItemType       = "PHYSICAL"
	defaultRegisterCard   = 0
	defaultTimeout        = 30 * time.Second
)

// IyzicoProvider implements the provider.PaymentProvider interface for Iyzico
type IyzicoProvider struct {
	apiKey       string
	secretKey    string
	baseURL      string
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
	client       *http.Client
	logID        int64
}

// NewProvider creates a new Iyzico payment provider
func NewProvider() provider.PaymentProvider {
	return &IyzicoProvider{}
}

// GetRequiredConfig returns the configuration fields required for Iyzico
func (p *IyzicoProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "apiKey",
			Required:    true,
			Type:        "string",
			Description: "Iyzico API Key (found in Iyzico merchant panel)",
			Example:     "sandbox-BIOoONNaqF8UZZmP3...",
			MinLength:   20,
			MaxLength:   200,
		},
		{
			Key:         "secretKey",
			Required:    true,
			Type:        "string",
			Description: "Iyzico Secret Key (found in Iyzico merchant panel)",
			Example:     "sandbox-NjQwOTRkMDBkZmE1...",
			MinLength:   20,
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

// ValidateConfig validates the provided configuration against Iyzico requirements
func (p *IyzicoProvider) ValidateConfig(config map[string]string) error {
	requiredFields := p.GetRequiredConfig(config["environment"])
	return provider.ValidateConfigFields("iyzico", config, requiredFields)
}

// Initialize sets up the Iyzico payment provider with authentication credentials
func (p *IyzicoProvider) Initialize(conf map[string]string) error {
	p.apiKey = conf["apiKey"]
	p.secretKey = conf["secretKey"]

	if p.apiKey == "" || p.secretKey == "" {
		return errors.New("iyzico: apiKey and secretKey are required")
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

// CreatePayment makes a non-3D payment request
func (p *IyzicoProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("iyzico: invalid payment request: %w", err)
	}

	iyzicoReq := p.mapToIyzicoPaymentRequest(request, false)
	return p.sendPaymentRequest(ctx, endpointPayment, iyzicoReq)
}

// Create3DPayment starts a 3D secure payment process
func (p *IyzicoProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("iyzico: invalid 3D payment request: %w", err)
	}

	iyzicoReq := p.mapToIyzicoPaymentRequest(request, true)
	return p.sendPaymentRequest(ctx, endpoint3DInit, iyzicoReq)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *IyzicoProvider) Complete3DPayment(ctx context.Context, paymentID string, conversationID string, data map[string]string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("iyzico: paymentID is required for 3D completion")
	}

	req := map[string]any{
		"paymentId":      paymentID,
		"conversationId": conversationID,
		"locale":         defaultLocale,
	}

	// Add additional callback data received from 3D payment page
	for k, v := range data {
		if k != "paymentId" && k != "conversationId" && k != "locale" {
			req[k] = v
		}
	}

	return p.sendPaymentRequest(ctx, endpoint3DComplete, req)
}

// GetPaymentStatus retrieves the current status of a payment
func (p *IyzicoProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("iyzico: paymentID is required")
	}

	req := map[string]any{
		"paymentId":      paymentID,
		"locale":         defaultLocale,
		"conversationId": uuid.New().String(),
	}

	return p.sendPaymentRequest(ctx, endpointRetrieve, req)
}

// CancelPayment cancels a payment
func (p *IyzicoProvider) CancelPayment(ctx context.Context, paymentID string, reason string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("iyzico: paymentID is required")
	}

	req := map[string]any{
		"paymentId":      paymentID,
		"ip":             "127.0.0.1", // Usually this would come from the client
		"locale":         defaultLocale,
		"conversationId": uuid.New().String(),
	}

	if reason != "" {
		req["reason"] = reason
		req["description"] = reason
	}

	return p.sendPaymentRequest(ctx, endpointCancel, req)
}

// RefundPayment issues a refund for a payment
func (p *IyzicoProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("iyzico: paymentID is required for refund")
	}

	req := map[string]any{
		"paymentTransactionId": request.PaymentID,
		"locale":               defaultLocale,
		"ip":                   "127.0.0.1",
		"conversationId":       request.ConversationID,
	}

	if request.ConversationID == "" {
		req["conversationId"] = uuid.New().String()
	}

	if request.RefundAmount > 0 {
		req["price"] = fmt.Sprintf("%.2f", request.RefundAmount)
	}

	if request.Reason != "" {
		req["reason"] = request.Reason
	}

	if request.Description != "" {
		req["description"] = request.Description
	}

	resp, err := p.sendRequest(ctx, endpointRefund, req)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	// Parse the Iyzico response into RefundResponse
	refundResp := &provider.RefundResponse{
		Success:      resp["status"] == statusSuccess,
		PaymentID:    request.PaymentID,
		RefundAmount: request.RefundAmount,
		SystemTime:   &now,
		RawResponse:  resp,
	}

	if refundID, ok := resp["paymentTransactionId"].(string); ok {
		refundResp.RefundID = refundID
	}

	if resp["status"] != statusSuccess {
		refundResp.ErrorCode = fmt.Sprintf("%v", resp["errorCode"])
		refundResp.Message = fmt.Sprintf("%v", resp["errorMessage"])
	} else {
		refundResp.Status = "success"
		refundResp.Message = "Refund successful"
	}

	return refundResp, nil
}

// ValidateWebhook validates an incoming webhook notification
// Note: İyzico doesn't provide a specific webhook validation mechanism
// This implementation validates by querying the payment status
func (p *IyzicoProvider) ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	paymentID, ok := data["paymentId"]
	if !ok {
		return false, nil, errors.New("iyzico: missing paymentId in webhook data")
	}

	response, err := p.GetPaymentStatus(ctx, paymentID)
	if err != nil {
		return false, nil, fmt.Errorf("iyzico: failed to validate webhook: %w", err)
	}

	return response.Success, map[string]string{
		"status":    string(response.Status),
		"paymentId": response.PaymentID,
		"message":   response.Message,
	}, nil
}

// validatePaymentRequest validates the payment request
func (p *IyzicoProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
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

// mapToIyzicoPaymentRequest creates the Iyzico payment request structure
func (p *IyzicoProvider) mapToIyzicoPaymentRequest(request provider.PaymentRequest, is3D bool) map[string]any {
	// Create a unique conversation ID if not provided
	conversationID := request.ConversationID
	if conversationID == "" {
		conversationID = uuid.New().String()
	}

	// Use default locale if not provided
	locale := request.Locale
	if locale == "" {
		locale = defaultLocale
	}

	// Format price as string with 2 decimal places
	priceStr := fmt.Sprintf("%.2f", request.Amount)

	// Set default installment count if not provided
	installmentCount := request.InstallmentCount
	if installmentCount < 1 {
		installmentCount = 1
	}

	// Basic payment request structure
	req := map[string]any{
		"locale":         locale,
		"conversationId": conversationID,
		"price":          priceStr,
		"paidPrice":      priceStr,
		"currency":       request.Currency,
		"installment":    installmentCount,
		"paymentChannel": request.PaymentChannel,
		"paymentGroup":   request.PaymentGroup,
	}

	// Add basket items if available
	if len(request.Items) > 0 {
		basketItems := make([]map[string]any, len(request.Items))
		for i, item := range request.Items {
			basketItems[i] = map[string]any{
				"id":        item.ID,
				"name":      item.Name,
				"category1": item.Category,
				"itemType":  "PHYSICAL",
				"price":     fmt.Sprintf("%.2f", item.Price),
			}
		}
		req["basketItems"] = basketItems
	}

	// Add buyer information
	buyerIP := request.Customer.IPAddress
	if buyerIP == "" {
		buyerIP = request.ClientIP
	}
	if buyerIP == "" {
		buyerIP = "127.0.0.1" // Default IP
	}

	buyerID := request.Customer.ID
	if buyerID == "" {
		buyerID = uuid.New().String()
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	// Address fields with nil check
	var regAddress, city, country, zipCode string
	if request.Customer.Address != nil {
		regAddress = request.Customer.Address.Address
		city = request.Customer.Address.City
		country = request.Customer.Address.Country
		zipCode = request.Customer.Address.ZipCode
	}

	req["buyer"] = map[string]any{
		"id":                  buyerID,
		"name":                request.Customer.Name,
		"surname":             request.Customer.Surname,
		"gsmNumber":           request.Customer.PhoneNumber,
		"email":               request.Customer.Email,
		"identityNumber":      defaultIdentityNumber, // Test identity number
		"lastLoginDate":       now,
		"registrationDate":    now,
		"registrationAddress": regAddress,
		"ip":                  buyerIP,
		"city":                city,
		"country":             country,
		"zipCode":             zipCode,
	}

	// Add shipping address
	shippingAddress := map[string]any{
		"contactName": request.Customer.Name + " " + request.Customer.Surname,
		"address":     regAddress,
		"city":        city,
		"country":     country,
		"zipCode":     zipCode,
	}
	if request.Customer.Address != nil {
		shippingAddress["address"] = request.Customer.Address.Address
		shippingAddress["city"] = request.Customer.Address.City
		shippingAddress["country"] = request.Customer.Address.Country
		shippingAddress["zipCode"] = request.Customer.Address.ZipCode
	}
	req["shippingAddress"] = shippingAddress

	// Add billing address
	billingAddress := map[string]any{
		"contactName": request.Customer.Name + " " + request.Customer.Surname,
		"address":     regAddress,
		"city":        city,
		"country":     country,
		"zipCode":     zipCode,
	}
	if request.Customer.Address != nil {
		billingAddress["address"] = request.Customer.Address.Address
		billingAddress["city"] = request.Customer.Address.City
		billingAddress["country"] = request.Customer.Address.Country
		billingAddress["zipCode"] = request.Customer.Address.ZipCode
	}
	req["billingAddress"] = billingAddress

	// Add payment card information
	req["paymentCard"] = map[string]any{
		"cardHolderName": request.CardInfo.CardHolderName,
		"cardNumber":     request.CardInfo.CardNumber,
		"expireMonth":    request.CardInfo.ExpireMonth,
		"expireYear":     request.CardInfo.ExpireYear,
		"cvc":            request.CardInfo.CVV,
		"registerCard":   defaultRegisterCard,
	}

	// Add 3D specific fields
	if is3D {
		// Build GoPay's own callback URL with user's original callback URL as parameter
		if request.CallbackURL != "" {
			gopayCallbackURL := fmt.Sprintf("%s/v1/callback/iyzico?originalCallbackUrl=%s",
				p.gopayBaseURL,
				request.CallbackURL)
			// Add tenant ID to callback URL for proper tenant identification
			if request.TenantID != 0 {
				gopayCallbackURL += fmt.Sprintf("&tenantId=%d", request.TenantID)
			}
			req["callbackUrl"] = gopayCallbackURL
		} else {
			// If no callback URL provided, use GoPay's callback without redirect
			gopayCallbackURL := fmt.Sprintf("%s/v1/callback/iyzico", p.gopayBaseURL)
			if request.TenantID != 0 {
				gopayCallbackURL += fmt.Sprintf("?tenantId=%d", request.TenantID)
			}
			req["callbackUrl"] = gopayCallbackURL
		}
	}

	return req
}

// sendPaymentRequest sends payment requests to Iyzico and maps the response
func (p *IyzicoProvider) sendPaymentRequest(ctx context.Context, endpoint string, requestData map[string]any) (*provider.PaymentResponse, error) {
	now := time.Now()
	resp, err := p.sendRequest(ctx, endpoint, requestData)
	if err != nil {
		return nil, err
	}

	// add provider request to client request
	_ = provider.AddProviderRequestToClientRequest("iyzico", "providerRequest", requestData, p.logID)

	// Map Iyzico response to our common PaymentResponse
	paymentResp := &provider.PaymentResponse{
		Success:          resp["status"] == statusSuccess,
		SystemTime:       &now,
		ProviderResponse: resp,
	}

	// Extract payment info based on response
	if resp["status"] == statusSuccess {
		paymentResp.Status = provider.StatusSuccessful
		paymentResp.Message = "Payment successful"

		// Extract payment ID
		if paymentID, ok := resp["paymentId"].(string); ok {
			paymentResp.PaymentID = paymentID
		}

		// Extract transaction ID
		if transactionID, ok := resp["paymentTransactionId"].(string); ok {
			paymentResp.TransactionID = transactionID
		}

		// Extract fraud status
		if fraudStatus, ok := resp["fraudStatus"].(float64); ok {
			paymentResp.FraudStatus = int(fraudStatus)
		}

		// If this is a 3D response with HTML content
		if htmlContent, ok := resp["threeDSHtmlContent"].(string); ok && htmlContent != "" {
			paymentResp.Status = provider.StatusPending
			paymentResp.HTML = htmlContent
			paymentResp.Message = "3D Secure authentication required"
		}

		// Check for redirect URL in 3D response
		if redirectURL, ok := resp["redirectUrl"].(string); ok && redirectURL != "" {
			paymentResp.RedirectURL = redirectURL
		}
	} else {
		paymentResp.Status = provider.StatusFailed
		if errorCode, ok := resp["errorCode"].(string); ok {
			paymentResp.ErrorCode = errorCode
		}
		if errorMessage, ok := resp["errorMessage"].(string); ok {
			paymentResp.Message = errorMessage
		} else {
			paymentResp.Message = "Payment failed"
		}
	}

	// Parse the amount if available
	if price, ok := resp["price"].(string); ok {
		if priceFloat, err := parseFloat(price); err == nil {
			paymentResp.Amount = priceFloat
		}
	} else if paidPrice, ok := resp["paidPrice"].(string); ok {
		if priceFloat, err := parseFloat(paidPrice); err == nil {
			paymentResp.Amount = priceFloat
		}
	}

	// Extract currency
	if currency, ok := resp["currency"].(string); ok {
		paymentResp.Currency = currency
	}

	return paymentResp, nil
}

// sendRequest sends a request to Iyzico API
func (p *IyzicoProvider) sendRequest(ctx context.Context, endpoint string, requestData map[string]any) (map[string]any, error) {
	// Add some default values if not present
	if _, ok := requestData["locale"]; !ok {
		requestData["locale"] = defaultLocale
	}

	// Generate random conversation ID if not present
	if _, ok := requestData["conversationId"]; !ok {
		requestData["conversationId"] = uuid.New().String()
	}

	// Convert request data to JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+endpoint, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Calculate and set authentication headers
	authStr := p.generateAuthString(endpoint, string(jsonData))
	req.Header.Set("Authorization", authStr)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response as JSON
	var responseData map[string]any
	if err := json.Unmarshal(body, &responseData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return responseData, nil
}

// generateAuthString generates Iyzico authentication string using HMAC-SHA1
func (p *IyzicoProvider) generateAuthString(uri string, body string) string {
	// Calculate HMAC-SHA1 signature
	hash := hmac.New(sha1.New, []byte(p.secretKey))
	hash.Write([]byte(p.apiKey + uri + sortAndConcatRequest(body) + p.secretKey))
	hmacDigest := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	// Return formatted authorization header
	return fmt.Sprintf("IYZWS %s:%s", p.apiKey, hmacDigest)
}

// sortAndConcatRequest sorts and concatenates request fields for HMAC calculation
func sortAndConcatRequest(jsonString string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonString), &data); err != nil {
		return ""
	}

	// Get all keys and sort them
	var keys []string
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Concatenate key=value pairs
	var result string
	for _, key := range keys {
		value := fmt.Sprintf("%v", data[key])
		if value != "" && value != "[]" && value != "{}" {
			result += key + value
		}
	}

	return result
}

// parseFloat parses float values from string, handling comma decimal separators
func parseFloat(value string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
}
