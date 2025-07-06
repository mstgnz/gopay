package stripe

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/provider"
	"github.com/stripe/stripe-go/v82"
)

// Helper function to parse string to int64
func parseInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

// StripeProvider implements the provider.PaymentProvider interface for Stripe
type StripeProvider struct {
	client       *stripe.Client
	gopayBaseURL string // GoPay's own base URL for callbacks
	isProduction bool
}

// NewProvider creates a new Stripe payment provider
func NewProvider() provider.PaymentProvider {
	return &StripeProvider{}
}

// Initialize sets up the Stripe payment provider with authentication credentials
func (p *StripeProvider) Initialize(conf map[string]string) error {
	secretKey := conf["secretKey"]
	if secretKey == "" {
		return errors.New("stripe: secretKey is required")
	}

	// Set GoPay base URL for callbacks
	if gopayBaseURL, ok := conf["gopayBaseURL"]; ok && gopayBaseURL != "" {
		p.gopayBaseURL = gopayBaseURL
	} else {
		p.gopayBaseURL = config.GetEnv("APP_URL", "http://localhost:9999")
	}

	p.isProduction = conf["environment"] == "production"

	// Initialize Stripe client with the new approach
	p.client = stripe.NewClient(secretKey)

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

	// Confirm the PaymentIntent after 3D authentication
	params := &stripe.PaymentIntentConfirmParams{
		ReturnURL: stripe.String(fmt.Sprintf("%s/v1/callback/stripe", p.gopayBaseURL)),
	}

	pi, err := p.client.V1PaymentIntents.Confirm(ctx, paymentID, params)
	if err != nil {
		return nil, fmt.Errorf("stripe: failed to confirm payment intent: %w", err)
	}

	return p.mapPaymentIntentToResponse(pi), nil
}

// GetPaymentStatus retrieves the current status of a payment
func (p *StripeProvider) GetPaymentStatus(ctx context.Context, paymentID string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("stripe: paymentID is required")
	}

	pi, err := p.client.V1PaymentIntents.Retrieve(ctx, paymentID, nil)
	if err != nil {
		return nil, fmt.Errorf("stripe: failed to get payment intent: %w", err)
	}

	return p.mapPaymentIntentToResponse(pi), nil
}

// CancelPayment cancels a payment
func (p *StripeProvider) CancelPayment(ctx context.Context, paymentID string, reason string) (*provider.PaymentResponse, error) {
	if paymentID == "" {
		return nil, errors.New("stripe: paymentID is required")
	}

	params := &stripe.PaymentIntentCancelParams{
		CancellationReason: stripe.String("requested_by_customer"),
	}

	pi, err := p.client.V1PaymentIntents.Cancel(ctx, paymentID, params)
	if err != nil {
		return nil, fmt.Errorf("stripe: failed to cancel payment intent: %w", err)
	}

	return p.mapPaymentIntentToResponse(pi), nil
}

