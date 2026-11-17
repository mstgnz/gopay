package provider

import (
	"context"
)

// CheckoutService runs hosted-checkout operations: it resolves the tenant's provider, checks the
// HostedCheckoutProvider capability, validates the request and logs each call like PaymentService.
type CheckoutService struct {
	log operationLog
}

func NewCheckoutService(logger PaymentLogger) *CheckoutService {
	return &CheckoutService{log: operationLog{logger: logger}}
}

func checkoutProvider(ctx context.Context, providerName, environment string) (HostedCheckoutProvider, int, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	p, err := GetProvider(tenantID, providerName, environment)
	if err != nil {
		return nil, 0, err
	}
	hc, ok := p.(HostedCheckoutProvider)
	if !ok {
		return nil, 0, ErrCheckoutUnsupported
	}
	return hc, tenantID, nil
}

// StartCheckout opens the provider's payment page for a basket.
func (s *CheckoutService) StartCheckout(ctx context.Context, environment, providerName string, request HostedCheckoutRequest) (*HostedCheckoutSession, error) {
	hc, tenantID, err := checkoutProvider(ctx, providerName, environment)
	if err != nil {
		return nil, err
	}
	if err := ValidateHostedCheckout(request, environment); err != nil {
		return nil, err
	}

	logID, started := s.log.start(ctx, tenantID, providerName, "POST", "/checkout", request, request.Buyer.IP)
	request.LogID = logID
	resp, err := hc.StartHostedCheckout(ctx, request)
	s.log.finish(ctx, providerName, logID, started, resp, err)
	return resp, err
}

// CheckoutResult reads the verdict on a hosted payment by its token.
func (s *CheckoutService) CheckoutResult(ctx context.Context, environment, providerName string, request HostedCheckoutResultRequest) (*HostedCheckoutResult, error) {
	hc, tenantID, err := checkoutProvider(ctx, providerName, environment)
	if err != nil {
		return nil, err
	}
	if request.Token == "" {
		return nil, invalid("token is required")
	}

	logID, started := s.log.start(ctx, tenantID, providerName, "GET", "/checkout/result", request, "")
	request.LogID = logID
	resp, err := hc.HostedCheckoutResult(ctx, request)
	s.log.finish(ctx, providerName, logID, started, resp, err)
	return resp, err
}

// CancelPayment voids a whole hosted payment.
func (s *CheckoutService) CancelPayment(ctx context.Context, environment, providerName string, request HostedCancelRequest) (*HostedCancelResult, error) {
	hc, tenantID, err := checkoutProvider(ctx, providerName, environment)
	if err != nil {
		return nil, err
	}
	if err := ValidateHostedCancel(request); err != nil {
		return nil, err
	}

	logID, started := s.log.start(ctx, tenantID, providerName, "POST", "/checkout/cancel", request, request.IP)
	request.LogID = logID
	resp, err := hc.CancelHostedPayment(ctx, request)
	s.log.finish(ctx, providerName, logID, started, resp, err)
	return resp, err
}

// RefundItem returns part or all of one paid basket item.
func (s *CheckoutService) RefundItem(ctx context.Context, environment, providerName string, request HostedRefundRequest) (*HostedRefundResult, error) {
	hc, tenantID, err := checkoutProvider(ctx, providerName, environment)
	if err != nil {
		return nil, err
	}
	if err := ValidateHostedRefund(request); err != nil {
		return nil, err
	}

	logID, started := s.log.start(ctx, tenantID, providerName, "POST", "/checkout/refund", request, request.IP)
	request.LogID = logID
	resp, err := hc.RefundHostedItem(ctx, request)
	s.log.finish(ctx, providerName, logID, started, resp, err)
	return resp, err
}
