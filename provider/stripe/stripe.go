package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
)

const (
	// API URLs
	apiSandboxURL    = "https://api.stripe.com"
	apiProductionURL = "https://api.stripe.com"

	// API Endpoints
	endpointPaymentIntents        = "/v1/payment_intents"
	endpointPaymentIntentConfirm  = "/v1/payment_intents/%s/confirm" // %s will be replaced with payment intent ID
	endpointPaymentIntentRetrieve = "/v1/payment_intents/%s"         // %s will be replaced with payment intent ID
	endpointPaymentIntentCancel   = "/v1/payment_intents/%s/cancel"  // %s will be replaced with payment intent ID
	endpointRefunds               = "/v1/refunds"

	// Stripe Status Codes
	statusRequiresPaymentMethod = "requires_payment_method"
	statusRequiresConfirmation  = "requires_confirmation"
	statusRequiresAction        = "requires_action"
	statusProcessing            = "processing"
	statusRequiresCapture       = "requires_capture"
	statusCanceled              = "canceled"
	statusSucceeded             = "succeeded"

	// Default Values
	defaultCurrency = "USD"
	defaultTimeout  = 30 * time.Second
)

// StripeProvider implements the provider.PaymentProvider interface for Stripe
type StripeProvider struct {
	secretKey    string
	publicKey    string
	baseURL      string
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
	client       *http.Client
}

// NewProvider creates a new Stripe payment provider
func NewProvider() provider.PaymentProvider {
	return &StripeProvider{
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Initialize sets up the Stripe payment provider with authentication credentials
func (p *StripeProvider) Initialize(conf map[string]string) error {
	p.secretKey = conf["secretKey"]
	p.publicKey = conf["publicKey"]

	if p.secretKey == "" {
		return errors.New("stripe: secretKey is required")
	}

	// Set GoPay base URL for callbacks
	if gopayBaseURL, ok := conf["gopayBaseURL"]; ok && gopayBaseURL != "" {
		p.gopayBaseURL = gopayBaseURL
	} else {
		p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")
	}

	p.isProduction = conf["environment"] == "production"
	// Stripe uses the same base URL for both sandbox and production
	p.baseURL = apiProductionURL

	return nil
}

// CreatePayment makes a non-3D payment request
func (p *StripeProvider) CreatePayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if err := p.validatePaymentRequest(request, false); err != nil {
		return nil, fmt.Errorf("stripe: invalid payment request: %w", err)
	}

	return p.processPayment(ctx, request, false)
}

// Create3DPayment starts a 3D secure payment process
func (p *StripeProvider) Create3DPayment(ctx context.Context, request provider.PaymentRequest) (*provider.PaymentResponse, error) {
	if err := p.validatePaymentRequest(request, true); err != nil {
		return nil, fmt.Errorf("stripe: invalid 3D payment request: %w", err)
	}

	return p.processPayment(ctx, request, true)
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (p *StripeProvider) Complete3DPayment(ctx context.Context, paymentID string, conversationID string, data map[string]string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("stripe: paymentID is required for 3D completion")
	}

	// For Stripe, we need to confirm the PaymentIntent after 3D authentication
	confirmData := map[string]any{
		"return_url": fmt.Sprintf("%s/v1/callback/stripe", p.gopayBaseURL),
	}

	// Add any additional data from the callback
	for k, v := range data {
		if k != "return_url" {
			confirmData[k] = v
		}
	}

	return p.sendRequest(ctx, fmt.Sprintf(endpointPaymentIntentConfirm, paymentID), confirmData)
}

// GetPaymentStatus retrieves the current status of a payment
func (p *StripeProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("stripe: paymentID is required")
	}

	return p.sendRequest(ctx, fmt.Sprintf(endpointPaymentIntentRetrieve, paymentID), nil)
}

// CancelPayment cancels a payment
func (p *StripeProvider) CancelPayment(ctx context.Context, paymentID string, reason string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("stripe: paymentID is required")
	}

	cancelData := map[string]any{
		"cancellation_reason": "requested_by_customer",
	}

	if reason != "" {
		cancelData["cancellation_reason"] = reason
	}

	return p.sendRequest(ctx, fmt.Sprintf(endpointPaymentIntentCancel, paymentID), cancelData)
}

