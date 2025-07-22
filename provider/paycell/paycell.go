package paycell

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// API URLs
	apiSandboxURL    = "https://tpay-test.turkcell.com.tr"
	apiProductionURL = "https://tpay.turkcell.com.tr"

	// Payment Management URLs (different domain for 3D secure)
	paymentManagementSandboxURL    = "https://omccstb.turkcell.com.tr"
	paymentManagementProductionURL = "https://epayment.turkcell.com.tr"

	// API Endpoints - Provision Services
	endpointProvision               = "/tpay/provision/services/restful/getCardToken/provision/"
	endpointInquire                 = "/tpay/provision/services/restful/getCardToken/inquire/"
	endpointReverse                 = "/tpay/provision/services/restful/getCardToken/reverse/"
	endpointRefund                  = "/tpay/provision/services/restful/getCardToken/refund/"
	endpointGetThreeDSession        = "/tpay/provision/services/restful/getCardToken/getThreeDSession/"
	endpointGetThreeDSessionResult  = "/tpay/provision/services/restful/getCardToken/getThreeDSessionResult/"
	endpointGetCards                = "/tpay/provision/services/restful/getCardToken/getCards/"
	endpointRegisterCard            = "/tpay/provision/services/restful/getCardToken/registerCard/"
	endpointUpdateCard              = "/tpay/provision/services/restful/getCardToken/updateCard/"
	endpointDeleteCard              = "/tpay/provision/services/restful/getCardToken/deleteCard/"
	endpointGetCardBinInformation   = "/tpay/provision/services/restful/getCardToken/getCardBinInformation/"
	endpointGetPaymentMethods       = "/tpay/provision/services/restful/getCardToken/getPaymentMethods/"
	endpointSummaryReconciliation   = "/tpay/provision/services/restful/getCardToken/summaryReconciliation/"
	endpointGetProvisionHistory     = "/tpay/provision/services/restful/getCardToken/getProvisionHistory/"
	endpointProvisionForMarketPlace = "/tpay/provision/services/restful/getCardToken/provisionForMarketPlace/"
	endpointOpenMobilePayment       = "/tpay/provision/services/restful/getCardToken/openMobilePayment/"
	endpointSendOTP                 = "/tpay/provision/services/restful/getCardToken/sendOTP/"
	endpointValidateOTP             = "/tpay/provision/services/restful/getCardToken/validateOTP/"
	endpointProvisionAll            = "/tpay/provision/services/restful/getCardToken/provisionAll/"
	endpointInquireAll              = "/tpay/provision/services/restful/getCardToken/inquireAll/"
	endpointRefundAll               = "/tpay/provision/services/restful/getCardToken/refundAll/"

	// Payment Management Endpoints (for 3D secure)
	endpointGetCardTokenSecure = "/paymentmanagement/rest/getCardTokenSecure"
	endpointThreeDSecure       = "/paymentmanagement/rest/threeDSecure"

	// Paycell Status Codes
	statusSuccess    = "SUCCESS"
	statusPending    = "PENDING"
	statusWaiting    = "WAITING"
	statusFailed     = "FAILED"
	statusCancelled  = "CANCELLED"
	statusRefunded   = "REFUNDED"
	statusProcessing = "PROCESSING"

	// Paycell Error Codes
	errorCodeInsufficientFunds = "INSUFFICIENT_FUNDS"
	errorCodeInvalidCard       = "INVALID_CARD"
	errorCodeExpiredCard       = "EXPIRED_CARD"
	errorCodeFraudulent        = "FRAUDULENT_TRANSACTION"
	errorCodeDeclined          = "CARD_DECLINED"
	errorCodeSystemError       = "SYSTEM_ERROR"

	// Paycell Response Codes
	responseCodeSuccess = "0"
	responseCodeError   = "1"

	// Default Values
	defaultCurrency = "TRY"
	defaultTimeout  = 30 * time.Second

	// Test constants from official PHP implementation
	testPrefix          = "666"
	testApplicationName = "PAYCELLTEST"
	testApplicationPwd  = "PaycellTestPassword"
	testSecureCode      = "PAYCELL12345"
	testMerchantCode    = "9998"
	testEulaID          = "17"
)

type TestCard struct {
	CardNumber  string
	ExpireMonth string
	ExpireYear  string
	CVV         string
}

var testCards = []TestCard{
	{
		CardNumber:  "4355084355084358",
		ExpireMonth: "12",
		ExpireYear:  "26",
		CVV:         "000",
	},
	{
		CardNumber:  "5571135571135575",
		ExpireMonth: "12",
		ExpireYear:  "26",
		CVV:         "000",
	},
	{
		CardNumber:  "4546711234567894",
		ExpireMonth: "12",
		ExpireYear:  "26",
		CVV:         "000",
	},
	{
		CardNumber:  "4508034508034509",
		ExpireMonth: "12",
		ExpireYear:  "26",
		CVV:         "000",
	},
	{
		CardNumber:  "5528790000000008",
		ExpireMonth: "12",
		ExpireYear:  "26",
		CVV:         "001",
	},
}

// PaycellProvider implements the provider.PaymentProvider interface for Paycell
type PaycellProvider struct {
	username             string
	password             string
	merchantID           string
	secureCode           string // Paycell secure code for hash generation
	baseURL              string
	paymentManagementURL string // For 3D secure operations
	gopayBaseURL         string // GoPay's own base URL for callbacks
	isProduction         bool
	logID                int64
	phoneNumber          string
	clientIP             string
	client               *http.Client
}

