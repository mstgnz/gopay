package iyzico

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/mstgnz/gopay/provider"
)

// Checkout Form, cancel and refund (docs.iyzico.com: "CF-Initialize", "CF-Retrieve",
// "Refund & Cancel", "Response Signature Validation").
const (
	pathCFInitialize = "/payment/iyzipos/checkoutform/initialize/auth/ecom"
	pathCFRetrieve   = "/payment/iyzipos/checkoutform/auth/ecom/detail"
	pathCancel       = "/payment/cancel"
	pathRefund       = "/payment/refund"

	paymentGroupProduct = "PRODUCT"
	defaultLocale       = "tr"
	defaultCountry      = "Turkey"

	paymentStatusSuccess = "SUCCESS"
	paymentStatusFailure = "FAILURE"

	// GoPay's own error codes, prefixed so they never read as iyzico's.
	codeSignatureMismatch = "GOPAY_SIGNATURE_MISMATCH"
	codeUnreadableResult  = "GOPAY_UNREADABLE_RESULT"
)

type buyerBody struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Surname             string `json:"surname"`
	IdentityNumber      string `json:"identityNumber"`
	Email               string `json:"email"`
	GSMNumber           string `json:"gsmNumber"`
	RegistrationAddress string `json:"registrationAddress"`
	City                string `json:"city"`
	Country             string `json:"country"`
	ZipCode             string `json:"zipCode,omitempty"`
	IP                  string `json:"ip"`
}

type addressBody struct {
	ContactName string `json:"contactName"`
	Address     string `json:"address"`
	City        string `json:"city"`
	Country     string `json:"country"`
	ZipCode     string `json:"zipCode,omitempty"`
}

type basketItemBody struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Category1        string `json:"category1"`
	ItemType         string `json:"itemType"`
	Price            string `json:"price"`
	SubMerchantKey   string `json:"subMerchantKey,omitempty"`
	SubMerchantPrice string `json:"subMerchantPrice,omitempty"`
}

type checkoutInitBody struct {
	Locale              string           `json:"locale"`
	ConversationID      string           `json:"conversationId"`
	Price               string           `json:"price"`
	PaidPrice           string           `json:"paidPrice"`
	Currency            string           `json:"currency"`
	BasketID            string           `json:"basketId"`
	PaymentGroup        string           `json:"paymentGroup"`
	CallbackURL         string           `json:"callbackUrl"`
	EnabledInstallments []int            `json:"enabledInstallments"`
	Buyer               buyerBody        `json:"buyer"`
	ShippingAddress     *addressBody     `json:"shippingAddress,omitempty"`
	BillingAddress      addressBody      `json:"billingAddress"`
	BasketItems         []basketItemBody `json:"basketItems"`
}

