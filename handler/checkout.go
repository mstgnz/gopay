package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/response"
	"github.com/mstgnz/gopay/provider"
)

// CheckoutServiceInterface is what the checkout routes need from provider.CheckoutService.
type CheckoutServiceInterface interface {
	StartCheckout(ctx context.Context, environment, providerName string, request provider.HostedCheckoutRequest) (*provider.HostedCheckoutSession, error)
	CheckoutResult(ctx context.Context, environment, providerName string, request provider.HostedCheckoutResultRequest) (*provider.HostedCheckoutResult, error)
	CancelPayment(ctx context.Context, environment, providerName string, request provider.HostedCancelRequest) (*provider.HostedCancelResult, error)
	RefundItem(ctx context.Context, environment, providerName string, request provider.HostedRefundRequest) (*provider.HostedRefundResult, error)
}

// CheckoutHandler serves the hosted-checkout routes. Card data never reaches it: the customer pays
// on the provider's page.
type CheckoutHandler struct {
	service  CheckoutServiceInterface
	validate *validator.Validate
}

func NewCheckoutHandler(service CheckoutServiceInterface, validate *validator.Validate) *CheckoutHandler {
	return &CheckoutHandler{service: service, validate: validate}
}

// Request bodies are handler-local DTOs: only client-facing fields bind; log ids are set server-side.
// Amount formats and the money rules are checked in provider.ValidateHostedCheckout.

type checkoutBuyerBody struct {
	ID             string `json:"id" validate:"required,max=64"`
	Name           string `json:"name" validate:"required,max=100"`
	Surname        string `json:"surname" validate:"required,max=100"`
	IdentityNumber string `json:"identityNumber" validate:"required,min=5,max=20"`
	Email          string `json:"email" validate:"required,email,max=100"`
	GSMNumber      string `json:"gsmNumber" validate:"required,e164"`
	Address        string `json:"address" validate:"required,max=500"`
	City           string `json:"city" validate:"required,max=100"`
	Country        string `json:"country" validate:"omitempty,max=100"`
	ZipCode        string `json:"zipCode" validate:"omitempty,max=20"`
	IP             string `json:"ip" validate:"required,ip"`
}

type checkoutAddressBody struct {
	ContactName string `json:"contactName" validate:"required,max=200"`
	Address     string `json:"address" validate:"required,max=500"`
	City        string `json:"city" validate:"required,max=100"`
	Country     string `json:"country" validate:"omitempty,max=100"`
	ZipCode     string `json:"zipCode" validate:"omitempty,max=20"`
}

func (a *checkoutAddressBody) toProvider() *provider.CheckoutAddress {
	if a == nil {
		return nil
	}
	return &provider.CheckoutAddress{ContactName: a.ContactName, Address: a.Address, City: a.City, Country: a.Country, ZipCode: a.ZipCode}
}

type checkoutItemBody struct {
	ID               string `json:"id" validate:"required,max=64"`
	Name             string `json:"name" validate:"required,max=200"`
	Category         string `json:"category" validate:"required,max=100"`
	ItemType         string `json:"itemType" validate:"required,oneof=VIRTUAL PHYSICAL"`
	Price            string `json:"price" validate:"required"`
	SubMerchantKey   string `json:"subMerchantKey" validate:"omitempty,max=100"`
	SubMerchantPrice string `json:"subMerchantPrice"`
}

type startCheckoutBody struct {
	ConversationID  string               `json:"conversationId" validate:"omitempty,max=64"`
	BasketID        string               `json:"basketId" validate:"required,max=64"`
	Price           string               `json:"price" validate:"required"`
	Currency        string               `json:"currency" validate:"omitempty,oneof=TRY"`
	CallbackURL     string               `json:"callbackUrl" validate:"required,url,max=1000"`
	Locale          string               `json:"locale" validate:"omitempty,oneof=tr en"`
	Buyer           checkoutBuyerBody    `json:"buyer" validate:"required"`
	BillingAddress  *checkoutAddressBody `json:"billingAddress"`
	ShippingAddress *checkoutAddressBody `json:"shippingAddress"`
	Items           []checkoutItemBody   `json:"items" validate:"required,min=1,max=50,dive"`
}

func (b startCheckoutBody) toProvider() provider.HostedCheckoutRequest {
	req := provider.HostedCheckoutRequest{
		ConversationID: b.ConversationID, BasketID: b.BasketID, Price: b.Price,
		Currency: provider.CheckoutCurrencyTRY, CallbackURL: b.CallbackURL, Locale: b.Locale,
		Buyer: provider.CheckoutBuyer{
			ID: b.Buyer.ID, Name: b.Buyer.Name, Surname: b.Buyer.Surname, IdentityNumber: b.Buyer.IdentityNumber,
			Email: b.Buyer.Email, GSMNumber: b.Buyer.GSMNumber, Address: b.Buyer.Address, City: b.Buyer.City,
			Country: b.Buyer.Country, ZipCode: b.Buyer.ZipCode, IP: b.Buyer.IP,
		},
		ShippingAddress: b.ShippingAddress.toProvider(),
	}
	// Without a billing address the buyer is billed at the registration address.
	if billing := b.BillingAddress.toProvider(); billing != nil {
		req.BillingAddress = *billing
	} else {
		req.BillingAddress = provider.CheckoutAddress{ContactName: b.Buyer.Name + " " + b.Buyer.Surname,
			Address: b.Buyer.Address, City: b.Buyer.City, Country: b.Buyer.Country, ZipCode: b.Buyer.ZipCode}
	}
	for _, it := range b.Items {
		req.Items = append(req.Items, provider.CheckoutItem{ID: it.ID, Name: it.Name, Category: it.Category,
			ItemType: it.ItemType, Price: it.Price, SubMerchantKey: it.SubMerchantKey, SubMerchantPrice: it.SubMerchantPrice})
	}
	return req
}