// NewProvider creates a new Paycell payment provider
func NewProvider() provider.PaymentProvider {
	return &PaycellProvider{}
}

// GetRequiredConfig returns the configuration fields required for Paycell
func (p *PaycellProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{
			Key:         "username",
			Required:    true,
			Type:        "string",
			Description: "Paycell API Username (provided by Paycell)",
			Example:     "PAYCELL_USERNAME",
			MinLength:   3,
			MaxLength:   50,
		},
		{
			Key:         "password",
			Required:    true,
			Type:        "string",
			Description: "Paycell API Password (provided by Paycell)",
			Example:     "PAYCELL_PASSWORD",
			MinLength:   6,
			MaxLength:   100,
		},
		{
			Key:         "merchantId",
			Required:    true,
			Type:        "string",
			Description: "Paycell Merchant ID (provided by Paycell)",
			Example:     "123456789",
			MinLength:   5,
			MaxLength:   20,
		},
		{
			Key:         "secureCode",
			Required:    true,
			Type:        "string",
			Description: "Paycell Secure Code (provided by Paycell)",
			Example:     "PAYCELL12345",
			MinLength:   10,
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

// ValidateConfig validates the provided configuration against Paycell requirements
func (p *PaycellProvider) ValidateConfig(config map[string]string) error {
	requiredFields := p.GetRequiredConfig(config["environment"])
	return provider.ValidateConfigFields("paycell", config, requiredFields)
}

// Initialize sets up the Paycell payment provider with authentication credentials
func (p *PaycellProvider) Initialize(conf map[string]string) error {
	p.isProduction = conf["environment"] == "production"

	if p.isProduction {
		// Production environment - use provided credentials
		p.username = conf["username"]
		p.password = conf["password"]
		p.merchantID = conf["merchantId"]
		p.secureCode = conf["secureCode"]

		if p.username == "" || p.password == "" || p.merchantID == "" || p.secureCode == "" {
			return errors.New("paycell: username, password, merchantId and secureCode are required for production")
		}

		p.baseURL = apiProductionURL
		p.paymentManagementURL = paymentManagementProductionURL
		// Production environment - use secure TLS
		p.client = &http.Client{
			Timeout: defaultTimeout,
		}
	} else {
		// Test environment - use test credentials for integration tests [[memory:2471205]]
		p.username = testApplicationName
		p.password = testApplicationPwd
		p.merchantID = testMerchantCode
		p.secureCode = testSecureCode

		fmt.Printf("Using test credentials - Username: %s, MerchantID: %s\n", p.username, p.merchantID)

		p.baseURL = apiSandboxURL
		p.paymentManagementURL = paymentManagementSandboxURL
		// Sandbox environment - skip TLS verification for test endpoints
		p.client = &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}

	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	return nil
}

// CreatePayment makes a non-3D payment request
func (p *PaycellProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.clientIP = request.ClientIP
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("paycell: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *PaycellProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	p.clientIP = request.ClientIP
	p.logID = request.LogID
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("paycell: invalid 3D payment request: %w", err)
	}

	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *PaycellProvider) Complete3DPayment(ctx context.Context, paymentID, conversationID string, data map[string]string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	// Get 3D session result after authentication
	threeDSessionID, ok := data["threeDSessionId"]
	if !ok || threeDSessionID == "" {
		return nil, errors.New("paycell: threeDSessionId is required")
	}

	endpoint := endpointGetThreeDSessionResult
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	requestHeader := PaycellRequestHeader{
		ApplicationName:     p.username,
		ApplicationPwd:      p.password,
		ClientIPAddress:     p.clientIP,
		TransactionDateTime: transactionDateTime,
		TransactionID:       transactionID,
	}

	paycellReq := PaycellGetThreeDSessionResultRequest{
		RequestHeader:   requestHeader,
		MerchantCode:    p.merchantID,
		MSISDN:          p.phoneNumber,
		ThreeDSessionID: threeDSessionID,
	}

	return p.sendProvisionRequest(ctx, endpoint, paycellReq)
}

// GetPaymentStatus retrieves the current status of a payment
func (p *PaycellProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	// get spesific key in log jsonb
	originalReferenceNumber, err := provider.GetProviderRequestFromLog("paycell", paymentID, "referenceNumber")
	if err != nil {
		return nil, fmt.Errorf("failed to get reference number: %w", err)
	}

	endpoint := endpointInquireAll
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	requestHeader := PaycellRequestHeader{
		ApplicationName:     p.username,
		ApplicationPwd:      p.password,
		ClientIPAddress:     p.clientIP,
		TransactionDateTime: transactionDateTime,
		TransactionID:       transactionID,
	}

	paycellReq := PaycellInquireRequest{
		RequestHeader:           requestHeader,
		OriginalReferenceNumber: originalReferenceNumber,
		ReferenceNumber:         p.generateReferenceNumber(),
		MerchantCode:            p.merchantID,
		MSISDN:                  p.phoneNumber,
		PaymentMethodType:       "CREDIT_CARD",
	}

	return p.sendProvisionRequest(ctx, endpoint, paycellReq)
}

// CancelPayment cancels a payment (reverse operation)
func (p *PaycellProvider) CancelPayment(ctx context.Context, paymentID, reason string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	endpoint := endpointReverse
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	requestHeader := PaycellRequestHeader{
		ApplicationName:     p.username,
		ApplicationPwd:      p.password,
		ClientIPAddress:     p.clientIP,
		TransactionDateTime: transactionDateTime,
		TransactionID:       transactionID,
	}

	paycellReq := PaycellReverseRequest{
		RequestHeader:           requestHeader,
		OriginalReferenceNumber: paymentID,
		ReferenceNumber:         p.generateReferenceNumber(),
		MerchantCode:            p.merchantID,
		PaymentType:             "REVERSE",
	}

	return p.sendProvisionRequest(ctx, endpoint, paycellReq)
}

// RefundPayment refunds a payment
func (p *PaycellProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	if request.RefundAmount <= 0 {
		return nil, errors.New("paycell: refund amount must be greater than 0")
	}

	endpoint := endpointRefund
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	requestHeader := PaycellRequestHeader{
		ApplicationName:     p.username,
		ApplicationPwd:      p.password,
		ClientIPAddress:     p.clientIP,
		TransactionDateTime: transactionDateTime,
		TransactionID:       transactionID,
	}

	paycellReq := PaycellRefundRequest{
		RequestHeader:           requestHeader,
		OriginalReferenceNumber: request.PaymentID,
		ReferenceNumber:         p.generateReferenceNumber(),
		MerchantCode:            p.merchantID,
		Amount:                  fmt.Sprintf("%.0f", request.RefundAmount*100), // Convert TL to kuruş (multiply by 100)
		PaymentType:             "REFUND",
	}

	response, err := p.sendProvisionRequest(ctx, endpoint, paycellReq)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &provider.RefundResponse{
		Success:      response.Success,
		RefundID:     response.TransactionID,
		PaymentID:    request.PaymentID,
		RefundAmount: request.RefundAmount,
		Status:       string(response.Status),
		Message:      response.Message,
		ErrorCode:    response.ErrorCode,
		SystemTime:   &now,
		RawResponse:  response,
	}, nil
}

// ValidateWebhook validates Paycell webhook data
func (p *PaycellProvider) ValidateWebhook(ctx context.Context, data, headers map[string]string) (bool, map[string]string, error) {
	// Paycell doesn't have webhook validation in the same way
	// This is more for completion callbacks
	return true, data, nil
}

// validatePaymentRequest validates payment request parameters
func (p *PaycellProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
	if request.TenantID == 0 {
		return errors.New("tenantID is required")
	}

	if request.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}

	if request.Currency == "" {
		return errors.New("currency is required")
	}

	if request.Customer.PhoneNumber == "" {
		return errors.New("customer phone number is required")
	}

	// Validate phone number format for Paycell (should be 10 digits without country code)
	phoneNumber := strings.TrimPrefix(request.Customer.PhoneNumber, "+90")
	phoneNumber = strings.TrimPrefix(phoneNumber, "90")
	if len(phoneNumber) != 10 {
		return errors.New("phone number must be 10 digits")
	}

	if request.CardInfo.CardNumber == "" {
		return errors.New("card number is required")
	}

	if request.CardInfo.ExpireMonth == "" || request.CardInfo.ExpireYear == "" {
		return errors.New("card expiry date is required")
	}

	if request.CardInfo.CVV == "" {
		return errors.New("card CVV is required")
	}

	if is3D && request.CallbackURL == "" {
		return errors.New("callback URL is required for 3D payments")
	}

	return nil
}

// processPayment handles the main payment processing logic
func (p *PaycellProvider) processPayment(ctx context.Context, request provider.PaymentRequest, is3D bool) (*provider.PaymentResponse, error) {
	p.phoneNumber = request.Customer.PhoneNumber
	// Step 1: Get card token from getCardTokenSecure service
	cardToken, err := p.getCardTokenSecure(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to get card token: %w", err)
	}

	// Step 2: Process payment based on 3D requirement
	if is3D {
		return p.provision3DWithToken(ctx, request, cardToken)
	}

	return p.provisionWithToken(ctx, request, cardToken)
}

// getCardTokenSecure handles card tokenization according to Paycell docs
func (p *PaycellProvider) getCardTokenSecure(ctx context.Context, request provider.PaymentRequest) (string, error) {
	// Generate transaction details using docs-compliant format
	transactionDateTime := p.generateTransactionDateTime()
	transactionID := testPrefix + transactionDateTime

	// Get a card token from card details (card tokenization only)
	cardTokenRequest := PaycellGetCardTokenSecureRequest{
		Header: PaycellRequestHeader{
			ApplicationName:     p.username,
			TransactionDateTime: transactionDateTime,
			TransactionID:       transactionID,
		},
		CCAuthor:        request.CardInfo.CardHolderName,
		CreditCardNo:    request.CardInfo.CardNumber,
		ExpireDateMonth: request.CardInfo.ExpireMonth,
		ExpireDateYear:  getLastTwoDigits(request.CardInfo.ExpireYear),
		CvcCode:         request.CardInfo.CVV,
		HashData:        p.generateHashData(transactionID, transactionDateTime),
	}

	// Call getCardTokenSecure to get card token - use provider's payment management URL
	cardTokenEndpoint := p.paymentManagementURL + endpointGetCardTokenSecure

	jsonData, err := json.Marshal(cardTokenRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal card token request: %v", err)
	}

	// add provider request to client request
	_ = provider.AddProviderRequestToClientRequest("paycell", "cardTokenRequest", cardTokenRequest, p.logID)

	req, err := http.NewRequestWithContext(ctx, "POST", cardTokenEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create card token request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send card token request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read card token response: %v", err)
	}

	var cardTokenResp PaycellGetCardTokenSecureResponse
	if err := json.Unmarshal(body, &cardTokenResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal card token response: %v. Response body: %s", err, string(body))
	}

	// Check for success
	if cardTokenResp.Header.ResponseCode != "0" {
		return "", fmt.Errorf("card token error: %s - %s", cardTokenResp.Header.ResponseCode, cardTokenResp.Header.ResponseDescription)
	}

	// Return the card token
	return cardTokenResp.CardToken, nil
}

// generateHashData generates hash data for requests using test constants
func (p *PaycellProvider) generateHashData(transactionID, transactionDateTime string) string {
	// Stage 1: SecurityData = hash(applicationPwd + applicationName)
	securityDataInput := p.password + p.username
	securityData := p.generateHash(securityDataInput)

	// Stage 2: HashData = hash(applicationName + transactionId + transactionDateTime + secureCode + securityData)
	hashDataInput := p.username + transactionID + transactionDateTime + p.secureCode + securityData
	return p.generateHash(hashDataInput)
}

// generateHash generates hash using Paycell's algorithm
func (p *PaycellProvider) generateHash(data string) string {
	// Convert to uppercase, then SHA-256, then base64
	upperData := strings.ToUpper(data)
	hasher := sha256.New()
	hasher.Write([]byte(upperData))
	hashBytes := hasher.Sum(nil)
	return base64.StdEncoding.EncodeToString(hashBytes)
}

// provisionWithToken processes a regular payment with card token
func (p *PaycellProvider) provisionWithToken(ctx context.Context, request provider.PaymentRequest, cardToken string) (*provider.PaymentResponse, error) {
	endpoint := endpointProvisionAll
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	requestHeader := PaycellRequestHeader{
		ApplicationName:     p.username,
		ApplicationPwd:      p.password,
		ClientIPAddress:     p.clientIP,
		TransactionDateTime: transactionDateTime,
		TransactionID:       transactionID,
	}

	paycellReq := PaycellProvisionRequest{
		ExtraParameters:         nil,
		RequestHeader:           requestHeader,
		AcquirerBankCode:        nil,
		Amount:                  fmt.Sprintf("%.0f", request.Amount*100), // Convert TL to kuruş (multiply by 100)
		CardID:                  nil,
		CardToken:               &cardToken,
		Currency:                request.Currency,
		InstallmentCount:        nil,
		MerchantCode:            p.merchantID,
		MSISDN:                  request.Customer.PhoneNumber,
		OriginalReferenceNumber: nil,
		PaymentType:             "SALE",
		PaymentMethodType:       "CREDIT_CARD",
		Pin:                     nil,
		PointAmount:             nil,
		ReferenceNumber:         p.generateReferenceNumber(),
		ThreeDSessionID:         nil,
	}

	return p.sendProvisionRequest(ctx, endpoint, paycellReq)
}

// provision3DWithToken processes a 3D secure payment with card token
func (p *PaycellProvider) provision3DWithToken(ctx context.Context, request provider.PaymentRequest, cardToken string) (*provider.PaymentResponse, error) {
	// First, get 3D session
	threeDSession, err := p.getThreeDSession(ctx, request, cardToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get 3D session: %w", err)
	}

	// Build callback URL through GoPay (like other providers)
	gopayCallbackURL := fmt.Sprintf("%s/v1/callback/paycell", p.gopayBaseURL)
	if request.CallbackURL != "" {
		gopayCallbackURL += "?originalCallbackUrl=" + request.CallbackURL
		// Add tenant ID to callback URL for proper tenant identification
		if request.TenantID != 0 {
			gopayCallbackURL += fmt.Sprintf("&tenantId=%d", request.TenantID)
		}
	} else {
		// Add tenant ID to callback URL for proper tenant identification
		if request.TenantID != 0 {
			gopayCallbackURL += fmt.Sprintf("?tenantId=%d", request.TenantID)
		}
	}

	// Instead of returning HTML form to app, GoPay submits form internally and gets redirect URL
	redirectURL, err := p.submit3DForm(ctx, threeDSession.ThreeDSessionId, gopayCallbackURL)
	if err != nil {
		return nil, fmt.Errorf("failed to submit 3D form: %w", err)
	}

	now := time.Now()
	// Return only redirect URL (like other providers)
	return &provider.PaymentResponse{
		Success:          true,
		Status:           provider.StatusPending,
		PaymentID:        threeDSession.ThreeDSessionId,
		TransactionID:    threeDSession.ResponseHeader.TransactionID,
		Amount:           request.Amount,
		Currency:         request.Currency,
		RedirectURL:      redirectURL,
		HTML:             p.generate3DSecureHTML(threeDSession.ThreeDSessionId, gopayCallbackURL),
		Message:          "3D secure authentication required",
		SystemTime:       &now,
		ProviderResponse: threeDSession,
	}, nil
}

// getThreeDSession gets 3D session for secure payment
func (p *PaycellProvider) getThreeDSession(ctx context.Context, request provider.PaymentRequest, cardToken string) (*PaycellGetThreeDSessionResponse, error) {
	endpoint := p.baseURL + endpointGetThreeDSession
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	// Clean phone number (remove country code)
	msisdn := strings.TrimPrefix(request.Customer.PhoneNumber, "+90")
	msisdn = strings.TrimPrefix(msisdn, "90")

	paycellReq := PaycellGetThreeDSessionRequest{
		RequestHeader: PaycellRequestHeader{
			ApplicationName:     p.username,
			ApplicationPwd:      p.password,
			ClientIPAddress:     p.clientIP,
			TransactionDateTime: transactionDateTime,
			TransactionID:       transactionID,
		},
		Amount:           fmt.Sprintf("%.0f", request.Amount*100), // Convert TL to kuruş (multiply by 100)
		CardToken:        cardToken,
		InstallmentCount: 0,
		MerchantCode:     p.merchantID,
		Msisdn:           msisdn,
		Target:           "MERCHANT",
		TransactionType:  "AUTH",
	}

	// Send request directly (not through sendProvisionRequest since response format is different)
	jsonData, err := json.Marshal(paycellReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal getThreeDSession request: %v", err)
	}

	fmt.Printf("getThreeDSession Request: %s\n", string(jsonData))
	_ = provider.AddProviderRequestToClientRequest("paycell", "getThreeDSessionRequest", paycellReq, p.logID)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create getThreeDSession request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send getThreeDSession request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read getThreeDSession response: %v", err)
	}

	fmt.Printf("getThreeDSession Response: %s\n", string(body))

	var threeDSessionResp PaycellGetThreeDSessionResponse
	if err := json.Unmarshal(body, &threeDSessionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal getThreeDSession response: %v. Response: %s", err, string(body))
	}

	// Check for success
	if threeDSessionResp.ResponseHeader.ResponseCode != "0" {
		return nil, fmt.Errorf("getThreeDSession error: %s - %s", threeDSessionResp.ResponseHeader.ResponseCode, threeDSessionResp.ResponseHeader.ResponseDescription)
	}

	return &threeDSessionResp, nil
}

