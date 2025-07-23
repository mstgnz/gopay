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
	p.username = conf["username"]
	p.password = conf["password"]
	p.merchantID = conf["merchantId"]
	p.secureCode = conf["secureCode"]

	if p.username == "" || p.password == "" || p.merchantID == "" || p.secureCode == "" {
		return errors.New("paycell: username, password, merchantId and secureCode are required")
	}

	p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")

	p.isProduction = conf["environment"] == "production"
	if p.isProduction {
		p.baseURL = apiProductionURL
		p.paymentManagementURL = paymentManagementProductionURL
		// Production environment - use secure TLS
		p.client = &http.Client{
			Timeout: defaultTimeout,
		}
	} else {
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
func (p *PaycellProvider) Complete3DPayment(ctx context.Context, callbackState *provider.CallbackState, data map[string]string) (*provider.PaymentResponse, error) {

	cardToken, err := provider.GetProviderRequestFromLog("paycell", callbackState.PaymentID, "cardToken")
	if err != nil {
		return nil, fmt.Errorf("failed to get card token: %s %w", callbackState.PaymentID, err)
	}

	msisdn, err := provider.GetProviderRequestFromLog("paycell", callbackState.PaymentID, "msisdn")
	if err != nil {
		return nil, fmt.Errorf("failed to get phone number: %s %w", callbackState.PaymentID, err)
	}
	p.phoneNumber = msisdn

	// sadece 3D doğrulama sonucu
	threeDSessionResp, err := p.threeDSessionResult(ctx, callbackState, data)
	if err != nil {
		return nil, fmt.Errorf("paycell: failed to complete 3D payment: %w", err)
	}
	// Convert to standard payment response
	now := time.Now()
	response := &provider.PaymentResponse{
		Success:          threeDSessionResp.ThreeDOperationResult.ThreeDResult == "0",
		PaymentID:        callbackState.PaymentID,
		TransactionID:    threeDSessionResp.ThreeDOperationResult.ResponseHeader.TransactionID,
		Amount:           callbackState.Amount,
		Currency:         callbackState.Currency,
		SystemTime:       &now,
		ProviderResponse: threeDSessionResp,
	}

	// Set status and message based on 3D result
	if threeDSessionResp.ThreeDOperationResult.ThreeDResult == "0" {
		response.Status = provider.StatusSuccessful
		response.Message = threeDSessionResp.ThreeDOperationResult.ThreeDResultDescription
	} else {
		response.Status = provider.StatusFailed
		response.Message = threeDSessionResp.MdErrorMessage
		if response.Message == "" {
			response.Message = threeDSessionResp.ThreeDOperationResult.ThreeDResultDescription
		}
		response.ErrorCode = threeDSessionResp.ThreeDOperationResult.ThreeDResult
	}

	if response.Success {

		// ödemeyi tamamla
		request := provider.PaymentRequest{
			Amount:   callbackState.Amount,
			Currency: callbackState.Currency,
			Customer: provider.Customer{
				PhoneNumber: msisdn,
			},
		}
		response, err = p.provisionAll(ctx, request, cardToken, callbackState.PaymentID)
		if err != nil {
			return nil, fmt.Errorf("paycell: failed to complete 3D payment: %w", err)
		}
	}

	response.PaymentID = callbackState.PaymentID
	response.RedirectURL = callbackState.OriginalCallback

	return response, nil
}

// GetPaymentStatus retrieves the current status of a payment
func (p *PaycellProvider) GetPaymentStatus(ctx context.Context, request provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	// Get reference number from log
	originalReferenceNumber, err := provider.GetProviderRequestFromLog("paycell", request.PaymentID, "referenceNumber")
	if err != nil {
		return nil, fmt.Errorf("failed to get reference number: %s %w", request.PaymentID, err)
	}

	// Get amount from log
	amountStr, err := provider.GetProviderRequestFromLog("paycell", request.PaymentID, "amount")
	if err != nil {
		return nil, fmt.Errorf("failed to get amount: %s %w", request.PaymentID, err)
	}

	// Get card token from log
	cardToken, err := provider.GetProviderRequestFromLog("paycell", request.PaymentID, "cardToken")
	if err != nil {
		return nil, fmt.Errorf("failed to get card token: %s %w", request.PaymentID, err)
	}

	// Set clientIP for inquire operation - use a default if not available
	if p.clientIP == "" {
		p.clientIP = "127.0.0.1" // Default fallback
	}

	// Prepare request according to PayCell documentation
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	// Create request structure as shown in documentation
	paycellReq := map[string]any{
		"paymentMethodType":       "CREDIT_CARD",
		"merchantCode":            p.merchantID,
		"msisdn":                  p.phoneNumber,
		"originalReferenceNumber": originalReferenceNumber,
		"referenceNumber":         p.generateReferenceNumber(),
		"amount":                  amountStr,
		"currency":                "TRY",
		"paymentType":             "SALE",
		"cardToken":               cardToken,
		"requestHeader": map[string]any{
			"transactionId":       transactionID,
			"transactionDateTime": transactionDateTime,
			"clientIPAddress":     p.clientIP,
			"applicationName":     p.username,
			"applicationPwd":      p.password,
		},
	}

	// Make HTTP request to inquireAll endpoint
	url := p.baseURL + endpointInquireAll

	jsonData, err := json.Marshal(paycellReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal inquire request: %w", err)
	}

	// Add provider request to client request log
	_ = provider.AddProviderRequestToClientRequest("paycell", "inquireRequest", paycellReq, p.logID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create inquire request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send inquire request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read inquire response: %w", err)
	}

	var inquireResp PaycellInquireResponse
	if err := json.Unmarshal(body, &inquireResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal inquire response: %w. Response body: %s", err, string(body))
	}

	// Convert to standard payment response
	now := time.Now()
	response := &provider.PaymentResponse{
		Success:          inquireResp.ResponseHeader.ResponseCode == "0",
		PaymentID:        request.PaymentID,
		TransactionID:    inquireResp.ResponseHeader.TransactionID,
		SystemTime:       &now,
		ProviderResponse: inquireResp,
	}

	// Set status and message based on inquire result
	if inquireResp.ResponseHeader.ResponseCode == "0" {
		// Map PayCell status to provider status
		switch inquireResp.Status {
		case "SALE":
			response.Status = provider.StatusSuccessful
		case "PENDING":
			response.Status = provider.StatusPending
		case "REFUNDED":
			response.Status = provider.StatusSuccessful // Treat refunded as successful
		case "CANCELLED":
			response.Status = provider.StatusCancelled
		default:
			response.Status = provider.StatusPending
		}
		response.Message = inquireResp.ResponseHeader.ResponseDescription

		// Get amount from provision list if available
		if len(inquireResp.ProvisionList) > 0 {
			provision := inquireResp.ProvisionList[0]
			if amountFloat, err := strconv.ParseFloat(provision.Amount, 64); err == nil {
				response.Amount = amountFloat / 100 // Convert from kuruş to TRY
			}
		}
	} else {
		response.Status = provider.StatusFailed
		response.Message = inquireResp.ResponseHeader.ResponseDescription
		response.ErrorCode = inquireResp.ResponseHeader.ResponseCode
	}

	return response, nil
}

// CancelPayment cancels a payment (reverse operation)
func (p *PaycellProvider) CancelPayment(ctx context.Context, request provider.CancelRequest) (*provider.PaymentResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	// Get original reference number from log
	originalReferenceNumber, err := provider.GetProviderRequestFromLog("paycell", request.PaymentID, "referenceNumber")
	if err != nil {
		return nil, fmt.Errorf("failed to get reference number: %s %w", request.PaymentID, err)
	}

	// Get amount from log for reverse operation
	amountStr, err := provider.GetProviderRequestFromLog("paycell", request.PaymentID, "amount")
	if err != nil {
		return nil, fmt.Errorf("failed to get amount: %s %w", request.PaymentID, err)
	}

	msisdn, err := provider.GetProviderRequestFromLog("paycell", request.PaymentID, "msisdn")
	if err != nil {
		return nil, fmt.Errorf("failed to get phone number: %s %w", request.PaymentID, err)
	}
	p.phoneNumber = msisdn

	// Set clientIP for reverse operation - use a default if not available
	if p.clientIP == "" {
		p.clientIP = "127.0.0.1" // Default fallback
	}

	// Prepare request according to PayCell documentation
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	// Create request structure as shown in documentation
	paycellReq := map[string]any{
		"merchantCode":            p.merchantID,
		"msisdn":                  p.phoneNumber,
		"originalReferenceNumber": originalReferenceNumber,
		"referenceNumber":         p.generateReferenceNumber(),
		"amount":                  amountStr,
		"requestHeader": map[string]any{
			"applicationName":     p.username,
			"applicationPwd":      p.password,
			"clientIPAddress":     p.clientIP,
			"transactionDateTime": transactionDateTime,
			"transactionId":       transactionID,
		},
	}

	// Make HTTP request to reverse endpoint
	url := p.baseURL + endpointReverse

	jsonData, err := json.Marshal(paycellReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reverse request: %w", err)
	}

	// Add provider request to client request log
	_ = provider.AddProviderRequestToClientRequest("paycell", "reverseRequest", paycellReq, p.logID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create reverse request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send reverse request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read reverse response: %w", err)
	}

	var reverseResp PaycellReverseResponse
	if err := json.Unmarshal(body, &reverseResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reverse response: %w. Response body: %s", err, string(body))
	}

	// Convert to standard payment response
	now := time.Now()
	response := &provider.PaymentResponse{
		Success:          reverseResp.ResponseHeader.ResponseCode == "0",
		PaymentID:        request.PaymentID,
		TransactionID:    reverseResp.ResponseHeader.TransactionID,
		SystemTime:       &now,
		ProviderResponse: reverseResp,
	}

	// Set status and message based on reverse result
	if reverseResp.ResponseHeader.ResponseCode == "0" {
		response.Status = provider.StatusCancelled
		response.Message = reverseResp.ResponseHeader.ResponseDescription
	} else {
		response.Status = provider.StatusFailed
		response.Message = reverseResp.ResponseHeader.ResponseDescription
		response.ErrorCode = reverseResp.ResponseHeader.ResponseCode
	}

	return response, nil
}

// RefundPayment refunds a payment
func (p *PaycellProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	if request.RefundAmount <= 0 {
		return nil, errors.New("paycell: refund amount must be greater than 0")
	}

	// Get original reference number from log
	originalReferenceNumber, err := provider.GetProviderRequestFromLog("paycell", request.PaymentID, "referenceNumber")
	if err != nil {
		return nil, fmt.Errorf("failed to get reference number: %s %w", request.PaymentID, err)
	}

	msisdn, err := provider.GetProviderRequestFromLog("paycell", request.PaymentID, "msisdn")
	if err != nil {
		return nil, fmt.Errorf("failed to get phone number: %s %w", request.PaymentID, err)
	}
	p.phoneNumber = msisdn

	// Set clientIP for refund operation - use a default if not available
	if p.clientIP == "" {
		p.clientIP = "127.0.0.1" // Default fallback
	}

	// Prepare request according to PayCell documentation
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	// Convert amount to kuruş (multiply by 100)
	amountInKurus := strconv.FormatFloat(request.RefundAmount*100, 'f', 0, 64)

	// Create request structure as shown in documentation
	paycellReq := map[string]any{
		"msisdn":                  p.phoneNumber,
		"merchantCode":            p.merchantID,
		"originalReferenceNumber": originalReferenceNumber,
		"referenceNumber":         p.generateReferenceNumber(),
		"amount":                  amountInKurus,
		"pointAmount":             "", // Empty as shown in example
		"requestHeader": map[string]any{
			"applicationName":     p.username,
			"applicationPwd":      p.password,
			"clientIPAddress":     p.clientIP,
			"transactionDateTime": transactionDateTime,
			"transactionId":       transactionID,
		},
	}

	// Make HTTP request to refundAll endpoint
	url := p.baseURL + endpointRefundAll

	jsonData, err := json.Marshal(paycellReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal refund request: %w", err)
	}

	// Add provider request to client request log
	_ = provider.AddProviderRequestToClientRequest("paycell", "refundRequest", paycellReq, p.logID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create refund request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send refund request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refund response: %w", err)
	}

	var refundResp PaycellReverseResponse // Using same response structure as reverse
	if err := json.Unmarshal(body, &refundResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal refund response: %w. Response body: %s", err, string(body))
	}

	// Convert to standard refund response
	now := time.Now()
	response := &provider.RefundResponse{
		Success:      refundResp.ResponseHeader.ResponseCode == "0",
		RefundID:     refundResp.ResponseHeader.TransactionID,
		PaymentID:    request.PaymentID,
		RefundAmount: request.RefundAmount,
		Status:       "refunded",
		Message:      refundResp.ResponseHeader.ResponseDescription,
		SystemTime:   &now,
		RawResponse:  refundResp,
	}

	// Set status and error code based on refund result
	if refundResp.ResponseHeader.ResponseCode != "0" {
		response.Success = false
		response.Status = "failed"
		response.ErrorCode = refundResp.ResponseHeader.ResponseCode
	}

	return response, nil
}

// ValidateWebhook validates Paycell webhook data
func (p *PaycellProvider) ValidateWebhook(ctx context.Context, data, headers map[string]string) (bool, map[string]string, error) {
	// Paycell doesn't have webhook validation in the same way
	// This is more for completion callbacks
	return true, data, nil
}

func (p *PaycellProvider) threeDSessionResult(ctx context.Context, callbackState *provider.CallbackState, data map[string]string) (*PaycellGetThreeDSessionResultResponse, error) {
	paymentID := callbackState.PaymentID
	if paymentID == "" {
		return nil, errors.New("paycell: paymentID is required")
	}

	// Prepare request according to PayCell documentation
	transactionID := p.generateTransactionID()
	transactionDateTime := p.generateTransactionDateTime()

	// Create request structure as shown in documentation
	paycellReq := map[string]any{
		"merchantCode":    p.merchantID,
		"msisdn":          p.phoneNumber,
		"threeDSessionId": callbackState.PaymentID,
		"requestHeader": map[string]any{
			"transactionId":       transactionID,
			"transactionDateTime": transactionDateTime,
			"clientIPAddress":     callbackState.ClientIP,
			"applicationName":     p.username,
			"applicationPwd":      p.password,
		},
	}

	// Make HTTP request to getThreeDSessionResult endpoint
	url := p.baseURL + endpointGetThreeDSessionResult

	jsonData, err := json.Marshal(paycellReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal getThreeDSessionResult request: %w", err)
	}

	// Add provider request to client request log
	if logID, err := strconv.ParseInt(data["logID"], 10, 64); err == nil {
		p.logID = logID
	}

	_ = provider.AddProviderRequestToClientRequest("paycell", "getThreeDSessionResultRequest", paycellReq, p.logID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create getThreeDSessionResult request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send getThreeDSessionResult request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read getThreeDSessionResult response: %w", err)
	}

	var threeDSessionResp PaycellGetThreeDSessionResultResponse
	if err := json.Unmarshal(body, &threeDSessionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal getThreeDSessionResult response: %w. Response body: %s", err, string(body))
	}

	return &threeDSessionResp, nil
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
		return p.provision3D(ctx, request, cardToken)
	}

	return p.provisionAll(ctx, request, cardToken, "")
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
func (p *PaycellProvider) provisionAll(ctx context.Context, request provider.PaymentRequest, cardToken string, threeDSessionID string) (*provider.PaymentResponse, error) {
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

	// Convert amount to kuruş (multiply by 100)
	amountInKurus := strconv.FormatFloat(request.Amount*100, 'f', 0, 64)

	paycellReq := PaycellProvisionRequest{
		ExtraParameters:         nil,
		RequestHeader:           requestHeader,
		AcquirerBankCode:        nil,
		Amount:                  amountInKurus,
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
		ThreeDSessionID:         &threeDSessionID,
	}

	return p.sendProvisionRequest(ctx, endpoint, paycellReq)
}

// provision3DWithToken processes a 3D secure payment with card token
func (p *PaycellProvider) provision3D(ctx context.Context, request provider.PaymentRequest, cardToken string) (*provider.PaymentResponse, error) {
	// First, get 3D session
	threeDSession, err := p.getThreeDSession(ctx, request, cardToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get 3D session: %w", err)
	}

	// Create encrypted state with all necessary callback information
	state := provider.CallbackState{
		TenantID:         int(request.TenantID),
		PaymentID:        threeDSession.ThreeDSessionId,
		OriginalCallback: request.CallbackURL,
		Amount:           request.Amount,
		Currency:         request.Currency,
		LogID:            p.logID,
		Provider:         "paycell",
		Environment:      request.Environment,
		Timestamp:        time.Now(),
		ClientIP:         request.ClientIP,
	}

	// Use short callback URL system with database storage
	gopayCallbackURL, err := provider.CreateShortCallbackURL(ctx, p.gopayBaseURL, "paycell", state)
	if err != nil {
		return nil, fmt.Errorf("failed to create short callback URL: %w", err)
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
		HTML:             p.generate3DSecureHTML(threeDSession.ThreeDSessionId, gopayCallbackURL),
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
			ClientIPAddress:     p.clientIP,
			TransactionDateTime: transactionDateTime,
			TransactionID:       transactionID,
		},
		MerchantCode:     p.merchantID,
		Msisdn:           msisdn,
		Amount:           fmt.Sprintf("%.0f", request.Amount*100), // Convert to kuruş
		InstallmentCount: 1,
		CardToken:        cardToken,
		TransactionType:  "AUTH",
		Target:           "MERCHANT",
	}

	// Make direct HTTP call instead of using sendProvisionRequest
	url := p.baseURL + endpoint

	jsonData, err := json.Marshal(paycellReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal getThreeDSession request: %w", err)
	}

	// add provider request to client request
	_ = provider.AddProviderRequestToClientRequest("paycell", "getThreeDSessionRequest", paycellReq, p.logID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create getThreeDSession request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send getThreeDSession request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read getThreeDSession response: %w", err)
	}

	var threeDSessionResp PaycellGetThreeDSessionResponse
	if err := json.Unmarshal(body, &threeDSessionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal getThreeDSession response: %w. Response body: %s", err, string(body))
	}

	// Check for success
	if threeDSessionResp.ResponseHeader.ResponseCode != "0" {
		return nil, fmt.Errorf("getThreeDSession error: %s - %s", threeDSessionResp.ResponseHeader.ResponseCode, threeDSessionResp.ResponseHeader.ResponseDescription)
	}

	return &threeDSessionResp, nil
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

// generateSignature generates MD5 signature (for backward compatibility)
func (p *PaycellProvider) generateSignature(data string) string {
	// Simple hash for testing
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
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
	CCAuthor        string               `json:"ccAuthor"`
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

// PaycellGetThreeDSessionResultResponse represents the response from getThreeDSessionResult endpoint
type PaycellGetThreeDSessionResultResponse struct {
	ExtraParameters       any                          `json:"extraParameters"`
	CurrentStep           string                       `json:"currentStep"`
	MdErrorMessage        string                       `json:"mdErrorMessage"`
	MdStatus              string                       `json:"mdStatus"`
	ThreeDOperationResult PaycellThreeDOperationResult `json:"threeDOperationResult"`
}

// PaycellThreeDOperationResult represents the 3D operation result within getThreeDSessionResult response
type PaycellThreeDOperationResult struct {
	ThreeDResult            string                `json:"threeDResult"`
	ThreeDResultDescription string                `json:"threeDResultDescription"`
	ResponseHeader          PaycellResponseHeader `json:"responseHeader"`
}

// PaycellReverseResponse represents the response from reverse endpoint
type PaycellReverseResponse struct {
	ReconciliationDate     string                `json:"reconciliationDate"`
	ApprovalCode           string                `json:"approvalCode"`
	RetryStatusCode        *string               `json:"retryStatusCode"`
	RetryStatusDescription *string               `json:"retryStatusDescription"`
	ResponseHeader         PaycellResponseHeader `json:"responseHeader"`
}

// PaycellInquireResponse represents the response from inquireAll endpoint
type PaycellInquireResponse struct {
	ExtraParameters   any                        `json:"extraParameters"`
	OrderID           string                     `json:"orderId"`
	AcquirerBankCode  string                     `json:"acquirerBankCode"`
	Status            string                     `json:"status"`
	PaymentMethodType string                     `json:"paymentMethodType"`
	ProvisionList     []PaycellProvisionListItem `json:"provisionList"`
	ResponseHeader    PaycellResponseHeader      `json:"responseHeader"`
}

// PaycellProvisionListItem represents an item in the provision list
type PaycellProvisionListItem struct {
	ProvisionType       string `json:"provisionType"`
	TransactionID       string `json:"transactionId"`
	Amount              string `json:"amount"`
	ApprovalCode        string `json:"approvalCode"`
	DateTime            string `json:"dateTime"`
	ReconciliationDate  string `json:"reconciliationDate"`
	ResponseCode        string `json:"responseCode"`
	ResponseDescription string `json:"responseDescription"`
}
