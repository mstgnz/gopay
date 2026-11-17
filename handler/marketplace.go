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

// MarketplaceServiceInterface is what the marketplace routes need from provider.MarketplaceService.
type MarketplaceServiceInterface interface {
	CreateSubMerchant(ctx context.Context, environment, providerName string, request provider.SubMerchantRequest) (*provider.SubMerchantResult, error)
	UpdateSubMerchant(ctx context.Context, environment, providerName string, request provider.SubMerchantRequest) (*provider.SubMerchantResult, error)
	SubMerchantDetail(ctx context.Context, environment, providerName string, request provider.SubMerchantDetailRequest) (*provider.SubMerchantDetail, error)
	ApproveItem(ctx context.Context, environment, providerName string, request provider.ItemApprovalRequest) (*provider.ItemApprovalResult, error)
	DisapproveItem(ctx context.Context, environment, providerName string, request provider.ItemApprovalRequest) (*provider.ItemApprovalResult, error)
}

// MarketplaceHandler serves sub-merchant and item-approval routes.
type MarketplaceHandler struct {
	service  MarketplaceServiceInterface
	validate *validator.Validate
}

func NewMarketplaceHandler(service MarketplaceServiceInterface, validate *validator.Validate) *MarketplaceHandler {
	return &MarketplaceHandler{service: service, validate: validate}
}

// subMerchantFields are the fields a create and an update share. Which identity fields a type
// needs is the provider's check.
type subMerchantFields struct {
	ConversationID    string `json:"conversationId" validate:"omitempty,max=64"`
	Name              string `json:"name" validate:"omitempty,max=200"`
	Email             string `json:"email" validate:"required,email,max=100"`
	GSMNumber         string `json:"gsmNumber" validate:"required,e164"`
	Address           string `json:"address" validate:"required,max=500"`
	IBAN              string `json:"iban" validate:"omitempty,alphanum,min=15,max=34"`
	Currency          string `json:"currency" validate:"omitempty,oneof=TRY"`
	ContactName       string `json:"contactName" validate:"omitempty,max=100"`
	ContactSurname    string `json:"contactSurname" validate:"omitempty,max=100"`
	IdentityNumber    string `json:"identityNumber" validate:"omitempty,min=5,max=20"`
	TaxOffice         string `json:"taxOffice" validate:"omitempty,max=100"`
	TaxNumber         string `json:"taxNumber" validate:"omitempty,max=20"`
	LegalCompanyTitle string `json:"legalCompanyTitle" validate:"omitempty,max=200"`
}

func (f subMerchantFields) toProvider() provider.SubMerchant {
	return provider.SubMerchant{Name: f.Name, Email: f.Email, GSMNumber: f.GSMNumber, Address: f.Address, IBAN: f.IBAN,
		Currency: f.Currency, ContactName: f.ContactName, ContactSurname: f.ContactSurname, IdentityNumber: f.IdentityNumber,
		TaxOffice: f.TaxOffice, TaxNumber: f.TaxNumber, LegalCompanyTitle: f.LegalCompanyTitle}
}

type createSubMerchantBody struct {
	subMerchantFields
	ExternalID string `json:"externalId" validate:"required,max=64"`
	Type       string `json:"type" validate:"required,oneof=PERSONAL PRIVATE_COMPANY LIMITED_OR_JOINT_STOCK_COMPANY"`
}

type itemApprovalBody struct {
	ConversationID string `json:"conversationId" validate:"omitempty,max=64"`
}

// CreateSubMerchant handles POST /v1/payments/{provider}/submerchants.
func (h *MarketplaceHandler) CreateSubMerchant(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var body createSubMerchantBody
	environment, ok := bind(w, r, h.validate, &body, false)
	if !ok {
		return
	}
	sm := body.toProvider()
	sm.ExternalID, sm.Type = body.ExternalID, body.Type

	providerName := chi.URLParam(r, "provider")
	result, err := h.service.CreateSubMerchant(ctx, environment, providerName, provider.SubMerchantRequest{
		ConversationID: body.ConversationID, SubMerchant: sm,
	})
	if err != nil {
		writeProviderError(w, providerName, "create sub-merchant", err)
		return
	}
	response.Success(w, http.StatusOK, "Sub-merchant created", result)
}

// UpdateSubMerchant handles PUT /v1/payments/{provider}/submerchants/{key}.
func (h *MarketplaceHandler) UpdateSubMerchant(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var body subMerchantFields
	environment, ok := bind(w, r, h.validate, &body, false)
	if !ok {
		return
	}
	providerName := chi.URLParam(r, "provider")
	result, err := h.service.UpdateSubMerchant(ctx, environment, providerName, provider.SubMerchantRequest{
		ConversationID: body.ConversationID, Key: chi.URLParam(r, "key"), SubMerchant: body.toProvider(),
	})
	if err != nil {
		writeProviderError(w, providerName, "update sub-merchant", err)
		return
	}
	response.Success(w, http.StatusOK, "Sub-merchant updated", result)
}

// SubMerchantDetail handles GET /v1/payments/{provider}/submerchants/{externalId}.
func (h *MarketplaceHandler) SubMerchantDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	environment, err := environmentFromRequest(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid environment", err)
		return
	}
	providerName := chi.URLParam(r, "provider")
	result, err := h.service.SubMerchantDetail(ctx, environment, providerName, provider.SubMerchantDetailRequest{
		ExternalID: chi.URLParam(r, "externalId"),
	})
	if err != nil {
		writeProviderError(w, providerName, "sub-merchant detail", err)
		return
	}
	response.Success(w, http.StatusOK, "Sub-merchant", result)
}

// ApproveItem handles POST /v1/payments/{provider}/items/{paymentTransactionId}/approve.
func (h *MarketplaceHandler) ApproveItem(w http.ResponseWriter, r *http.Request) {
	h.itemApproval(w, r, "approve item", "Item approved", h.service.ApproveItem)
}

// DisapproveItem handles POST /v1/payments/{provider}/items/{paymentTransactionId}/disapprove.
func (h *MarketplaceHandler) DisapproveItem(w http.ResponseWriter, r *http.Request) {
	h.itemApproval(w, r, "disapprove item", "Item disapproved", h.service.DisapproveItem)
}

func (h *MarketplaceHandler) itemApproval(w http.ResponseWriter, r *http.Request, operation, message string,
	call func(context.Context, string, string, provider.ItemApprovalRequest) (*provider.ItemApprovalResult, error)) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var body itemApprovalBody
	environment, ok := bind(w, r, h.validate, &body, true)
	if !ok {
		return
	}
	providerName := chi.URLParam(r, "provider")
	result, err := call(ctx, environment, providerName, provider.ItemApprovalRequest{
		PaymentTransactionID: chi.URLParam(r, "paymentTransactionId"), ConversationID: body.ConversationID,
	})
	if err != nil {
		writeProviderError(w, providerName, operation, err)
		return
	}
	response.Success(w, http.StatusOK, message, result)
}
