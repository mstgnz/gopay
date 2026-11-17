package provider

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// Hosted checkout and marketplace are OPTIONAL capabilities, like CardStorageProvider: the core
// PaymentProvider interface is not extended, and a provider without them answers
// ErrCheckoutUnsupported / ErrMarketplaceUnsupported. Every amount on these paths is an exact
// decimal string; nothing here passes through float64.

var (
	ErrCheckoutUnsupported    = errors.New("provider does not support hosted checkout")
	ErrMarketplaceUnsupported = errors.New("provider does not support marketplace operations")

	// ErrInvalidRequest marks a request refused before any provider call.
	ErrInvalidRequest = errors.New("invalid request")

	// ErrProviderUnavailable: the provider gave no verdict (transport failure, 5xx, rate limit,
	// unreadable answer). The operation may or may not have taken effect.
	ErrProviderUnavailable = errors.New("payment provider unavailable")
)

// ProviderRefusal is the provider's own answer that it will not do what was asked. Retrying the
// same call unchanged will not help unless Retryable says so.
type ProviderRefusal struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *ProviderRefusal) Error() string {
	return fmt.Sprintf("provider refused: %s %s", e.Code, e.Message)
}

// HostedCheckoutProvider starts a payment on the provider's own page, so card data never passes
// through GoPay, and reads its verdict server-side.
type HostedCheckoutProvider interface {
	StartHostedCheckout(ctx context.Context, request HostedCheckoutRequest) (*HostedCheckoutSession, error)
	HostedCheckoutResult(ctx context.Context, request HostedCheckoutResultRequest) (*HostedCheckoutResult, error)
	// CancelHostedPayment voids a whole payment; banks accept it on the payment's day only.
	CancelHostedPayment(ctx context.Context, request HostedCancelRequest) (*HostedCancelResult, error)
	// RefundHostedItem returns part or all of one paid basket item.
	RefundHostedItem(ctx context.Context, request HostedRefundRequest) (*HostedRefundResult, error)
}

// MarketplaceProvider registers sub-merchants and releases their share of paid items.
type MarketplaceProvider interface {
	CreateSubMerchant(ctx context.Context, request SubMerchantRequest) (*SubMerchantResult, error)
	UpdateSubMerchant(ctx context.Context, request SubMerchantRequest) (*SubMerchantResult, error)
	SubMerchantDetail(ctx context.Context, request SubMerchantDetailRequest) (*SubMerchantDetail, error)
	ApproveItem(ctx context.Context, request ItemApprovalRequest) (*ItemApprovalResult, error)
	DisapproveItem(ctx context.Context, request ItemApprovalRequest) (*ItemApprovalResult, error)
}

// Item types a hosted checkout accepts.
const (
	ItemTypeVirtual  = "VIRTUAL"
	ItemTypePhysical = "PHYSICAL"
)

// CheckoutCurrencyTRY is the only currency the hosted checkout accepts today.
const CheckoutCurrencyTRY = "TRY"

type CheckoutBuyer struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Surname        string `json:"surname"`
	IdentityNumber string `json:"identityNumber"`
	Email          string `json:"email"`
	GSMNumber      string `json:"gsmNumber"`
	Address        string `json:"address"`
	City           string `json:"city"`
	Country        string `json:"country"`
	ZipCode        string `json:"zipCode,omitempty"`
	IP             string `json:"ip"`
}

type CheckoutAddress struct {
	ContactName string `json:"contactName"`
	Address     string `json:"address"`
	City        string `json:"city"`
	Country     string `json:"country"`
	ZipCode     string `json:"zipCode,omitempty"`
}

// CheckoutItem is one basket line. SubMerchantKey and SubMerchantPrice come together or not at all:
// with them the line is a marketplace sale and SubMerchantPrice is the sub-merchant's share.
type CheckoutItem struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Category         string `json:"category"`
	ItemType         string `json:"itemType"`
	Price            string `json:"price"`
	SubMerchantKey   string `json:"subMerchantKey,omitempty"`
	SubMerchantPrice string `json:"subMerchantPrice,omitempty"`
}