// RefundPayment issues a refund for a payment
func (p *StripeProvider) RefundPayment(ctx context.Context, request provider.RefundRequest) (*provider.RefundResponse, error) {
	if request.PaymentID == "" {
		return nil, errors.New("stripe: paymentID is required for refund")
	}

	params := &stripe.RefundCreateParams{
		PaymentIntent: stripe.String(request.PaymentID),
	}

	if request.RefundAmount > 0 {
		// Convert to cents
		params.Amount = stripe.Int64(int64(request.RefundAmount * 100))
	}

	if request.Reason != "" {
		params.Reason = stripe.String(request.Reason)
	}

	if request.Description != "" {
		params.Metadata = map[string]string{
			"description": request.Description,
		}
	}

	ref, err := p.client.V1Refunds.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("stripe: failed to create refund: %w", err)
	}

	now := time.Now()
	return &provider.RefundResponse{
		Success:      true,
		RefundID:     ref.ID,
		PaymentID:    request.PaymentID,
		RefundAmount: float64(ref.Amount) / 100, // Convert back from cents
		Status:       "succeeded",
		Message:      "Refund successful",
		SystemTime:   &now,
		RawResponse:  ref,
	}, nil
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
	// Step 1: Create PaymentMethod
	pmParams := &stripe.PaymentMethodCreateParams{
		Type: stripe.String("card"),
		Card: &stripe.PaymentMethodCreateCardParams{
			Number:   stripe.String(request.CardInfo.CardNumber),
			ExpMonth: stripe.Int64(parseInt64(request.CardInfo.ExpireMonth)),
			ExpYear:  stripe.Int64(parseInt64(request.CardInfo.ExpireYear)),
			CVC:      stripe.String(request.CardInfo.CVV),
		},
		BillingDetails: &stripe.PaymentMethodCreateBillingDetailsParams{
			Name:  stripe.String(fmt.Sprintf("%s %s", request.Customer.Name, request.Customer.Surname)),
			Email: stripe.String(request.Customer.Email),
		},
	}

	// Add address if available
	if request.Customer.Address.Address != "" {
		pmParams.BillingDetails.Address = &stripe.AddressParams{
			Line1:      stripe.String(request.Customer.Address.Address),
			City:       stripe.String(request.Customer.Address.City),
			Country:    stripe.String(request.Customer.Address.Country),
			PostalCode: stripe.String(request.Customer.Address.ZipCode),
		}
	}

	pm, err := p.client.V1PaymentMethods.Create(ctx, pmParams)
	if err != nil {
		return nil, fmt.Errorf("stripe: failed to create payment method: %w", err)
	}

	// Step 2: Create PaymentIntent
	piParams := &stripe.PaymentIntentCreateParams{
		Amount:             stripe.Int64(int64(request.Amount * 100)), // Convert to cents
		Currency:           stripe.String(strings.ToLower(request.Currency)),
		PaymentMethod:      stripe.String(pm.ID),
		ConfirmationMethod: stripe.String("manual"),
		CaptureMethod:      stripe.String("automatic"),
		// Only accept card payments
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Metadata: map[string]string{
			"reference_id": request.ReferenceID,
		},
	}

	if request.Description != "" {
		piParams.Description = stripe.String(request.Description)
	}

	if request.ConversationID != "" {
		piParams.Metadata["conversation_id"] = request.ConversationID
	}

	// Configure 3D Secure but don't add return_url here
	if force3D {
		piParams.PaymentMethodOptions = &stripe.PaymentIntentCreatePaymentMethodOptionsParams{
			Card: &stripe.PaymentIntentCreatePaymentMethodOptionsCardParams{
				RequestThreeDSecure: stripe.String("any"),
			},
		}
	} else {
		piParams.PaymentMethodOptions = &stripe.PaymentIntentCreatePaymentMethodOptionsParams{
			Card: &stripe.PaymentIntentCreatePaymentMethodOptionsCardParams{
				RequestThreeDSecure: stripe.String("automatic"),
			},
		}
	}

	pi, err := p.client.V1PaymentIntents.Create(ctx, piParams)
	if err != nil {
		return nil, fmt.Errorf("stripe: failed to create payment intent: %w", err)
	}

	// Step 3: If it's not 3D, confirm the payment immediately
	if !force3D {
		confirmParams := &stripe.PaymentIntentConfirmParams{
			PaymentMethod: stripe.String(pm.ID),
			ReturnURL:     stripe.String(fmt.Sprintf("%s/v1/callback/stripe", p.gopayBaseURL)),
		}

		// Add tenant ID to return URL if available
		if request.TenantID != "" {
			returnURL := fmt.Sprintf("%s/v1/callback/stripe?tenantId=%s", p.gopayBaseURL, request.TenantID)
			confirmParams.ReturnURL = stripe.String(returnURL)
		}

		pi, err = p.client.V1PaymentIntents.Confirm(ctx, pi.ID, confirmParams)
		if err != nil {
			return nil, fmt.Errorf("stripe: failed to confirm payment intent: %w", err)
		}
	} else {
		// For 3D payments, set return URL during creation (this will be used when user confirms)
		updateParams := &stripe.PaymentIntentParams{
			ReturnURL: stripe.String(fmt.Sprintf("%s/v1/callback/stripe", p.gopayBaseURL)),
		}

		// Add custom return URL for 3D Secure if provided
		if request.CallbackURL != "" {
			returnURL := fmt.Sprintf("%s/v1/callback/stripe?originalCallbackUrl=%s", p.gopayBaseURL, request.CallbackURL)
			if request.TenantID != "" {
				returnURL += fmt.Sprintf("&tenantId=%s", request.TenantID)
			}
			updateParams.ReturnURL = stripe.String(returnURL)
		} else if request.TenantID != "" {
			returnURL := fmt.Sprintf("%s/v1/callback/stripe?tenantId=%s", p.gopayBaseURL, request.TenantID)
			updateParams.ReturnURL = stripe.String(returnURL)
		}

		// Update the PaymentIntent with return URL, then confirm it
		confirmParams := &stripe.PaymentIntentConfirmParams{
			PaymentMethod: stripe.String(pm.ID),
			ReturnURL:     updateParams.ReturnURL,
		}

		pi, err = p.client.V1PaymentIntents.Confirm(ctx, pi.ID, confirmParams)
		if err != nil {
			return nil, fmt.Errorf("stripe: failed to confirm 3D payment intent: %w", err)
		}
	}

	return p.mapPaymentIntentToResponse(pi), nil
}

// Helper method to map Stripe PaymentIntent to our PaymentResponse
func (p *StripeProvider) mapPaymentIntentToResponse(pi *stripe.PaymentIntent) *provider.PaymentResponse {
	now := time.Now()
	response := &provider.PaymentResponse{
		PaymentID:        pi.ID,
		Amount:           float64(pi.Amount) / 100, // Convert from cents
		Currency:         strings.ToUpper(string(pi.Currency)),
		SystemTime:       &now,
		ProviderResponse: pi,
	}

	// Map Stripe status to our common status
	switch pi.Status {
	case stripe.PaymentIntentStatusSucceeded:
		response.Success = true
		response.Status = provider.StatusSuccessful
		response.Message = "Payment successful"
	case stripe.PaymentIntentStatusRequiresAction, stripe.PaymentIntentStatusRequiresConfirmation:
		response.Success = true
		response.Status = provider.StatusPending
		response.Message = "Payment requires additional action"

		// Extract redirect URL for 3D Secure
		if pi.NextAction != nil && pi.NextAction.RedirectToURL != nil {
			response.RedirectURL = pi.NextAction.RedirectToURL.URL
		}
	case stripe.PaymentIntentStatusProcessing, stripe.PaymentIntentStatusRequiresCapture:
		response.Success = true
		response.Status = provider.StatusProcessing
		response.Message = "Payment is being processed"
	case stripe.PaymentIntentStatusCanceled:
		response.Success = false
		response.Status = provider.StatusCancelled
		response.Message = "Payment was cancelled"
	case stripe.PaymentIntentStatusRequiresPaymentMethod:
		response.Success = false
		response.Status = provider.StatusFailed
		response.Message = "Payment failed - invalid payment method"
	default:
		response.Success = false
		response.Status = provider.StatusFailed
		response.Message = fmt.Sprintf("Payment status: %s", pi.Status)
	}

	// Extract transaction ID - we'll use the latest charge ID if available
	if pi.LatestCharge != nil {
		response.TransactionID = pi.LatestCharge.ID
	}

	return response
}
