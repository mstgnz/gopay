package iyzico

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mstgnz/gopay/provider"
)

// IyzicoProvider implements provider.PaymentProvider plus the optional HostedCheckoutProvider and
// MarketplaceProvider capabilities. Card data never passes through it: payments start on iyzico's
// Checkout Form, so the card-based PaymentProvider methods refuse.
type IyzicoProvider struct {
	client       *client
	isProduction bool
}

var (
	_ provider.PaymentProvider        = (*IyzicoProvider)(nil)
	_ provider.HostedCheckoutProvider = (*IyzicoProvider)(nil)
	_ provider.MarketplaceProvider    = (*IyzicoProvider)(nil)
)

// ErrUseCheckout answers the card-based and installment calls: iyzico payments go through
// POST /v1/payments/iyzico/checkout.
var ErrUseCheckout = errors.New("iyzico: not supported; start the payment with POST /v1/payments/iyzico/checkout")

func NewProvider() provider.PaymentProvider {
	return &IyzicoProvider{}
}

// Clone returns a per-request copy. The provider holds no per-request state; the client is
// read-only and shared.
func (p *IyzicoProvider) Clone() provider.PaymentProvider {
	c := *p
	return &c
}

func (p *IyzicoProvider) GetRequiredConfig(environment string) []provider.ConfigField {
	return []provider.ConfigField{
		{Key: "apiKey", Required: true, Type: "string", Description: "iyzico API key (merchant panel, Settings > Company Settings)",
			Example: "sandbox-xxxxxxxxxxxxxxxxxxxxxxxx", MinLength: 20, MaxLength: 200},
		{Key: "secretKey", Required: true, Type: "string", Description: "iyzico secret key (merchant panel, Settings > Company Settings)",
			Example: "sandbox-xxxxxxxxxxxxxxxxxxxxxxxx", MinLength: 20, MaxLength: 200},
		{Key: "environment", Required: true, Type: "string", Description: "Environment setting (sandbox or production)",
			Example: "sandbox", Pattern: "^(sandbox|production)$"},
	}
}

func (p *IyzicoProvider) ValidateConfig(config map[string]string) error {
	return provider.ValidateConfigFields("iyzico", config, p.GetRequiredConfig(config["environment"]))
}

// Initialize reads the tenant's key pair. Anything but "production" is the sandbox.
func (p *IyzicoProvider) Initialize(conf map[string]string) error {
	if conf["apiKey"] == "" || conf["secretKey"] == "" {
		return errors.New("iyzico: apiKey and secretKey are required")
	}
	p.isProduction = conf["environment"] == "production"
	baseURL := apiSandboxURL
	if p.isProduction {
		baseURL = apiProductionURL
	}
	p.client = newClient(conf["apiKey"], conf["secretKey"], baseURL)
	return nil
}

func (p *IyzicoProvider) CreatePayment(context.Context, provider.PaymentRequest) (*provider.PaymentResponse, error) {
	return nil, ErrUseCheckout
}

func (p *IyzicoProvider) Create3DPayment(context.Context, provider.PaymentRequest) (*provider.PaymentResponse, error) {
	return nil, ErrUseCheckout
}

func (p *IyzicoProvider) Complete3DPayment(context.Context, *provider.CallbackState, map[string]string) (*provider.PaymentResponse, error) {
	return nil, ErrUseCheckout
}

func (p *IyzicoProvider) GetInstallmentCount(context.Context, provider.InstallmentInquireRequest) (provider.InstallmentInquireResponse, error) {
	return provider.InstallmentInquireResponse{}, ErrUseCheckout
}

func (p *IyzicoProvider) GetCommission(context.Context, provider.CommissionRequest) (provider.CommissionResponse, error) {
	return provider.CommissionResponse{}, ErrUseCheckout
}

// ValidateWebhook: iyzico merchant notifications are not wired yet; a webhook is never accepted.
func (p *IyzicoProvider) ValidateWebhook(context.Context, map[string]string, map[string]string) (bool, map[string]string, error) {
	return false, nil, errors.New("iyzico: webhooks are not supported yet")
}

const pathPaymentDetail = "/payment/detail"

type paymentDetailAnswer struct {
	PaymentStatus  string     `json:"paymentStatus"`
	PaymentID      flexString `json:"paymentId"`
	ConversationID string     `json:"conversationId"`
	BasketID       string     `json:"basketId"`
	Currency       string     `json:"currency"`
	Price          amount     `json:"price"`
	PaidPrice      amount     `json:"paidPrice"`
	FraudStatus    flexInt    `json:"fraudStatus"`
	Signature      string     `json:"signature"`
}

