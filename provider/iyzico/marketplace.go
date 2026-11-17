package iyzico

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/mstgnz/gopay/provider"
)

// Marketplace (docs.iyzico.com: "Marketplace / Submerchant", "Marketplace / Approval").
const (
	pathSubMerchant       = "/onboarding/submerchant"
	pathSubMerchantDetail = "/onboarding/submerchant/detail"
	pathItemApprove       = "/payment/iyzipos/item/approve"
	pathItemDisapprove    = "/payment/iyzipos/item/disapprove"
)

type subMerchantBody struct {
	Locale            string `json:"locale"`
	ConversationID    string `json:"conversationId"`
	SubMerchantKey    string `json:"subMerchantKey,omitempty"`
	ExternalID        string `json:"subMerchantExternalId,omitempty"`
	Type              string `json:"subMerchantType,omitempty"`
	Name              string `json:"name,omitempty"`
	Email             string `json:"email"`
	GSMNumber         string `json:"gsmNumber"`
	Address           string `json:"address"`
	IBAN              string `json:"iban,omitempty"`
	Currency          string `json:"currency"`
	ContactName       string `json:"contactName,omitempty"`
	ContactSurname    string `json:"contactSurname,omitempty"`
	IdentityNumber    string `json:"identityNumber,omitempty"`
	TaxOffice         string `json:"taxOffice,omitempty"`
	TaxNumber         string `json:"taxNumber,omitempty"`
	LegalCompanyTitle string `json:"legalCompanyTitle,omitempty"`
}

func bodyFor(r provider.SubMerchantRequest) subMerchantBody {
	sm := r.SubMerchant
	return subMerchantBody{
		Locale: defaultLocale, ConversationID: conversationID(r.ConversationID),
		Name: sm.Name, Email: sm.Email, GSMNumber: sm.GSMNumber, Address: sm.Address, IBAN: sm.IBAN,
		Currency: orDefault(sm.Currency, provider.CheckoutCurrencyTRY), ContactName: sm.ContactName,
		ContactSurname: sm.ContactSurname, IdentityNumber: sm.IdentityNumber, TaxOffice: sm.TaxOffice,
		TaxNumber: sm.TaxNumber, LegalCompanyTitle: sm.LegalCompanyTitle,
	}
}

// requiredOnCreate is iyzico's per-type list (OpenAPI "required"), checked before the call so the
// tenant gets a field name instead of an iyzico code.
var requiredOnCreate = map[string][]string{
	provider.SubMerchantPersonal:                   {"email", "gsmNumber", "address", "contactName", "contactSurname", "identityNumber"},
	provider.SubMerchantPrivateCompany:             {"email", "gsmNumber", "address", "taxOffice", "legalCompanyTitle"},
	provider.SubMerchantLimitedOrJointStockCompany: {"email", "gsmNumber", "address", "taxOffice", "taxNumber", "legalCompanyTitle"},
}

// requiredOnUpdate: the type is not part of an update, so only the fields every type needs.
var requiredOnUpdate = []string{"email", "gsmNumber", "address", "iban"}

func fieldValue(sm provider.SubMerchant, name string) string {
	switch name {
	case "email":
		return sm.Email
	case "gsmNumber":
		return sm.GSMNumber
	case "address":
		return sm.Address
	case "iban":
		return sm.IBAN
	case "contactName":
		return sm.ContactName
	case "contactSurname":
		return sm.ContactSurname
	case "identityNumber":
		return sm.IdentityNumber
	case "taxOffice":
		return sm.TaxOffice
	case "taxNumber":
		return sm.TaxNumber
	case "legalCompanyTitle":
		return sm.LegalCompanyTitle
	}
	return ""
}

