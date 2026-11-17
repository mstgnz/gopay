package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/response"
	"github.com/mstgnz/gopay/provider"
)

// fakeCheckoutService returns err, or a fixed answer, and keeps the last request.
type fakeCheckoutService struct {
	err         error
	environment string
	provider    string
	got         any
	outcome     provider.PaymentOutcome
}

func (f *fakeCheckoutService) keep(env, name string, req any) {
	f.environment, f.provider, f.got = env, name, req
}

func (f *fakeCheckoutService) StartCheckout(_ context.Context, env, name string, r provider.HostedCheckoutRequest) (*provider.HostedCheckoutSession, error) {
	f.keep(env, name, r)
	if f.err != nil {
		return nil, f.err
	}
	return &provider.HostedCheckoutSession{Token: "tok", PaymentPageURL: "https://p", ConversationID: "c"}, nil
}

func (f *fakeCheckoutService) CheckoutResult(_ context.Context, env, name string, r provider.HostedCheckoutResultRequest) (*provider.HostedCheckoutResult, error) {
	f.keep(env, name, r)
	if f.err != nil {
		return nil, f.err
	}
	return &provider.HostedCheckoutResult{Outcome: f.outcome, Items: []provider.CheckoutItemResult{}}, nil
}

func (f *fakeCheckoutService) CancelPayment(_ context.Context, env, name string, r provider.HostedCancelRequest) (*provider.HostedCancelResult, error) {
	f.keep(env, name, r)
	if f.err != nil {
		return nil, f.err
	}
	return &provider.HostedCancelResult{PaymentID: r.PaymentID}, nil
}

func (f *fakeCheckoutService) RefundItem(_ context.Context, env, name string, r provider.HostedRefundRequest) (*provider.HostedRefundResult, error) {
	f.keep(env, name, r)
	if f.err != nil {
		return nil, f.err
	}
	return &provider.HostedRefundResult{PaymentTransactionID: r.PaymentTransactionID, Price: r.Price}, nil
}

func checkoutRouter(svc CheckoutServiceInterface, mkt MarketplaceServiceInterface) http.Handler {
	v := validator.New()
	ch, mh := NewCheckoutHandler(svc, v), NewMarketplaceHandler(mkt, v)
	r := chi.NewRouter()
	r.Post("/payments/{provider}/checkout", ch.StartCheckout)
	r.Get("/payments/{provider}/checkout/{token}", ch.CheckoutResult)
	r.Post("/payments/{provider}/checkout/cancel", ch.CancelPayment)
	r.Post("/payments/{provider}/checkout/refund", ch.RefundItem)
	r.Post("/payments/{provider}/submerchants", mh.CreateSubMerchant)
	r.Put("/payments/{provider}/submerchants/{key}", mh.UpdateSubMerchant)
	r.Get("/payments/{provider}/submerchants/{externalId}", mh.SubMerchantDetail)
	r.Post("/payments/{provider}/items/{paymentTransactionId}/approve", mh.ApproveItem)
	r.Post("/payments/{provider}/items/{paymentTransactionId}/disapprove", mh.DisapproveItem)
	return r
}

func call(t *testing.T, h http.Handler, method, path, body string) (int, response.Response) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s answered non-JSON %q", method, path, rec.Body.String())
	}
	return rec.Code, out
}

const startBody = `{
  "conversationId": "c-1", "basketId": "B-1", "price": "450.50", "callbackUrl": "https://tenant.example/odeme",
  "buyer": {"id": "u-7", "name": "Ada", "surname": "Kaya", "identityNumber": "12345678901", "email": "ada@example.com",
            "gsmNumber": "+905551112233", "address": "Moda Cd. 1", "city": "Istanbul", "ip": "85.34.78.112"},
  "items": [{"id": "BK-9", "name": "Saha 1", "category": "Saha", "itemType": "VIRTUAL", "price": "450.50",
             "subMerchantKey": "smk-1", "subMerchantPrice": "405.45"}]
}`

func TestStartCheckoutMapsTheBody(t *testing.T) {
	svc := &fakeCheckoutService{}
	code, out := call(t, checkoutRouter(svc, &fakeMarketplaceService{}), "POST", "/payments/iyzico/checkout?environment=sandbox", startBody)
	if code != http.StatusOK || !out.Success {
		t.Fatalf("got %d %+v", code, out)
	}
	if svc.environment != "sandbox" || svc.provider != "iyzico" {
		t.Errorf("routed to %s/%s", svc.provider, svc.environment)
	}
	got := svc.got.(provider.HostedCheckoutRequest)
	if got.Price != "450.50" || got.Currency != "TRY" || got.Buyer.IP != "85.34.78.112" || got.Buyer.IdentityNumber != "12345678901" {
		t.Errorf("request = %+v", got)
	}
	if got.Items[0].SubMerchantPrice != "405.45" || got.Items[0].ItemType != "VIRTUAL" {
		t.Errorf("item = %+v", got.Items[0])
	}
	if got.BillingAddress.ContactName != "Ada Kaya" || got.BillingAddress.Address != "Moda Cd. 1" || got.ShippingAddress != nil {
		t.Errorf("billing must default to the buyer: %+v, shipping %+v", got.BillingAddress, got.ShippingAddress)
	}
}