// RefundPayment issues a refund for a payment
func (p *StripeProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("stripe: paymentID is required for refund")
	}

	refundData := map[string]any{
		"payment_intent": request.PaymentID,
	}

	if request.RefundAmount > 0 {
		// Convert to cents
		refundData["amount"] = int64(request.RefundAmount * 100)
	}

	if request.Reason != "" {
		refundData["reason"] = request.Reason
	}

	if request.Description != "" {
		refundData["metadata"] = map[string]string{
			"description": request.Description,
		}
	}

	resp, err := p.sendRequest(ctx, endpointRefunds, refundData)
	if err != nil {
		return nil, err
	}

	// Map to RefundResponse
	refundResp := &provider.RefundResponse{
		Success:     resp.Success,
		PaymentID:   request.PaymentID,
		Status:      "succeeded", // Stripe refunds are typically immediate
		Message:     resp.Message,
		ErrorCode:   resp.ErrorCode,
		SystemTime:  time.Now(),
		RawResponse: resp.ProviderResponse,
	}

	if providerResp, ok := resp.ProviderResponse.(map[string]any); ok {
		if refundID, ok := providerResp["id"].(string); ok {
			refundResp.RefundID = refundID
		}
		if amount, ok := providerResp["amount"].(float64); ok {
			refundResp.RefundAmount = amount / 100 // Convert back from cents
		}
	}

	return refundResp, nil
}

// ValidateWebhook validates an incoming webhook notification
func (p *StripeProvider) ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	// Stripe webhook validation would go here
	// For now, we'll return true to indicate validation passed
	return true, data, nil
}

// Helper method to validate payment request
func (p *StripeProvider) validatePaymentRequest(request provider.PaymentRequest, is3D bool) error {
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

	if request.CardInfo.ExpireMonth == "" || request.CardInfo.ExpireYear == "" {
		return errors.New("card expiry month and year are required")
	}

	if request.CardInfo.CVV == "" {
		return errors.New("card CVV is required")
	}

	if is3D && request.CallbackURL == "" {
		return errors.New("callback URL is required for 3D secure payments")
	}

	return nil
}

// Helper method to process a payment
func (p *StripeProvider) processPayment(ctx context.Context, request provider.PaymentRequest, force3D bool) (*provider.PaymentResponse, error) {
	// Create PaymentIntent first
	intentData := p.mapToStripePaymentIntentRequest(request, force3D)

	response, err := p.sendRequest(ctx, endpointPaymentIntents, intentData)
	if err != nil {
		return nil, err
	}

	// If it's not 3D, we need to confirm the payment immediately
	if !force3D {
		if providerResp, ok := response.ProviderResponse.(map[string]any); ok {
			if paymentIntentID, ok := providerResp["id"].(string); ok {
				confirmData := map[string]any{
					"payment_method": intentData["payment_method"],
				}
				return p.sendRequest(ctx, fmt.Sprintf(endpointPaymentIntentConfirm, paymentIntentID), confirmData)
			}
		}
	}

	return response, nil
}

// Helper method to map our common request to Stripe PaymentIntent format
func (p *StripeProvider) mapToStripePaymentIntentRequest(request provider.PaymentRequest, force3D bool) map[string]any {
	// Convert amount to cents
	amountInCents := int64(request.Amount * 100)

	// Create payment method data
	paymentMethodData := map[string]any{
		"type": "card",
		"card": map[string]any{
			"number":    request.CardInfo.CardNumber,
			"exp_month": request.CardInfo.ExpireMonth,
			"exp_year":  request.CardInfo.ExpireYear,
			"cvc":       request.CardInfo.CVV,
		},
		"billing_details": map[string]any{
			"name":  fmt.Sprintf("%s %s", request.Customer.Name, request.Customer.Surname),
			"email": request.Customer.Email,
		},
	}

	// Add address if available
	if request.Customer.Address.Address != "" {
		paymentMethodData["billing_details"].(map[string]any)["address"] = map[string]any{
			"line1":       request.Customer.Address.Address,
			"city":        request.Customer.Address.City,
			"country":     request.Customer.Address.Country,
			"postal_code": request.Customer.Address.ZipCode,
		}
	}

	// Build PaymentIntent request
	intentData := map[string]any{
		"amount":              amountInCents,
		"currency":            strings.ToLower(request.Currency),
		"payment_method_data": paymentMethodData,
		"confirmation_method": "manual",
		"capture_method":      "automatic",
	}

	// Add description if provided
	if request.Description != "" {
		intentData["description"] = request.Description
	}

	// Add metadata
	metadata := map[string]string{
		"reference_id": request.ReferenceID,
	}
	if request.ConversationID != "" {
		metadata["conversation_id"] = request.ConversationID
	}
	intentData["metadata"] = metadata

	// Configure 3D Secure
	if force3D {
		intentData["payment_method_options"] = map[string]any{
			"card": map[string]any{
				"request_three_d_secure": "any",
			},
		}

		// Add return URL for 3D Secure
		if request.CallbackURL != "" {
			intentData["return_url"] = fmt.Sprintf("%s/v1/callback/stripe?originalCallbackUrl=%s",
				p.gopayBaseURL, request.CallbackURL)
		} else {
			intentData["return_url"] = fmt.Sprintf("%s/v1/callback/stripe", p.gopayBaseURL)
		}
	} else {
		intentData["payment_method_options"] = map[string]any{
			"card": map[string]any{
				"request_three_d_secure": "automatic",
			},
		}
	}

	return intentData
}