type checkoutInitAnswer struct {
	ConversationID  string  `json:"conversationId"`
	Token           string  `json:"token"`
	PaymentPageURL  string  `json:"paymentPageUrl"`
	TokenExpireTime flexInt `json:"tokenExpireTime"`
	Signature       string  `json:"signature"`
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func conversationID(given string) string {
	return orDefault(given, uuid.NewString())
}

func toAddress(a provider.CheckoutAddress) addressBody {
	return addressBody{ContactName: a.ContactName, Address: a.Address, City: a.City,
		Country: orDefault(a.Country, defaultCountry), ZipCode: a.ZipCode}
}

// StartHostedCheckout opens an iyzico Checkout Form. One installment only; the amount is charged
// as sent (paidPrice = price). The caller has run provider.ValidateHostedCheckout.
func (p *IyzicoProvider) StartHostedCheckout(ctx context.Context, r provider.HostedCheckoutRequest) (*provider.HostedCheckoutSession, error) {
	b := checkoutInitBody{
		Locale: orDefault(r.Locale, defaultLocale), ConversationID: conversationID(r.ConversationID),
		Price: r.Price, PaidPrice: r.Price, Currency: r.Currency, BasketID: r.BasketID,
		PaymentGroup: paymentGroupProduct, CallbackURL: r.CallbackURL, EnabledInstallments: []int{1},
		Buyer: buyerBody{
			ID: r.Buyer.ID, Name: r.Buyer.Name, Surname: r.Buyer.Surname, IdentityNumber: r.Buyer.IdentityNumber,
			Email: r.Buyer.Email, GSMNumber: r.Buyer.GSMNumber, RegistrationAddress: r.Buyer.Address,
			City: r.Buyer.City, Country: orDefault(r.Buyer.Country, defaultCountry), ZipCode: r.Buyer.ZipCode, IP: r.Buyer.IP,
		},
		BillingAddress: toAddress(r.BillingAddress),
	}
	// iyzico wants no shipping address when every item is VIRTUAL.
	if r.ShippingAddress != nil {
		shipping := toAddress(*r.ShippingAddress)
		b.ShippingAddress = &shipping
	}
	for _, it := range r.Items {
		b.BasketItems = append(b.BasketItems, basketItemBody{
			ID: it.ID, Name: it.Name, Category1: it.Category, ItemType: it.ItemType, Price: it.Price,
			SubMerchantKey: it.SubMerchantKey, SubMerchantPrice: it.SubMerchantPrice,
		})
	}

	var a checkoutInitAnswer
	if err := p.client.do(ctx, http.MethodPost, pathCFInitialize, b, &a); err != nil {
		return nil, err
	}
	if a.Token == "" || a.PaymentPageURL == "" {
		return nil, unavailable("initialize answered success without a token or a page")
	}
	// A tampered answer could send the customer to someone else's page. iyzico documents the
	// signature on this answer; when it is sent it must match.
	if a.Signature != "" && !p.client.signatureMatches(a.Signature, a.ConversationID, a.Token) {
		return nil, unavailable("initialize answer signature does not match")
	}
	return &provider.HostedCheckoutSession{
		Token: a.Token, PaymentPageURL: a.PaymentPageURL,
		TokenExpireSeconds: a.TokenExpireTime.value, ConversationID: b.ConversationID,
	}, nil
}

type itemTransactionAnswer struct {
	ItemID               string     `json:"itemId"`
	PaymentTransactionID flexString `json:"paymentTransactionId"`
	TransactionStatus    flexInt    `json:"transactionStatus"`
	Price                amount     `json:"price"`
	PaidPrice            amount     `json:"paidPrice"`
	SubMerchantKey       string     `json:"subMerchantKey"`
	SubMerchantPrice     amount     `json:"subMerchantPrice"`
}

type retrieveAnswer struct {
	PaymentStatus    string                  `json:"paymentStatus"`
	PaymentID        flexString              `json:"paymentId"`
	ConversationID   string                  `json:"conversationId"`
	BasketID         string                  `json:"basketId"`
	Currency         string                  `json:"currency"`
	Price            amount                  `json:"price"`
	PaidPrice        amount                  `json:"paidPrice"`
	FraudStatus      flexInt                 `json:"fraudStatus"`
	Token            string                  `json:"token"`
	Signature        string                  `json:"signature"`
	ErrorCode        flexString              `json:"errorCode"`
	ErrorMessage     string                  `json:"errorMessage"`
	ItemTransactions []itemTransactionAnswer `json:"itemTransactions"`
}

func (a retrieveAnswer) result() *provider.HostedCheckoutResult {
	r := &provider.HostedCheckoutResult{
		Outcome: provider.OutcomeNoVerdict, Readable: a.PaymentStatus != "", PaymentStatus: a.PaymentStatus,
		FraudStatus: a.FraudStatus.ptr(), PaymentID: string(a.PaymentID), ConversationID: a.ConversationID,
		BasketID: a.BasketID, Price: string(a.Price), PaidPrice: string(a.PaidPrice), Currency: a.Currency,
		ErrorCode: string(a.ErrorCode), ErrorMessage: a.ErrorMessage, Items: []provider.CheckoutItemResult{},
	}
	for _, it := range a.ItemTransactions {
		r.Items = append(r.Items, provider.CheckoutItemResult{
			ItemID: it.ItemID, PaymentTransactionID: string(it.PaymentTransactionID),
			TransactionStatus: it.TransactionStatus.ptr(), Price: string(it.Price), PaidPrice: string(it.PaidPrice),
			SubMerchantKey: it.SubMerchantKey, SubMerchantPrice: string(it.SubMerchantPrice),
		})
	}
	return r
}

// outcome maps iyzico's record to a verdict. Only FAILURE or a fraud rejection is a decline; a
// status in between (3-D Secure still running) or a success GoPay cannot read is no verdict.
func (a retrieveAnswer) outcome() (provider.PaymentOutcome, string) {
	switch {
	case a.PaymentStatus == paymentStatusFailure:
		return provider.OutcomeDeclined, ""
	case a.PaymentStatus != paymentStatusSuccess:
		return provider.OutcomeNoVerdict, ""
	case !a.FraudStatus.set:
		return provider.OutcomeNoVerdict, codeUnreadableResult
	case a.FraudStatus.value == -1:
		return provider.OutcomeDeclined, ""
	case a.FraudStatus.value == 0:
		return provider.OutcomeInReview, ""
	case a.FraudStatus.value == 1 && a.PaymentID != "" && itemsReadable(a.ItemTransactions):
		return provider.OutcomePaid, ""
	default:
		return provider.OutcomeNoVerdict, codeUnreadableResult
	}
}

// itemsReadable: a paid basket must name every line, or refunds and payouts cannot address them.
func itemsReadable(items []itemTransactionAnswer) bool {
	if len(items) == 0 {
		return false
	}
	for _, it := range items {
		if it.PaymentTransactionID == "" || it.PaidPrice == "" {
			return false
		}
	}
	return true
}

// HostedCheckoutResult reads a Checkout Form payment by its token. A transport failure is an
// error; an iyzico error answer is a no_verdict result; FAILURE is a decline even on an error
// answer, because iyzico reports a declined card there.
func (p *IyzicoProvider) HostedCheckoutResult(ctx context.Context, r provider.HostedCheckoutResultRequest) (*provider.HostedCheckoutResult, error) {
	body := map[string]string{"locale": defaultLocale, "conversationId": orDefault(r.ConversationID, r.Token), "token": r.Token}

	var a retrieveAnswer
	err := p.client.do(ctx, http.MethodPost, pathCFRetrieve, body, &a)
	var refusal *provider.ProviderRefusal
	if errors.As(err, &refusal) {
		res := a.result()
		res.ErrorCode, res.ErrorMessage = refusal.Code, refusal.Message
		if a.PaymentStatus != paymentStatusFailure {
			res.Readable = false
			return res, nil
		}
		// iyzico documents no signature on error answers, so an unsigned decline counts; a
		// signature that is sent must still match.
		if a.Signature != "" && !p.retrieveSigned(a) {
			res.Readable, res.ErrorCode = false, codeSignatureMismatch
			return res, nil
		}
		res.Outcome = provider.OutcomeDeclined
		return res, nil
	}
	if err != nil {
		return nil, err
	}

	res := a.result()
	outcome, code := a.outcome()
	if code != "" {
		res.Readable, res.ErrorCode = false, code
	}
	// A verdict on a success answer counts only when iyzico signed it; GoPay's TLS is not the
	// only hop.
	if outcome != provider.OutcomeNoVerdict && !p.retrieveSigned(a) {
		res.Readable, res.ErrorCode = false, codeSignatureMismatch
		return res, nil
	}
	res.Outcome = outcome
	return res, nil
}

// retrieveSigned checks iyzico's CF retrieve signature; a missing one does not match.
func (p *IyzicoProvider) retrieveSigned(a retrieveAnswer) bool {
	return p.client.signatureMatches(a.Signature, a.PaymentStatus, string(a.PaymentID), a.Currency, a.BasketID,
		a.ConversationID, string(a.PaidPrice), string(a.Price), a.Token)
}

type cancelAnswer struct {
	PaymentID      flexString `json:"paymentId"`
	Price          amount     `json:"price"`
	Currency       string     `json:"currency"`
	ConversationID string     `json:"conversationId"`
}

func withReason(body map[string]string, reason, description string) map[string]string {
	if reason != "" {
		body["reason"] = reason
	}
	if description != "" {
		body["description"] = description
	}
	return body
}

// CancelHostedPayment voids a whole payment. Banks accept it on the payment's day only; on a later
// day iyzico refuses and the caller refunds instead.
func (p *IyzicoProvider) CancelHostedPayment(ctx context.Context, r provider.HostedCancelRequest) (*provider.HostedCancelResult, error) {
	body := withReason(map[string]string{"locale": defaultLocale, "conversationId": conversationID(r.ConversationID),
		"paymentId": r.PaymentID, "ip": r.IP}, r.Reason, r.Description)

	var a cancelAnswer
	if err := p.client.do(ctx, http.MethodPost, pathCancel, body, &a); err != nil {
		return nil, err
	}
	return &provider.HostedCancelResult{PaymentID: orDefault(string(a.PaymentID), r.PaymentID), Price: string(a.Price),
		Currency: a.Currency, ConversationID: a.ConversationID}, nil
}

type refundAnswer struct {
	PaymentID            flexString `json:"paymentId"`
	PaymentTransactionID flexString `json:"paymentTransactionId"`
	Price                amount     `json:"price"`
	Currency             string     `json:"currency"`
	ConversationID       string     `json:"conversationId"`
	Signature            string     `json:"signature"`
}

// RefundHostedItem returns price of one basket item, up to 365 days after the payment. On the
// payment's day some banks refuse it with 10223 (end of day required): the caller cancels instead.
// iyzico does not deduplicate refunds, so an unavailable answer means the refund may have happened.
func (p *IyzicoProvider) RefundHostedItem(ctx context.Context, r provider.HostedRefundRequest) (*provider.HostedRefundResult, error) {
	body := withReason(map[string]string{"locale": defaultLocale, "conversationId": conversationID(r.ConversationID),
		"paymentTransactionId": r.PaymentTransactionID, "price": r.Price, "currency": r.Currency, "ip": r.IP},
		r.Reason, r.Description)

	var a refundAnswer
	if err := p.client.do(ctx, http.MethodPost, pathRefund, body, &a); err != nil {
		return nil, err
	}
	if a.Signature != "" && !p.client.signatureMatches(a.Signature, string(a.PaymentID), string(a.Price), a.Currency, a.ConversationID) {
		return nil, unavailable("refund answer signature does not match")
	}
	return &provider.HostedRefundResult{
		PaymentID:            string(a.PaymentID),
		PaymentTransactionID: orDefault(string(a.PaymentTransactionID), r.PaymentTransactionID),
		Price:                string(a.Price),
		Currency:             a.Currency,
		ConversationID:       a.ConversationID,
	}, nil
}