type cancelCheckoutBody struct {
	PaymentID      string `json:"paymentId" validate:"required,max=64"`
	IP             string `json:"ip" validate:"required,ip"`
	ConversationID string `json:"conversationId" validate:"omitempty,max=64"`
	Reason         string `json:"reason" validate:"omitempty,oneof=OTHER FRAUD BUYER_REQUEST DOUBLE_PAYMENT"`
	Description    string `json:"description" validate:"omitempty,max=255"`
}

type refundCheckoutBody struct {
	PaymentTransactionID string `json:"paymentTransactionId" validate:"required,max=64"`
	Price                string `json:"price" validate:"required"`
	Currency             string `json:"currency" validate:"omitempty,oneof=TRY"`
	IP                   string `json:"ip" validate:"required,ip"`
	ConversationID       string `json:"conversationId" validate:"omitempty,max=64"`
	Reason               string `json:"reason" validate:"omitempty,oneof=OTHER FRAUD BUYER_REQUEST DOUBLE_PAYMENT"`
	Description          string `json:"description" validate:"omitempty,max=255"`
}

// bind decodes strictly, validates, and reads the environment; on failure it has answered 400.
func bind(w http.ResponseWriter, r *http.Request, v *validator.Validate, dst any, allowEmpty bool) (string, bool) {
	if err := decodeStrict(w, r, dst, allowEmpty); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request format", err)
		return "", false
	}
	if err := v.Struct(dst); err != nil {
		response.Error(w, http.StatusBadRequest, "Validation error", err)
		return "", false
	}
	environment, err := environmentFromRequest(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid environment", err)
		return "", false
	}
	return environment, true
}

// StartCheckout handles POST /v1/payments/{provider}/checkout.
func (h *CheckoutHandler) StartCheckout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var body startCheckoutBody
	environment, ok := bind(w, r, h.validate, &body, false)
	if !ok {
		return
	}
	providerName := chi.URLParam(r, "provider")
	session, err := h.service.StartCheckout(ctx, environment, providerName, body.toProvider())
	if err != nil {
		writeProviderError(w, providerName, "start checkout", err)
		return
	}
	response.Success(w, http.StatusOK, "Checkout started", session)
}

// CheckoutResult handles GET /v1/payments/{provider}/checkout/{token}. The envelope's success is
// true only for a paid outcome; data.outcome is the verdict.
func (h *CheckoutHandler) CheckoutResult(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	environment, err := environmentFromRequest(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid environment", err)
		return
	}
	conversationID := r.URL.Query().Get("conversationId")
	if len(conversationID) > 64 {
		response.Error(w, http.StatusBadRequest, "Validation error", nil)
		return
	}
	providerName := chi.URLParam(r, "provider")
	result, err := h.service.CheckoutResult(ctx, environment, providerName, provider.HostedCheckoutResultRequest{
		Token: chi.URLParam(r, "token"), ConversationID: conversationID,
	})
	if err != nil {
		writeProviderError(w, providerName, "checkout result", err)
		return
	}
	response.Return(w, http.StatusOK, result.Outcome == provider.OutcomePaid, "Checkout result", result)
}

// CancelPayment handles POST /v1/payments/{provider}/checkout/cancel.
func (h *CheckoutHandler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var body cancelCheckoutBody
	environment, ok := bind(w, r, h.validate, &body, false)
	if !ok {
		return
	}
	providerName := chi.URLParam(r, "provider")
	result, err := h.service.CancelPayment(ctx, environment, providerName, provider.HostedCancelRequest{
		PaymentID: body.PaymentID, IP: body.IP, ConversationID: body.ConversationID,
		Reason: body.Reason, Description: body.Description,
	})
	if err != nil {
		writeProviderError(w, providerName, "cancel checkout payment", err)
		return
	}
	response.Success(w, http.StatusOK, "Payment cancelled", result)
}

// RefundItem handles POST /v1/payments/{provider}/checkout/refund.
func (h *CheckoutHandler) RefundItem(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var body refundCheckoutBody
	environment, ok := bind(w, r, h.validate, &body, false)
	if !ok {
		return
	}
	providerName := chi.URLParam(r, "provider")
	result, err := h.service.RefundItem(ctx, environment, providerName, provider.HostedRefundRequest{
		PaymentTransactionID: body.PaymentTransactionID, Price: body.Price, Currency: provider.CheckoutCurrencyTRY,
		IP: body.IP, ConversationID: body.ConversationID, Reason: body.Reason, Description: body.Description,
	})
	if err != nil {
		writeProviderError(w, providerName, "refund checkout item", err)
		return
	}
	response.Success(w, http.StatusOK, "Item refunded", result)
}