type HostedCheckoutRequest struct {
	LogID           int64            `json:"logId,omitempty"`
	ConversationID  string           `json:"conversationId,omitempty"`
	BasketID        string           `json:"basketId"`
	Price           string           `json:"price"`
	Currency        string           `json:"currency"`
	CallbackURL     string           `json:"callbackUrl"`
	Locale          string           `json:"locale,omitempty"`
	Buyer           CheckoutBuyer    `json:"buyer"`
	BillingAddress  CheckoutAddress  `json:"billingAddress"`
	ShippingAddress *CheckoutAddress `json:"shippingAddress,omitempty"`
	Items           []CheckoutItem   `json:"items"`
}

// HostedCheckoutSession is a started payment: the customer goes to PaymentPageURL, and Token reads
// the result. The provider POSTs only the token to CallbackURL; that POST is not a verdict.
type HostedCheckoutSession struct {
	Token              string `json:"token"`
	PaymentPageURL     string `json:"paymentPageUrl"`
	TokenExpireSeconds int    `json:"tokenExpireSeconds,omitempty"`
	ConversationID     string `json:"conversationId"`
}

type HostedCheckoutResultRequest struct {
	LogID          int64  `json:"logId,omitempty"`
	Token          string `json:"token"`
	ConversationID string `json:"conversationId,omitempty"`
}

// PaymentOutcome is the verdict on a hosted payment.
type PaymentOutcome string

const (
	OutcomePaid     PaymentOutcome = "paid"
	OutcomeInReview PaymentOutcome = "in_review"
	OutcomeDeclined PaymentOutcome = "declined"
	// OutcomeNoVerdict: an error answer, a page still open, or an answer GoPay cannot trust. The
	// payment must stay open and be read again later.
	OutcomeNoVerdict PaymentOutcome = "no_verdict"
)

type CheckoutItemResult struct {
	ItemID               string `json:"itemId"`
	PaymentTransactionID string `json:"paymentTransactionId"`
	TransactionStatus    *int   `json:"transactionStatus,omitempty"`
	Price                string `json:"price"`
	PaidPrice            string `json:"paidPrice"`
	SubMerchantKey       string `json:"subMerchantKey,omitempty"`
	SubMerchantPrice     string `json:"subMerchantPrice,omitempty"`
}

type HostedCheckoutResult struct {
	Outcome PaymentOutcome `json:"outcome"`
	// Readable: the provider answered with a payment status, not an error answer.
	Readable       bool                 `json:"readable"`
	PaymentStatus  string               `json:"paymentStatus,omitempty"`
	FraudStatus    *int                 `json:"fraudStatus,omitempty"`
	PaymentID      string               `json:"paymentId,omitempty"`
	ConversationID string               `json:"conversationId,omitempty"`
	BasketID       string               `json:"basketId,omitempty"`
	Price          string               `json:"price,omitempty"`
	PaidPrice      string               `json:"paidPrice,omitempty"`
	Currency       string               `json:"currency,omitempty"`
	ErrorCode      string               `json:"errorCode,omitempty"`
	ErrorMessage   string               `json:"errorMessage,omitempty"`
	Items          []CheckoutItemResult `json:"itemTransactions"`
}

// Cancel and refund reasons, as iyzico names them.
var refundReasons = map[string]bool{"OTHER": true, "FRAUD": true, "BUYER_REQUEST": true, "DOUBLE_PAYMENT": true}

type HostedCancelRequest struct {
	LogID          int64  `json:"logId,omitempty"`
	PaymentID      string `json:"paymentId"`
	IP             string `json:"ip"`
	ConversationID string `json:"conversationId,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Description    string `json:"description,omitempty"`
}

type HostedCancelResult struct {
	PaymentID      string `json:"paymentId"`
	Price          string `json:"price,omitempty"`
	Currency       string `json:"currency,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
}

type HostedRefundRequest struct {
	LogID                int64  `json:"logId,omitempty"`
	PaymentTransactionID string `json:"paymentTransactionId"`
	Price                string `json:"price"`
	Currency             string `json:"currency"`
	IP                   string `json:"ip"`
	ConversationID       string `json:"conversationId,omitempty"`
	Reason               string `json:"reason,omitempty"`
	Description          string `json:"description,omitempty"`
}

