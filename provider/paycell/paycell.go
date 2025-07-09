package paycell

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

// PaycellProvider implements the provider.PaymentProvider interface for Paycell
type PaycellProvider struct {
	username             string
	password             string
	merchantID           string
	terminalID           string
	secureCode           string // Paycell secure code for hash generation
	baseURL              string
	paymentManagementURL string // For 3D secure operations
	gopayBaseURL         string // GoPay's own base URL for callbacks
	isProduction         bool
	client               *http.Client
}

// NewProvider creates a new Paycell payment provider
func NewProvider() provider.PaymentProvider {
	return &PaycellProvider{
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
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
			Key:         "terminalId",
			Required:    true,
			Type:        "string",
			Description: "Paycell Terminal ID (provided by Paycell)",
			Example:     "VP123456",
			MinLength:   5,
			MaxLength:   20,
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
	p.username = conf["username"]
	p.password = conf["password"]
	p.merchantID = conf["merchantId"]
	p.terminalID = conf["terminalId"]
	p.secureCode = conf["secureCode"]

	if p.username == "" || p.password == "" || p.merchantID == "" || p.terminalID == "" {
		return errors.New("paycell: username, password, merchantId and terminalId are required")
	}

	// Set default secure code if not provided
	if p.secureCode == "" {
		p.secureCode = "PAYCELL12345" // Default test secure code
	}

	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	p.isProduction = conf["environment"] == "production"
	if p.isProduction {
		p.baseURL = apiProductionURL
		p.paymentManagementURL = paymentManagementProductionURL
	} else {
		p.baseURL = apiSandboxURL
		p.paymentManagementURL = paymentManagementSandboxURL
	}

	return nil
}

// CreatePayment makes a non-3D payment request
func (p *PaycellProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("paycell: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *PaycellProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
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
		ClientIPAddress:     "127.0.0.1", // Default IP
		TransactionDateTime: transactionDateTime,
		TransactionID:       transactionID,
	}

	paycellReq := PaycellGetThreeDSessionResultRequest{
		RequestHeader:   requestHeader,
		ThreeDSessionID: threeDSessionID,
	}

	return p.sendProvisionRequest(ctx, endpoint, paycellReq)
}

// GetPaymentStatus retrieves the current status of a payment
func (p *PaycellProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	endpoint := endpointInquire
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	requestHeader := PaycellRequestHeader{
		ApplicationName:     p.username,
		ApplicationPwd:      p.password,
		ClientIPAddress:     "127.0.0.1", // Default IP
		TransactionDateTime: transactionDateTime,
		TransactionID:       transactionID,
	}

	paycellReq := PaycellInquireRequest{
		RequestHeader:   requestHeader,
		ReferenceNumber: paymentID,
		MerchantCode:    p.merchantID,
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
		ClientIPAddress:     "127.0.0.1", // Default IP
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
		ClientIPAddress:     "127.0.0.1", // Default IP
		TransactionDateTime: transactionDateTime,
		TransactionID:       transactionID,
	}

	// Convert amount to kuruş (multiply by 100)
	amountInKurus := strconv.FormatFloat(request.RefundAmount*100, 'f', 0, 64)

	paycellReq := PaycellRefundRequest{
		RequestHeader:           requestHeader,
		OriginalReferenceNumber: request.PaymentID,
		ReferenceNumber:         p.generateReferenceNumber(),
		MerchantCode:            p.merchantID,
		Amount:                  amountInKurus,
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
		Header: struct {
			ApplicationName     string `json:"applicationName"`
			TransactionDateTime string `json:"transactionDateTime"`
			TransactionID       string `json:"transactionId"`
		}{
			ApplicationName:     testApplicationName,
			TransactionDateTime: transactionDateTime,
			TransactionID:       transactionID,
		},
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

	fmt.Printf("getCardTokenSecure Request: %s\n", string(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", cardTokenEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create card token request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send card token request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read card token response: %v", err)
	}

	fmt.Printf("getCardTokenSecure Response: %s\n", string(body))

	var cardTokenResp PaycellGetCardTokenSecureResponse
	if err := json.Unmarshal(body, &cardTokenResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal card token response: %v", err)
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
	securityDataInput := testApplicationPwd + testApplicationName
	securityData := p.generateHash(securityDataInput)

	// Stage 2: HashData = hash(applicationName + transactionId + transactionDateTime + secureCode + securityData)
	hashDataInput := testApplicationName + transactionID + transactionDateTime + testSecureCode + securityData
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
	endpoint := endpointProvision
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	requestHeader := PaycellRequestHeader{
		ApplicationName:     p.username,
		ApplicationPwd:      p.password,
		ClientIPAddress:     "127.0.0.1", // Default IP
		TransactionDateTime: transactionDateTime,
		TransactionID:       transactionID,
	}

	// Clean phone number (remove country code)
	msisdn := strings.TrimPrefix(request.Customer.PhoneNumber, "+90")
	msisdn = strings.TrimPrefix(msisdn, "90")

	// Convert amount to kuruş (multiply by 100)
	amountInKurus := strconv.FormatFloat(request.Amount*100, 'f', 0, 64)

	paycellReq := PaycellProvisionRequest{
		RequestHeader:   requestHeader,
		CardToken:       cardToken,
		MerchantCode:    p.merchantID,
		MSISDN:          msisdn,
		ReferenceNumber: p.generateReferenceNumber(),
		Amount:          amountInKurus,
		PaymentType:     "SALE",
		EulaID:          p.terminalID,
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

	now := time.Now()
	// Return 3D redirect information
	return &provider.PaymentResponse{
		Success:          true,
		Status:           provider.StatusPending,
		PaymentID:        threeDSession.ThreeDSessionId,
		TransactionID:    threeDSession.ResponseHeader.TransactionID,
		Amount:           request.Amount,
		Currency:         request.Currency,
		RedirectURL:      p.paymentManagementURL + endpointThreeDSecure,
		HTML:             p.generate3DForm(threeDSession.ThreeDSessionId, request.CallbackURL),
		Message:          "3D secure authentication required",
		SystemTime:       &now,
		ProviderResponse: threeDSession,
	}, nil
}

// getThreeDSession gets 3D session for secure payment
func (p *PaycellProvider) getThreeDSession(ctx context.Context, request provider.PaymentRequest, cardToken string) (*PaycellGetThreeDSessionResponse, error) {
	endpoint := endpointGetThreeDSession
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	// Clean phone number (remove country code)
	msisdn := strings.TrimPrefix(request.Customer.PhoneNumber, "+90")
	msisdn = strings.TrimPrefix(msisdn, "90")

	paycellReq := PaycellGetThreeDSessionRequest{
		RequestHeader: PaycellRequestHeader{
			ApplicationName:     p.username,
			ApplicationPwd:      p.password,
			ClientIPAddress:     "127.0.0.1",
			TransactionDateTime: transactionDateTime,
			TransactionID:       transactionID,
		},
		Amount:           fmt.Sprintf("%.0f", request.Amount*100), // Convert to kuruş
		CardToken:        cardToken,
		InstallmentCount: 0,
		MerchantCode:     p.merchantID,
		Msisdn:           msisdn,
		ReferenceNumber:  p.generateReferenceNumber(),
		Target:           "AUTH",
		TransactionType:  "SALE",
	}

	response, err := p.sendProvisionRequest(ctx, endpoint, paycellReq)
	if err != nil {
		return nil, err
	}

	// Convert response to 3D session response
	return &PaycellGetThreeDSessionResponse{
		ResponseHeader: struct {
			TransactionID       string `json:"transactionId"`
			ResponseDateTime    string `json:"responseDateTime"`
			ResponseCode        string `json:"responseCode"`
			ResponseDescription string `json:"responseDescription"`
		}{
			TransactionID:       response.TransactionID,
			ResponseDateTime:    time.Now().Format("20060102150405000"),
			ResponseCode:        "0",
			ResponseDescription: "Success",
		},
		ExtraParameters: nil,
		ThreeDSessionId: response.PaymentID,
	}, nil
}

// generate3DForm generates HTML form for 3D secure authentication
func (p *PaycellProvider) generate3DForm(threeDSessionID, callbackURL string) string {
	return fmt.Sprintf(`
<html>
<head>
    <title>Paycell 3D-Secure Processing</title>
</head>
<body>
    <form name="threeDForm" action="%s%s" method="POST">
        <input type="hidden" name="threeDSessionId" value="%s" />
        <input type="hidden" name="callbackurl" value="%s" />
        <input type="submit" value="Confirm Payment" />
    </form>
    <script>
        document.threeDForm.submit();
    </script>
</body>
</html>`, p.paymentManagementURL, endpointThreeDSecure, threeDSessionID, callbackURL)
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

	return p.mapProvisionToPaymentResponse(paycellResp), nil
}

// mapProvisionToPaymentResponse maps Paycell provision response to standard payment response
func (p *PaycellProvider) mapProvisionToPaymentResponse(paycellResp PaycellProvisionResponse) *provider.PaymentResponse {
	success := paycellResp.ResponseHeader.ResponseCode == responseCodeSuccess
	status := provider.StatusFailed
	if success {
		status = provider.StatusSuccessful
	}

	// Convert amount back from kuruş to TRY
	amount := 0.0
	if paycellResp.Amount != "" {
		if amountInt, err := strconv.ParseFloat(paycellResp.Amount, 64); err == nil {
			amount = amountInt / 100
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

// Paycell API Request/Response Structures

// PaycellRequestHeader represents the common request header for Paycell API
type PaycellRequestHeader struct {
	ApplicationName     string `json:"applicationName"`
	ApplicationPwd      string `json:"applicationPwd"`
	ClientIPAddress     string `json:"clientIPAddress"`
	TransactionDateTime string `json:"transactionDateTime"`
	TransactionID       string `json:"transactionId"`
}

// PaycellGetCardTokenSecureRequest represents getCardTokenSecure request
type PaycellGetCardTokenSecureRequest struct {
	Header struct {
		ApplicationName     string `json:"applicationName"`
		TransactionDateTime string `json:"transactionDateTime"`
		TransactionID       string `json:"transactionId"`
	} `json:"header"`
	CreditCardNo    string `json:"creditCardNo"`
	ExpireDateMonth string `json:"expireDateMonth"`
	ExpireDateYear  string `json:"expireDateYear"`
	CvcCode         string `json:"cvcNo"` // Note: API uses "cvcNo" not "cvcCode"
	HashData        string `json:"hashData"`
}

// PaycellGetCardTokenSecureResponse represents getCardTokenSecure response
type PaycellGetCardTokenSecureResponse struct {
	Header struct {
		TransactionID       string `json:"transactionId"`
		ResponseDateTime    string `json:"responseDateTime"`
		ResponseCode        string `json:"responseCode"`
		ResponseDescription string `json:"responseDescription"`
	} `json:"header"`
	CardToken string `json:"cardToken"`
}

// PaycellProvisionRequest represents provision request
type PaycellProvisionRequest struct {
	RequestHeader   PaycellRequestHeader `json:"requestHeader"`
	CardToken       string               `json:"cardToken"`
	MerchantCode    string               `json:"merchantCode"`
	MSISDN          string               `json:"msisdn"`
	ReferenceNumber string               `json:"referenceNumber"`
	Amount          string               `json:"amount"`
	PaymentType     string               `json:"paymentType"`
	EulaID          string               `json:"eulaId"`
}

// PaycellProvisionResponse represents provision response
type PaycellProvisionResponse struct {
	ResponseHeader struct {
		TransactionID       string `json:"transactionId"`
		ResponseDateTime    string `json:"responseDateTime"`
		ResponseCode        string `json:"responseCode"`
		ResponseDescription string `json:"responseDescription"`
	} `json:"responseHeader"`
	ExtraParameters    map[string]any `json:"extraParameters"`
	AcquirerBankCode   string         `json:"acquirerBankCode"`
	IssuerBankCode     string         `json:"issuerBankCode"`
	ApprovalCode       string         `json:"approvalCode"`
	ReconciliationDate string         `json:"reconciliationDate"`
	Amount             string         `json:"amount"`
}

// PaycellGetThreeDSessionRequest represents getThreeDSession request matching docs format
type PaycellGetThreeDSessionRequest struct {
	RequestHeader    PaycellRequestHeader `json:"requestHeader"`
	Amount           string               `json:"amount"` // Amount in kuruş as string
	CardToken        string               `json:"cardToken"`
	InstallmentCount int                  `json:"installmentCount"`
	MerchantCode     string               `json:"merchantCode"`
	Msisdn           string               `json:"msisdn"`
	ReferenceNumber  string               `json:"referenceNumber"`
	Target           string               `json:"target"`
	TransactionType  string               `json:"transactionType"`
}

// PaycellGetThreeDSessionResponse represents getThreeDSession response matching docs format
type PaycellGetThreeDSessionResponse struct {
	ResponseHeader struct {
		TransactionID       string `json:"transactionId"`
		ResponseDateTime    string `json:"responseDateTime"`
		ResponseCode        string `json:"responseCode"`
		ResponseDescription string `json:"responseDescription"`
	} `json:"responseHeader"`
	ExtraParameters map[string]any `json:"extraParameters"`
	ThreeDSessionId string         `json:"threeDSessionId"`
}

// PaycellGetThreeDSessionResultRequest represents getThreeDSessionResult request
type PaycellGetThreeDSessionResultRequest struct {
	RequestHeader   PaycellRequestHeader `json:"requestHeader"`
	ThreeDSessionID string               `json:"threeDSessionId"`
}

// PaycellInquireRequest represents inquire request
type PaycellInquireRequest struct {
	RequestHeader   PaycellRequestHeader `json:"requestHeader"`
	ReferenceNumber string               `json:"referenceNumber"`
	MerchantCode    string               `json:"merchantCode"`
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
	ResponseHeader struct {
		TransactionID       string `json:"transactionId"`
		ResponseDateTime    string `json:"responseDateTime"`
		ResponseCode        string `json:"responseCode"`
		ResponseDescription string `json:"responseDescription"`
	} `json:"responseHeader,omitempty"`

	// Alternative header format (getCardTokenSecure uses this)
	Header struct {
		TransactionID       string `json:"transactionId"`
		ResponseDateTime    string `json:"responseDateTime"`
		ResponseCode        string `json:"responseCode"`
		ResponseDescription string `json:"responseDescription"`
	} `json:"header,omitempty"`

	ExtraParameters         map[string]any `json:"extraParameters,omitempty"`
	AcquirerBankCode        string         `json:"acquirerBankCode,omitempty"`
	IssuerBankCode          string         `json:"issuerBankCode,omitempty"`
	ApprovalCode            string         `json:"approvalCode,omitempty"`
	ReconciliationDate      string         `json:"reconciliationDate,omitempty"`
	IyzPaymentID            string         `json:"iyzPaymentId,omitempty"`
	IyzPaymentTransactionID string         `json:"iyzPaymentTransactionId,omitempty"`
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
			"clientIPAddress":     "127.0.0.1", // Default, should be real client IP in production
			"transactionDateTime": transactionDateTime,
			"transactionId":       transactionID,
		},
		"acquirerBankCode":        nil,
		"amount":                  fmt.Sprintf("%.0f", request.Amount*100), // Convert to cents
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
	// Parse amount from string
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

	if responseCode == responseCodeSuccess {
		response.Success = true
		response.Status = provider.StatusSuccessful
	} else {
		response.Success = false
		response.Status = provider.StatusFailed
	}

	// Add 3D secure information if present
	if paycellResp.RedirectURL != "" {
		response.RedirectURL = paycellResp.RedirectURL
		response.Status = provider.StatusPending
	}

	if paycellResp.HTML != "" {
		response.HTML = paycellResp.HTML
	}

	return response
}

// generateSignature generates MD5 signature (for backward compatibility)
func (p *PaycellProvider) generateSignature(data string) string {
	// Simple hash for testing
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}