// GetPaymentStatus serves the common status route by iyzico paymentId (/payment/detail). The
// common response carries float64 amounts; exact amounts and item lines are on the checkout route.
// An error answer is pending, never failed; a transport failure is an error.
func (p *IyzicoProvider) GetPaymentStatus(ctx context.Context, r provider.GetPaymentStatusRequest) (*provider.PaymentResponse, error) {
	if r.PaymentID == "" {
		return nil, errors.New("iyzico: paymentId is required")
	}
	body := map[string]string{"locale": defaultLocale, "conversationId": conversationID(r.ConversationID), "paymentId": r.PaymentID}

	var a paymentDetailAnswer
	err := p.client.do(ctx, http.MethodPost, pathPaymentDetail, body, &a)
	var refusal *provider.ProviderRefusal
	if errors.As(err, &refusal) {
		return &provider.PaymentResponse{Success: false, Status: provider.StatusPending, PaymentID: r.PaymentID,
			ErrorCode: refusal.Code, Message: refusal.Message}, nil
	}
	if err != nil {
		return nil, err
	}

	resp := &provider.PaymentResponse{PaymentID: orDefault(string(a.PaymentID), r.PaymentID), OrderID: a.BasketID,
		Currency: a.Currency, Status: provider.StatusPending, Message: a.PaymentStatus}
	if a.FraudStatus.set {
		resp.FraudStatus = a.FraudStatus.value
	}
	if paid, err := strconv.ParseFloat(string(a.PaidPrice), 64); err == nil {
		resp.Amount = paid
	}
	switch {
	case a.PaymentStatus == paymentStatusFailure, a.FraudStatus.set && a.FraudStatus.value == -1:
		resp.Status = provider.StatusFailed
	case a.PaymentStatus == paymentStatusSuccess && a.FraudStatus.set && a.FraudStatus.value == 1:
		resp.Status, resp.Success = provider.StatusSuccessful, true
	}
	// As on the checkout route, a verdict counts only on an answer iyzico signed.
	signed := p.client.signatureMatches(a.Signature, string(a.PaymentID), a.Currency, a.BasketID,
		a.ConversationID, string(a.PaidPrice), string(a.Price))
	if !signed && (resp.Status != provider.StatusPending || a.Signature != "") {
		resp.Status, resp.Success, resp.ErrorCode = provider.StatusPending, false, codeSignatureMismatch
	}
	return resp, nil
}

// CancelPayment serves the common cancel route. It sends no IP: the common request has none.
func (p *IyzicoProvider) CancelPayment(ctx context.Context, r provider.CancelRequest) (*provider.PaymentResponse, error) {
	if r.PaymentID == "" {
		return nil, errors.New("iyzico: paymentId is required")
	}
	body := map[string]string{"locale": defaultLocale, "conversationId": conversationID(r.ConversationID), "paymentId": r.PaymentID}

	var a cancelAnswer
	err := p.client.do(ctx, http.MethodPost, pathCancel, body, &a)
	var refusal *provider.ProviderRefusal
	if errors.As(err, &refusal) {
		return &provider.PaymentResponse{Success: false, PaymentID: r.PaymentID, ErrorCode: refusal.Code, Message: refusal.Message}, nil
	}
	if err != nil {
		return nil, err
	}
	return &provider.PaymentResponse{Success: true, Status: provider.StatusCancelled, PaymentID: r.PaymentID,
		Currency: a.Currency, Message: "Payment cancelled"}, nil
}

// RefundPayment serves the common refund route: PaymentID is the iyzico paymentTransactionId of
// one basket item, as it always was for this provider. The common request carries a float64
// amount; exact refunds go through POST /v1/payments/iyzico/checkout/refund.
func (p *IyzicoProvider) RefundPayment(ctx context.Context, r provider.RefundRequest) (*provider.RefundResponse, error) {
	if r.PaymentID == "" {
		return nil, errors.New("iyzico: paymentId (the item's paymentTransactionId) is required")
	}
	if r.RefundAmount <= 0 {
		return nil, errors.New("iyzico: refundAmount must be greater than 0")
	}
	body := map[string]string{"locale": defaultLocale, "conversationId": conversationID(r.ConversationID),
		"paymentTransactionId": r.PaymentID, "price": fmt.Sprintf("%.2f", r.RefundAmount),
		"currency": orDefault(r.Currency, provider.CheckoutCurrencyTRY)}

	var a refundAnswer
	err := p.client.do(ctx, http.MethodPost, pathRefund, body, &a)
	var refusal *provider.ProviderRefusal
	if errors.As(err, &refusal) {
		return &provider.RefundResponse{Success: false, PaymentID: r.PaymentID, ErrorCode: refusal.Code, Message: refusal.Message}, nil
	}
	if err != nil {
		return nil, err
	}
	if a.Signature != "" && !p.client.signatureMatches(a.Signature, string(a.PaymentID), string(a.Price), a.Currency, a.ConversationID) {
		return nil, unavailable("refund answer signature does not match")
	}
	return &provider.RefundResponse{Success: true, Status: "success", PaymentID: r.PaymentID,
		RefundID: string(a.PaymentTransactionID), RefundAmount: r.RefundAmount, Message: "Refund successful"}, nil
}