type HostedRefundResult struct {
	PaymentID            string `json:"paymentId,omitempty"`
	PaymentTransactionID string `json:"paymentTransactionId"`
	Price                string `json:"price"`
	Currency             string `json:"currency,omitempty"`
	ConversationID       string `json:"conversationId,omitempty"`
}

// Sub-merchant types, as iyzico names them.
const (
	SubMerchantPersonal                   = "PERSONAL"
	SubMerchantPrivateCompany             = "PRIVATE_COMPANY"
	SubMerchantLimitedOrJointStockCompany = "LIMITED_OR_JOINT_STOCK_COMPANY"
)

// SubMerchant is a seller as the marketplace registers it. Which identity fields are required
// depends on Type; the provider checks that.
type SubMerchant struct {
	ExternalID        string `json:"externalId,omitempty"`
	Type              string `json:"type,omitempty"`
	Name              string `json:"name,omitempty"`
	Email             string `json:"email"`
	GSMNumber         string `json:"gsmNumber"`
	Address           string `json:"address"`
	IBAN              string `json:"iban,omitempty"`
	Currency          string `json:"currency,omitempty"`
	ContactName       string `json:"contactName,omitempty"`
	ContactSurname    string `json:"contactSurname,omitempty"`
	IdentityNumber    string `json:"identityNumber,omitempty"`
	TaxOffice         string `json:"taxOffice,omitempty"`
	TaxNumber         string `json:"taxNumber,omitempty"`
	LegalCompanyTitle string `json:"legalCompanyTitle,omitempty"`
}

// SubMerchantRequest creates (Key empty) or updates (Key set; ExternalID and Type are ignored,
// the marketplace does not let them change) a sub-merchant.
type SubMerchantRequest struct {
	LogID          int64       `json:"logId,omitempty"`
	ConversationID string      `json:"conversationId,omitempty"`
	Key            string      `json:"subMerchantKey,omitempty"`
	SubMerchant    SubMerchant `json:"subMerchant"`
}

type SubMerchantResult struct {
	SubMerchantKey string `json:"subMerchantKey,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
}

type SubMerchantDetailRequest struct {
	LogID          int64  `json:"logId,omitempty"`
	ExternalID     string `json:"externalId"`
	ConversationID string `json:"conversationId,omitempty"`
}

// SubMerchantDetail is what the marketplace holds under an external id. Identity and tax numbers
// are deliberately not returned.
type SubMerchantDetail struct {
	SubMerchantKey    string `json:"subMerchantKey"`
	ExternalID        string `json:"externalId"`
	Type              string `json:"type,omitempty"`
	Name              string `json:"name,omitempty"`
	Email             string `json:"email,omitempty"`
	GSMNumber         string `json:"gsmNumber,omitempty"`
	Address           string `json:"address,omitempty"`
	IBAN              string `json:"iban,omitempty"`
	Currency          string `json:"currency,omitempty"`
	TaxOffice         string `json:"taxOffice,omitempty"`
	LegalCompanyTitle string `json:"legalCompanyTitle,omitempty"`
}

type ItemApprovalRequest struct {
	LogID                int64  `json:"logId,omitempty"`
	PaymentTransactionID string `json:"paymentTransactionId"`
	ConversationID       string `json:"conversationId,omitempty"`
}

type ItemApprovalResult struct {
	PaymentTransactionID string `json:"paymentTransactionId"`
	ConversationID       string `json:"conversationId,omitempty"`
}

// amountText: up to 12 integer digits and 2 fraction digits (kuruş), no sign, no exponent.
var amountText = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,2})?$`)

// ParseAmount reads a decimal amount string exactly.
func ParseAmount(s string) (*big.Rat, error) {
	if !amountText.MatchString(s) {
		return nil, fmt.Errorf("%w: %q is not a decimal amount with at most 2 fraction digits", ErrInvalidRequest, s)
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a decimal amount", ErrInvalidRequest, s)
	}
	return r, nil
}