func TestStartCheckoutKeepsGivenAddresses(t *testing.T) {
	svc := &fakeCheckoutService{}
	body := strings.Replace(startBody, `"items":`, `"billingAddress": {"contactName": "Moda Ltd", "address": "Fatura Sk. 2", "city": "Istanbul"},
  "shippingAddress": {"contactName": "Ada Kaya", "address": "Teslim Sk. 3", "city": "Izmir", "zipCode": "35000"},
  "items":`, 1)
	if code, out := call(t, checkoutRouter(svc, &fakeMarketplaceService{}), "POST", "/payments/iyzico/checkout?environment=sandbox", body); code != http.StatusOK {
		t.Fatalf("got %d %+v", code, out)
	}
	got := svc.got.(provider.HostedCheckoutRequest)
	if got.BillingAddress.ContactName != "Moda Ltd" || got.ShippingAddress == nil || got.ShippingAddress.ZipCode != "35000" {
		t.Errorf("addresses = %+v / %+v", got.BillingAddress, got.ShippingAddress)
	}
}

func TestStartCheckoutRefusesBadBodies(t *testing.T) {
	h := checkoutRouter(&fakeCheckoutService{}, &fakeMarketplaceService{})
	cases := map[string]string{
		"unknown field":   strings.Replace(startBody, `"basketId"`, `"amount": 1, "basketId"`, 1),
		"numeric price":   strings.Replace(startBody, `"price": "450.50", "callbackUrl"`, `"price": 450.5, "callbackUrl"`, 1),
		"missing ip":      strings.Replace(startBody, `, "ip": "85.34.78.112"`, ``, 1),
		"bad ip":          strings.Replace(startBody, `"85.34.78.112"`, `"localhost"`, 1),
		"no identity":     strings.Replace(startBody, `"identityNumber": "12345678901", `, ``, 1),
		"bad phone":       strings.Replace(startBody, `+905551112233`, `05551112233`, 1),
		"bad item type":   strings.Replace(startBody, `"VIRTUAL"`, `"SERVICE"`, 1),
		"two objects":     startBody + `{}`,
		"not json":        `price=1`,
		"empty items":     strings.SplitN(startBody, `"items":`, 2)[0] + `"items": []}`,
		"other currency":  strings.Replace(startBody, `"basketId"`, `"currency": "USD", "basketId"`, 1),
		"callback no url": strings.Replace(startBody, `https://tenant.example/odeme`, `not a url`, 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if code, _ := call(t, h, "POST", "/payments/iyzico/checkout?environment=sandbox", body); code != http.StatusBadRequest {
				t.Errorf("want 400, got %d", code)
			}
		})
	}
	if code, _ := call(t, h, "POST", "/payments/iyzico/checkout", startBody); code != http.StatusBadRequest {
		t.Errorf("a missing environment must be 400, got %d", code)
	}
}

func TestProviderErrorContract(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		kind string
	}{
		{"invalid", fmt.Errorf("x: %w", provider.ErrInvalidRequest), http.StatusBadRequest, ""},
		{"unsupported checkout", provider.ErrCheckoutUnsupported, http.StatusBadRequest, ""},
		{"unsupported marketplace", provider.ErrMarketplaceUnsupported, http.StatusBadRequest, ""},
		{"refused", &provider.ProviderRefusal{Code: "10223", Message: "End of day required", Retryable: true}, http.StatusUnprocessableEntity, kindRefused},
		{"unavailable", fmt.Errorf("x: %w", provider.ErrProviderUnavailable), http.StatusBadGateway, kindUnavailable},
		{"deadline", context.DeadlineExceeded, http.StatusBadGateway, kindUnavailable},
		{"internal", errors.New("no configuration found"), http.StatusInternalServerError, kindInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := checkoutRouter(&fakeCheckoutService{err: tc.err}, &fakeMarketplaceService{})
			code, out := call(t, h, "POST", "/payments/iyzico/checkout/refund?environment=sandbox",
				`{"paymentTransactionId":"7","price":"10.00","ip":"85.34.78.112"}`)
			if code != tc.code || out.Success {
				t.Fatalf("got %d %+v, want %d", code, out, tc.code)
			}
			if tc.kind == "" {
				return
			}
			data := out.Data.(map[string]any)
			if data["kind"] != tc.kind {
				t.Errorf("kind = %v, want %s", data["kind"], tc.kind)
			}
			if tc.kind == kindRefused && (data["providerErrorCode"] != "10223" || data["retryable"] != true || data["providerErrorMessage"] != "End of day required") {
				t.Errorf("refusal data = %v", data)
			}
		})
	}
}