// submit3DForm submits the 3D secure form to Paycell and returns the redirect URL
func (p *PaycellProvider) submit3DForm(ctx context.Context, threeDSessionID, callbackURL string) (string, error) {
	formData := url.Values{}
	formData.Set("threeDSessionId", threeDSessionID)
	formData.Set("callbackurl", callbackURL)

	req, err := http.NewRequestWithContext(ctx, "POST", p.paymentManagementURL+endpointThreeDSecure, bytes.NewBufferString(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create 3D form submission request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to submit 3D form: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read 3D form response: %v", err)
	}

	// Check if response is HTML (3D secure page)
	if strings.Contains(resp.Header.Get("Content-Type"), "text/html") || strings.HasPrefix(string(body), "<") {
		// This is normal for 3D secure - return the 3D secure URL
		return p.paymentManagementURL + endpointThreeDSecure, nil
	}

	// Try to parse as JSON if it's not HTML
	var paycellResp PaycellProvisionResponse
	if err := json.Unmarshal(body, &paycellResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal 3D form response: %v", err)
	}

	// Check for success
	if paycellResp.ResponseHeader.ResponseCode != "0" {
		return "", fmt.Errorf("3D form submission error: %s - %s", paycellResp.ResponseHeader.ResponseCode, paycellResp.ResponseHeader.ResponseDescription)
	}

	// Return the redirect URL from the response
	return p.paymentManagementURL + endpointThreeDSecure, nil
}

// generate3DSecureHTML generates HTML form for 3D secure authentication according to Paycell docs
func (p *PaycellProvider) generate3DSecureHTML(threeDSessionID, callbackURL string) string {
	// Determine the correct 3D secure URL based on environment
	threeDSecureURL := p.paymentManagementURL + endpointThreeDSecure

	html := fmt.Sprintf(`<!DOCTYPE html><html><head><title>3D Secure Authentication</title><meta charset="utf-8"></head><body><div style="text-align: center; margin-top: 50px;"><p>Ödeme işleminiz 3D güvenlik sayfasına yönlendiriliyor...</p><p>Payment is being redirected to 3D secure page...</p></div><form name="threeDForm" action="%s" method="POST"><input type="hidden" name="threeDSessionId" value="%s"><input type="hidden" name="callbackurl" value="%s"></form><script type="text/javascript">document.threeDForm.submit();</script></body></html>`, threeDSecureURL, threeDSessionID, callbackURL)

	return html
}

// sendProvisionRequest sends request to Paycell provision API
func (p *PaycellProvider) sendProvisionRequest(ctx context.Context, endpoint string, data any) (*provider.PaymentResponse, error) {
	url := p.baseURL + endpoint

	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var paycellResp PaycellProvisionResponse
	if err := json.Unmarshal(respBody, &paycellResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// add provider request to client request
	_ = provider.AddProviderRequestToClientRequest("paycell", "providerRequest", data, p.logID)

	return p.mapProvisionToPaymentResponse(paycellResp), nil
}

// mapProvisionToPaymentResponse maps Paycell provision response to standard payment response
func (p *PaycellProvider) mapProvisionToPaymentResponse(paycellResp PaycellProvisionResponse) *provider.PaymentResponse {
	success := paycellResp.ResponseHeader.ResponseCode == responseCodeSuccess
	status := provider.StatusFailed
	if success {
		status = provider.StatusSuccessful
	}

	// Use amount as received from response
	amount := 0.0
	if paycellResp.Amount != "" {
		if amountFloat, err := strconv.ParseFloat(paycellResp.Amount, 64); err == nil {
			amount = amountFloat
		}
	}

	now := time.Now()
	return &provider.PaymentResponse{
		Success:          success,
		Status:           status,
		PaymentID:        paycellResp.ResponseHeader.TransactionID,
		TransactionID:    paycellResp.ResponseHeader.TransactionID,
		Amount:           amount,
		Currency:         defaultCurrency,
		Message:          paycellResp.ResponseHeader.ResponseDescription,
		ErrorCode:        paycellResp.ResponseHeader.ResponseCode,
		SystemTime:       &now,
		ProviderResponse: paycellResp,
	}
}

// generateTransactionID creates a 20-digit transaction ID
func (p *PaycellProvider) generateTransactionID() string {
	now := time.Now()
	return fmt.Sprintf("%010d%010d", now.Unix()%10000000000, now.Nanosecond()%10000000000)
}

// generateTransactionDateTime creates transaction datetime in Paycell format (YYYYMMddHHmmssSSS - 17 chars)
func (p *PaycellProvider) generateTransactionDateTime() string {
	now := time.Now()
	return now.Format("20060102150405") + fmt.Sprintf("%03d", now.Nanosecond()/1000000)
}

// generateReferenceNumber creates a unique reference number
func (p *PaycellProvider) generateReferenceNumber() string {
	now := time.Now()
	return fmt.Sprintf("REF_%d", now.UnixNano())
}

// generatePaycellHash generates hash for Paycell API authentication
func (p *PaycellProvider) generatePaycellHash(transactionID, transactionDateTime, secureCode string) string {
	// İlk aşama: SecurityData = hash(applicationPwd + applicationName)
	// Her parametre büyük harfe dönüştürülür
	securityDataInput := strings.ToUpper(p.password + p.username)
	securityDataHash := p.paycellHash(securityDataInput)
	securityDataUpper := strings.ToUpper(securityDataHash)

	// İkinci aşama: HashData = hash(applicationName + transactionId + transactionDateTime + secureCode + securityData)
	// Tüm parametreler büyük harfe dönüştürülür
	hashDataInput := strings.ToUpper(p.username+transactionID+transactionDateTime+secureCode) + securityDataUpper
	return p.paycellHash(hashDataInput)
}

// paycellHash generates SHA-256 hash and converts to base64 (no uppercase conversion here)
func (p *PaycellProvider) paycellHash(data string) string {
	hash := sha256.Sum256([]byte(data))
	encoded := base64.StdEncoding.EncodeToString(hash[:])
	return encoded
}

// getLastTwoDigits extracts last two digits from year
func getLastTwoDigits(year string) string {
	if len(year) >= 2 {
		return year[len(year)-2:]
	}
	return year
}

// generateSignature generates MD5 signature (for backward compatibility)
func (p *PaycellProvider) generateSignature(data string) string {
	// Simple hash for testing
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// mapToPaycellRequest converts a standard payment request to Paycell format (for backward compatibility)
func (p *PaycellProvider) mapToPaycellRequest(request provider.PaymentRequest, _ bool) map[string]any {
	// Create transaction datetime in Paycell format (YmdHisu - 17 chars)
	transactionDateTime := p.generateTransactionDateTime()
	transactionID := p.generateTransactionID()

	// Extract MSISDN (remove country code if present)
	msisdn := request.Customer.PhoneNumber
	if strings.HasPrefix(msisdn, "+90") {
		msisdn = msisdn[3:]
	} else if strings.HasPrefix(msisdn, "90") {
		msisdn = msisdn[2:]
	}
	if len(msisdn) > 10 {
		msisdn = msisdn[len(msisdn)-10:] // Take last 10 digits
	}

	// Create reference number (use transactionID as default)
	referenceNumber := transactionID
	if request.ConversationID != "" {
		referenceNumber = request.ConversationID
	}

	// Paycell request structure according to real API
	paycellReq := map[string]any{
		"extraParameters": nil,
		"requestHeader": map[string]any{
			"applicationName":     p.username,
			"applicationPwd":      p.password,
			"clientIPAddress":     p.clientIP,
			"transactionDateTime": transactionDateTime,
			"transactionId":       transactionID,
		},
		"acquirerBankCode":        nil,
		"amount":                  fmt.Sprintf("%.0f", request.Amount*100), // Convert TL to kuruş (multiply by 100)
		"cardId":                  nil,
		"cardToken":               nil,
		"currency":                request.Currency,
		"installmentCount":        nil,
		"merchantCode":            p.merchantID,
		"msisdn":                  msisdn,
		"originalReferenceNumber": nil,
		"paymentType":             "SALE", // Payment type for provision
		"pin":                     nil,
		"pointAmount":             nil,
		"referenceNumber":         referenceNumber,
		"threeDSessionId":         nil,
	}

	return paycellReq
}

// mapToPaymentResponse converts Paycell response to standard payment response (for backward compatibility)
func (p *PaycellProvider) mapToPaymentResponse(paycellResp PaycellResponse) *provider.PaymentResponse {
	// Parse amount from string - use amount as received
	amount, _ := strconv.ParseFloat(paycellResp.Amount, 64)

	// Use OrderID as PaymentID if PaymentID is empty
	paymentID := paycellResp.PaymentID
	if paymentID == "" {
		paymentID = paycellResp.OrderID
	}

	// Get message from different sources
	message := paycellResp.Message
	if message == "" {
		message = paycellResp.ErrorMessage
	}
	if message == "" {
		message = paycellResp.ResponseMessage
	}
	if message == "" && paycellResp.ResponseHeader.ResponseDescription != "" {
		message = paycellResp.ResponseHeader.ResponseDescription
	}
	if message == "" && paycellResp.Header.ResponseDescription != "" {
		message = paycellResp.Header.ResponseDescription
	}

	// Get transaction ID from response header if available
	transactionID := paycellResp.TransactionID
	if transactionID == "" && paycellResp.ResponseHeader.TransactionID != "" {
		transactionID = paycellResp.ResponseHeader.TransactionID
	}
	if transactionID == "" && paycellResp.Header.TransactionID != "" {
		transactionID = paycellResp.Header.TransactionID
	}

	// Get error code from response header if available
	errorCode := paycellResp.ErrorCode
	if errorCode == "" && paycellResp.ResponseHeader.ResponseCode != "" {
		errorCode = paycellResp.ResponseHeader.ResponseCode
	}
	if errorCode == "" && paycellResp.Header.ResponseCode != "" {
		errorCode = paycellResp.Header.ResponseCode
	}

	now := time.Now()
	response := &provider.PaymentResponse{
		Success:          paycellResp.Success,
		PaymentID:        paymentID,
		TransactionID:    transactionID,
		Amount:           amount,
		Currency:         paycellResp.Currency,
		Message:          message,
		ErrorCode:        errorCode,
		SystemTime:       &now,
		ProviderResponse: paycellResp,
	}

	// Determine success based on response code
	responseCode := ""
	if paycellResp.ResponseHeader.ResponseCode != "" {
		responseCode = paycellResp.ResponseHeader.ResponseCode
	} else if paycellResp.Header.ResponseCode != "" {
		responseCode = paycellResp.Header.ResponseCode
	}

	// Add 3D secure information if present
	if paycellResp.RedirectURL != "" {
		response.RedirectURL = paycellResp.RedirectURL
		response.Status = provider.StatusPending
		response.Success = true // Pending is still successful initiation
		// Also preserve HTML if provided
		if paycellResp.HTML != "" {
			response.HTML = paycellResp.HTML
		}
	} else if paycellResp.HTML != "" {
		response.HTML = paycellResp.HTML
		response.Status = provider.StatusPending
		response.Success = true // Pending is still successful initiation
	} else if paycellResp.Status == statusPending {
		response.Status = provider.StatusPending
		response.Success = false // Pending without redirect means waiting
	} else if paycellResp.Status == statusCancelled {
		response.Status = provider.StatusCancelled
		response.Success = false
	} else if responseCode == responseCodeSuccess {
		response.Success = true
		response.Status = provider.StatusSuccessful
	} else {
		response.Success = false
		response.Status = provider.StatusFailed
	}

	return response
}

// PaycellRequestHeader represents the common request header for Paycell API
type PaycellRequestHeader struct {
	ApplicationName     string `json:"applicationName"`
	ApplicationPwd      string `json:"applicationPwd"`
	ClientIPAddress     string `json:"clientIPAddress"`
	TransactionDateTime string `json:"transactionDateTime"`
	TransactionID       string `json:"transactionId"`
}

type PaycellResponseHeader struct {
	TransactionID       string `json:"transactionId"`
	ResponseDateTime    string `json:"responseDateTime"`
	ResponseCode        string `json:"responseCode"`
	ResponseDescription string `json:"responseDescription"`
}

// PaycellGetCardTokenSecureRequest represents getCardTokenSecure request
type PaycellGetCardTokenSecureRequest struct {
	Header          PaycellRequestHeader `json:"header"`
	CCAuthor        string               `json:"ccAuthor,omitempty"`
	CreditCardNo    string               `json:"creditCardNo"`
	ExpireDateMonth string               `json:"expireDateMonth"`
	ExpireDateYear  string               `json:"expireDateYear"`
	CvcCode         string               `json:"cvcNo"`
	HashData        string               `json:"hashData"`
}

// PaycellGetCardTokenSecureResponse represents getCardTokenSecure response
type PaycellGetCardTokenSecureResponse struct {
	Header    PaycellResponseHeader `json:"header"`
	CardToken string                `json:"cardToken"`
	HashData  string                `json:"hashData"`
}

// PaycellProvisionRequest represents provision request according to official docs
type PaycellProvisionRequest struct {
	ExtraParameters         map[string]any       `json:"extraParameters"`
	RequestHeader           PaycellRequestHeader `json:"requestHeader"`
	AcquirerBankCode        *string              `json:"acquirerBankCode"`
	Amount                  string               `json:"amount"`
	CardID                  *string              `json:"cardId"`
	CardToken               *string              `json:"cardToken"`
	Currency                string               `json:"currency"`
	InstallmentCount        *int                 `json:"installmentCount"`
	MerchantCode            string               `json:"merchantCode"`
	MSISDN                  string               `json:"msisdn"`
	OriginalReferenceNumber *string              `json:"originalReferenceNumber"`
	PaymentType             string               `json:"paymentType"`
	PaymentMethodType       string               `json:"paymentMethodType"`
	Pin                     *string              `json:"pin"`
	PointAmount             *string              `json:"pointAmount"`
	ReferenceNumber         string               `json:"referenceNumber"`
	ThreeDSessionID         *string              `json:"threeDSessionId"`
}

// PaycellProvisionResponse represents provision response
type PaycellProvisionResponse struct {
	ResponseHeader     PaycellResponseHeader `json:"responseHeader"`
	ExtraParameters    map[string]any        `json:"extraParameters"`
	AcquirerBankCode   string                `json:"acquirerBankCode"`
	IssuerBankCode     string                `json:"issuerBankCode"`
	ApprovalCode       string                `json:"approvalCode"`
	ReconciliationDate string                `json:"reconciliationDate"`
	Amount             string                `json:"amount"`
}

// PaycellGetThreeDSessionRequest represents getThreeDSession request matching docs format
type PaycellGetThreeDSessionRequest struct {
	RequestHeader    PaycellRequestHeader `json:"requestHeader"`
	Amount           string               `json:"amount"` // Amount in kuruş as string
	CardToken        string               `json:"cardToken"`
	InstallmentCount int                  `json:"installmentCount"`
	MerchantCode     string               `json:"merchantCode"`
	Msisdn           string               `json:"msisdn"`
	Target           string               `json:"target"`
	TransactionType  string               `json:"transactionType"`
}

// PaycellGetThreeDSessionResponse represents getThreeDSession response matching docs format
type PaycellGetThreeDSessionResponse struct {
	ResponseHeader  PaycellResponseHeader `json:"responseHeader"`
	ExtraParameters map[string]any        `json:"extraParameters"`
	ThreeDSessionId string                `json:"threeDSessionId"`
}

// PaycellGetThreeDSessionResultRequest represents getThreeDSessionResult request
type PaycellGetThreeDSessionResultRequest struct {
	RequestHeader   PaycellRequestHeader `json:"requestHeader"`
	MerchantCode    string               `json:"merchantCode"`
	ThreeDSessionID string               `json:"threeDSessionId"`
	MSISDN          string               `json:"msisdn"`
}

// PaycellInquireRequest represents inquire request
type PaycellInquireRequest struct {
	RequestHeader           PaycellRequestHeader `json:"requestHeader"`
	OriginalReferenceNumber string               `json:"originalReferenceNumber"`
	ReferenceNumber         string               `json:"referenceNumber"`
	MerchantCode            string               `json:"merchantCode"`
	MSISDN                  string               `json:"msisdn"`
	PaymentMethodType       string               `json:"paymentMethodType"`
}

// PaycellReverseRequest represents reverse request
type PaycellReverseRequest struct {
	RequestHeader           PaycellRequestHeader `json:"requestHeader"`
	OriginalReferenceNumber string               `json:"originalReferenceNumber"`
	ReferenceNumber         string               `json:"referenceNumber"`
	MerchantCode            string               `json:"merchantCode"`
	PaymentType             string               `json:"paymentType"`
}

// PaycellRefundRequest represents refund request
type PaycellRefundRequest struct {
	RequestHeader           PaycellRequestHeader `json:"requestHeader"`
	OriginalReferenceNumber string               `json:"originalReferenceNumber"`
	ReferenceNumber         string               `json:"referenceNumber"`
	MerchantCode            string               `json:"merchantCode"`
	Amount                  string               `json:"amount"`
	PaymentType             string               `json:"paymentType"`
}

// PaycellResponse represents a response from Paycell API (for backward compatibility)
type PaycellResponse struct {
	// Standard fields (backward compatibility)
	Success           bool   `json:"success"`
	Status            string `json:"status"`
	OrderID           string `json:"orderId"`
	PaymentID         string `json:"paymentId"`
	TransactionID     string `json:"transactionId"`
	Amount            string `json:"amount"`
	Currency          string `json:"currency"`
	Message           string `json:"message"`
	ErrorCode         string `json:"errorCode"`
	ErrorMessage      string `json:"errorMessage"`
	RedirectURL       string `json:"redirectUrl,omitempty"`
	HTML              string `json:"html,omitempty"`
	ThreeDSessionID   string `json:"threeDSessionId,omitempty"`
	ThreeDURL         string `json:"threeDUrl,omitempty"`
	ProvisionResponse string `json:"provisionResponse,omitempty"`
	ResponseCode      string `json:"responseCode"`
	ResponseMessage   string `json:"responseMessage"`

	// Real Paycell API response structure (for different endpoints)
	ResponseHeader PaycellResponseHeader `json:"responseHeader"`

	// Alternative header format (getCardTokenSecure uses this)
	Header PaycellResponseHeader `json:"header"`

	ExtraParameters         map[string]any `json:"extraParameters,omitempty"`
	AcquirerBankCode        string         `json:"acquirerBankCode,omitempty"`
	IssuerBankCode          string         `json:"issuerBankCode,omitempty"`
	ApprovalCode            string         `json:"approvalCode,omitempty"`
	ReconciliationDate      string         `json:"reconciliationDate,omitempty"`
	IyzPaymentID            string         `json:"iyzPaymentId,omitempty"`
	IyzPaymentTransactionID string         `json:"iyzPaymentTransactionId,omitempty"`
}