func positiveAmount(field, s string) (*big.Rat, error) {
	r, err := ParseAmount(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	if r.Sign() <= 0 {
		return nil, fmt.Errorf("%w: %s must be greater than 0", ErrInvalidRequest, field)
	}
	return r, nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRequest, fmt.Sprintf(format, args...))
}

// ValidateHostedCheckout checks the money and shape rules every provider shares, before any call:
// price equals the sum of the item prices, a marketplace share is positive and at most its item,
// a PHYSICAL item needs a shipping address, and the callback URL is https (http only in sandbox).
func ValidateHostedCheckout(request HostedCheckoutRequest, environment string) error {
	if request.Currency != CheckoutCurrencyTRY {
		return invalid("currency must be %s", CheckoutCurrencyTRY)
	}
	if err := validateCallbackURL(request.CallbackURL, environment); err != nil {
		return err
	}
	if net.ParseIP(request.Buyer.IP) == nil {
		return invalid("buyer.ip must be an IP address")
	}
	if len(request.Items) == 0 {
		return invalid("items must not be empty")
	}
	total, err := positiveAmount("price", request.Price)
	if err != nil {
		return err
	}

	sum := new(big.Rat)
	physical := false
	for i, item := range request.Items {
		price, err := positiveAmount(fmt.Sprintf("items[%d].price", i), item.Price)
		if err != nil {
			return err
		}
		sum.Add(sum, price)

		switch item.ItemType {
		case ItemTypeVirtual:
		case ItemTypePhysical:
			physical = true
		default:
			return invalid("items[%d].itemType must be %s or %s", i, ItemTypeVirtual, ItemTypePhysical)
		}

		if (item.SubMerchantKey == "") != (item.SubMerchantPrice == "") {
			return invalid("items[%d]: subMerchantKey and subMerchantPrice come together", i)
		}
		if item.SubMerchantPrice != "" {
			share, err := positiveAmount(fmt.Sprintf("items[%d].subMerchantPrice", i), item.SubMerchantPrice)
			if err != nil {
				return err
			}
			if share.Cmp(price) > 0 {
				return invalid("items[%d].subMerchantPrice must not exceed the item price", i)
			}
		}
	}
	if sum.Cmp(total) != 0 {
		return invalid("price %s must equal the sum of the item prices %s", request.Price, sum.FloatString(2))
	}
	if physical && request.ShippingAddress == nil {
		return invalid("shippingAddress is required when an item is %s", ItemTypePhysical)
	}
	return nil
}

func validateCallbackURL(raw, environment string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return invalid("callbackUrl must be an absolute URL")
	}
	switch {
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && environment == "sandbox":
		return nil
	default:
		return invalid("callbackUrl must use https")
	}
}

// ValidateHostedCancel checks a cancel before any call.
func ValidateHostedCancel(request HostedCancelRequest) error {
	if strings.TrimSpace(request.PaymentID) == "" {
		return invalid("paymentId is required")
	}
	if net.ParseIP(request.IP) == nil {
		return invalid("ip must be an IP address")
	}
	return validateReason(request.Reason, request.Description)
}

// ValidateHostedRefund checks a refund before any call.
func ValidateHostedRefund(request HostedRefundRequest) error {
	if strings.TrimSpace(request.PaymentTransactionID) == "" {
		return invalid("paymentTransactionId is required")
	}
	if request.Currency != CheckoutCurrencyTRY {
		return invalid("currency must be %s", CheckoutCurrencyTRY)
	}
	if _, err := positiveAmount("price", request.Price); err != nil {
		return err
	}
	if net.ParseIP(request.IP) == nil {
		return invalid("ip must be an IP address")
	}
	return validateReason(request.Reason, request.Description)
}

// iyzico requires a reason whenever a description is sent.
func validateReason(reason, description string) error {
	if reason == "" {
		if description != "" {
			return invalid("reason is required when description is sent")
		}
		return nil
	}
	if !refundReasons[reason] {
		return invalid("reason must be one of OTHER, FRAUD, BUYER_REQUEST, DOUBLE_PAYMENT")
	}
	return nil
}