func TestCheckoutResultEnvelope(t *testing.T) {
	for outcome, success := range map[provider.PaymentOutcome]bool{provider.OutcomePaid: true, provider.OutcomeInReview: false,
		provider.OutcomeDeclined: false, provider.OutcomeNoVerdict: false} {
		svc := &fakeCheckoutService{outcome: outcome}
		code, out := call(t, checkoutRouter(svc, &fakeMarketplaceService{}), "GET", "/payments/iyzico/checkout/tok-1?environment=production&conversationId=c-1", "")
		if code != http.StatusOK || out.Success != success {
			t.Errorf("%s: got %d success=%v", outcome, code, out.Success)
		}
		if out.Data.(map[string]any)["outcome"] != string(outcome) {
			t.Errorf("%s: data = %v", outcome, out.Data)
		}
		if got := svc.got.(provider.HostedCheckoutResultRequest); got.Token != "tok-1" || got.ConversationID != "c-1" {
			t.Errorf("request = %+v", got)
		}
	}
	h := checkoutRouter(&fakeCheckoutService{}, &fakeMarketplaceService{})
	if code, _ := call(t, h, "GET", "/payments/iyzico/checkout/t?environment=x", ""); code != http.StatusBadRequest {
		t.Errorf("bad environment: %d", code)
	}
	if code, _ := call(t, h, "GET", "/payments/iyzico/checkout/t?environment=sandbox&conversationId="+strings.Repeat("c", 65), ""); code != http.StatusBadRequest {
		t.Errorf("long conversationId: %d", code)
	}
	if code, _ := call(t, checkoutRouter(&fakeCheckoutService{err: provider.ErrProviderUnavailable}, &fakeMarketplaceService{}),
		"GET", "/payments/iyzico/checkout/t?environment=sandbox", ""); code != http.StatusBadGateway {
		t.Errorf("an unavailable lookup must be 502, never a failed payment: %d", code)
	}
}

func TestCancelAndRefundMapTheBody(t *testing.T) {
	svc := &fakeCheckoutService{}
	h := checkoutRouter(svc, &fakeMarketplaceService{})

	code, _ := call(t, h, "POST", "/payments/iyzico/checkout/cancel?environment=sandbox",
		`{"paymentId":"123","ip":"85.34.78.112","reason":"BUYER_REQUEST","description":"x"}`)
	if got := svc.got.(provider.HostedCancelRequest); code != http.StatusOK || got.PaymentID != "123" || got.IP != "85.34.78.112" || got.Reason != "BUYER_REQUEST" {
		t.Errorf("cancel: %d %+v", code, svc.got)
	}
	code, _ = call(t, h, "POST", "/payments/iyzico/checkout/refund?environment=sandbox",
		`{"paymentTransactionId":"7","price":"10.50","ip":"2001:db8::1","conversationId":"c"}`)
	if got := svc.got.(provider.HostedRefundRequest); code != http.StatusOK || got.Price != "10.50" || got.Currency != "TRY" || got.IP != "2001:db8::1" {
		t.Errorf("refund: %d %+v", code, svc.got)
	}

	for name, body := range map[string]string{
		"cancel without ip":    `{"paymentId":"123"}`,
		"cancel lower reason":  `{"paymentId":"123","ip":"1.1.1.1","reason":"buyer_request"}`,
		"cancel unknown field": `{"paymentId":"123","ip":"1.1.1.1","amount":"1"}`,
	} {
		if code, _ := call(t, h, "POST", "/payments/iyzico/checkout/cancel?environment=sandbox", body); code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, code)
		}
	}
	for name, body := range map[string]string{
		"refund numeric price": `{"paymentTransactionId":"7","price":10.5,"ip":"1.1.1.1"}`,
		"refund without ip":    `{"paymentTransactionId":"7","price":"10.50"}`,
		"refund euro":          `{"paymentTransactionId":"7","price":"10.50","ip":"1.1.1.1","currency":"EUR"}`,
	} {
		if code, _ := call(t, h, "POST", "/payments/iyzico/checkout/refund?environment=sandbox", body); code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, code)
		}
	}
}