// sendRequest sends HTTP requests to Stripe and maps the response
func (p *StripeProvider) sendRequest(ctx context.Context, endpoint string, requestData map[string]any) (*provider.PaymentResponse, error) {
	url := p.baseURL + endpoint

	var body io.Reader
	if requestData != nil {
		jsonData, err := json.Marshal(requestData)
		if err != nil {
			return nil, fmt.Errorf("stripe: failed to marshal request data: %w", err)
		}
		body = strings.NewReader(string(jsonData))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("stripe: failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.secretKey)
	req.Header.Set("Stripe-Version", "2020-08-27") // Use a stable API version

	// For GET requests (like retrieve payment status)
	if requestData == nil {
		req.Method = "GET"
		req.Body = nil
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("stripe: failed to read response: %w", err)
	}

	var stripeResp map[string]any
	if err := json.Unmarshal(respBody, &stripeResp); err != nil {
		return nil, fmt.Errorf("stripe: failed to parse response: %w", err)
	}

	// Map Stripe response to our common PaymentResponse
	return p.mapToPaymentResponse(stripeResp, resp.StatusCode), nil
}

// Helper method to map Stripe response to common PaymentResponse format
func (p *StripeProvider) mapToPaymentResponse(response map[string]any, statusCode int) *provider.PaymentResponse {
	paymentResp := &provider.PaymentResponse{
		Success:          statusCode >= 200 && statusCode < 300,
		SystemTime:       time.Now(),
		ProviderResponse: response,
	}

	if !paymentResp.Success {
		// Handle error response
		if errorData, ok := response["error"].(map[string]any); ok {
			if message, ok := errorData["message"].(string); ok {
				paymentResp.Message = message
			}
			if code, ok := errorData["code"].(string); ok {
				paymentResp.ErrorCode = code
			}
		}
		paymentResp.Status = provider.StatusFailed
		return paymentResp
	}

	// Extract payment information
	if id, ok := response["id"].(string); ok {
		paymentResp.PaymentID = id
	}

	if amount, ok := response["amount"].(float64); ok {
		paymentResp.Amount = amount / 100 // Convert from cents
	}

	if currency, ok := response["currency"].(string); ok {
		paymentResp.Currency = strings.ToUpper(currency)
	}

	// Map Stripe status to our common status
	if status, ok := response["status"].(string); ok {
		switch status {
		case statusSucceeded:
			paymentResp.Status = provider.StatusSuccessful
			paymentResp.Message = "Payment successful"
		case statusRequiresAction, statusRequiresConfirmation:
			paymentResp.Status = provider.StatusPending
			paymentResp.Message = "Payment requires additional action"

			// Extract next action for 3D Secure
			if nextAction, ok := response["next_action"].(map[string]any); ok {
				if redirectToURL, ok := nextAction["redirect_to_url"].(map[string]any); ok {
					if redirectURL, ok := redirectToURL["url"].(string); ok {
						paymentResp.RedirectURL = redirectURL
					}
				}
			}
		case statusProcessing, statusRequiresCapture:
			paymentResp.Status = provider.StatusProcessing
			paymentResp.Message = "Payment is being processed"
		case statusCanceled:
			paymentResp.Status = provider.StatusCancelled
			paymentResp.Message = "Payment was cancelled"
		case statusRequiresPaymentMethod:
			paymentResp.Status = provider.StatusFailed
			paymentResp.Message = "Payment failed - invalid payment method"
		default:
			paymentResp.Status = provider.StatusPending
			paymentResp.Message = fmt.Sprintf("Payment status: %s", status)
		}
	}

	// Extract transaction ID if available
	if charges, ok := response["charges"].(map[string]any); ok {
		if data, ok := charges["data"].([]any); ok && len(data) > 0 {
			if charge, ok := data[0].(map[string]any); ok {
				if chargeID, ok := charge["id"].(string); ok {
					paymentResp.TransactionID = chargeID
				}
			}
		}
	}

	return paymentResp
}