func requireFields(sm provider.SubMerchant, names []string) error {
	var missing []string
	for _, n := range names {
		if strings.TrimSpace(fieldValue(sm, n)) == "" {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing %s", provider.ErrInvalidRequest, strings.Join(missing, ", "))
	}
	return nil
}

type subMerchantKeyAnswer struct {
	SubMerchantKey string `json:"subMerchantKey"`
	ConversationID string `json:"conversationId"`
}

// CreateSubMerchant registers a seller. A success without a key is unavailable, not a refusal:
// the seller may exist now, and SubMerchantDetail by external id finds it.
func (p *IyzicoProvider) CreateSubMerchant(ctx context.Context, r provider.SubMerchantRequest) (*provider.SubMerchantResult, error) {
	required, ok := requiredOnCreate[r.SubMerchant.Type]
	if !ok {
		return nil, fmt.Errorf("%w: type must be %s, %s or %s", provider.ErrInvalidRequest,
			provider.SubMerchantPersonal, provider.SubMerchantPrivateCompany, provider.SubMerchantLimitedOrJointStockCompany)
	}
	if err := requireFields(r.SubMerchant, required); err != nil {
		return nil, err
	}

	b := bodyFor(r)
	b.ExternalID, b.Type = r.SubMerchant.ExternalID, r.SubMerchant.Type
	var a subMerchantKeyAnswer
	if err := p.client.do(ctx, http.MethodPost, pathSubMerchant, b, &a); err != nil {
		return nil, err
	}
	if a.SubMerchantKey == "" {
		return nil, unavailable("create answered success without a sub-merchant key")
	}
	return &provider.SubMerchantResult{SubMerchantKey: a.SubMerchantKey, ConversationID: a.ConversationID}, nil
}

// UpdateSubMerchant changes a registered seller by its key; iyzico does not let the type change.
func (p *IyzicoProvider) UpdateSubMerchant(ctx context.Context, r provider.SubMerchantRequest) (*provider.SubMerchantResult, error) {
	if err := requireFields(r.SubMerchant, requiredOnUpdate); err != nil {
		return nil, err
	}

	b := bodyFor(r)
	b.SubMerchantKey = r.Key
	var a subMerchantKeyAnswer
	if err := p.client.do(ctx, http.MethodPut, pathSubMerchant, b, &a); err != nil {
		return nil, err
	}
	return &provider.SubMerchantResult{SubMerchantKey: r.Key, ConversationID: a.ConversationID}, nil
}

type subMerchantDetailAnswer struct {
	SubMerchantKey    string `json:"subMerchantKey"`
	ExternalID        string `json:"subMerchantExternalId"`
	Type              string `json:"subMerchantType"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	GSMNumber         string `json:"gsmNumber"`
	Address           string `json:"address"`
	IBAN              string `json:"iban"`
	Currency          string `json:"currency"`
	TaxOffice         string `json:"taxOffice"`
	LegalCompanyTitle string `json:"legalCompanyTitle"`
}

// SubMerchantDetail finds a seller by the tenant's external id, for when a create succeeded at
// iyzico but the tenant never stored the key.
func (p *IyzicoProvider) SubMerchantDetail(ctx context.Context, r provider.SubMerchantDetailRequest) (*provider.SubMerchantDetail, error) {
	body := map[string]string{"locale": defaultLocale, "conversationId": conversationID(r.ConversationID),
		"subMerchantExternalId": r.ExternalID}

	var a subMerchantDetailAnswer
	if err := p.client.do(ctx, http.MethodPost, pathSubMerchantDetail, body, &a); err != nil {
		return nil, err
	}
	if a.SubMerchantKey == "" {
		return nil, unavailable("detail answered success without a sub-merchant key")
	}
	return &provider.SubMerchantDetail{
		SubMerchantKey: a.SubMerchantKey, ExternalID: orDefault(a.ExternalID, r.ExternalID), Type: a.Type,
		Name: a.Name, Email: a.Email, GSMNumber: a.GSMNumber, Address: a.Address, IBAN: a.IBAN,
		Currency: a.Currency, TaxOffice: a.TaxOffice, LegalCompanyTitle: a.LegalCompanyTitle,
	}, nil
}

type itemApprovalAnswer struct {
	PaymentTransactionID flexString `json:"paymentTransactionId"`
	ConversationID       string     `json:"conversationId"`
}

// ApproveItem releases a paid item's sub-merchant share. There is no panel action for it.
func (p *IyzicoProvider) ApproveItem(ctx context.Context, r provider.ItemApprovalRequest) (*provider.ItemApprovalResult, error) {
	return p.itemApproval(ctx, pathItemApprove, r)
}

// DisapproveItem withdraws an approval given earlier.
func (p *IyzicoProvider) DisapproveItem(ctx context.Context, r provider.ItemApprovalRequest) (*provider.ItemApprovalResult, error) {
	return p.itemApproval(ctx, pathItemDisapprove, r)
}

func (p *IyzicoProvider) itemApproval(ctx context.Context, path string, r provider.ItemApprovalRequest) (*provider.ItemApprovalResult, error) {
	body := map[string]string{"locale": defaultLocale, "conversationId": conversationID(r.ConversationID),
		"paymentTransactionId": r.PaymentTransactionID}

	var a itemApprovalAnswer
	if err := p.client.do(ctx, http.MethodPost, path, body, &a); err != nil {
		return nil, err
	}
	return &provider.ItemApprovalResult{
		PaymentTransactionID: orDefault(string(a.PaymentTransactionID), r.PaymentTransactionID),
		ConversationID:       a.ConversationID,
	}, nil
}
