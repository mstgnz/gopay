package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

type fakeMarketplaceService struct {
	err  error
	op   string
	got  any
	envs []string
}

func (f *fakeMarketplaceService) keep(op, env string, req any) {
	f.op, f.got, f.envs = op, req, append(f.envs, env)
}

func (f *fakeMarketplaceService) CreateSubMerchant(_ context.Context, env, _ string, r provider.SubMerchantRequest) (*provider.SubMerchantResult, error) {
	f.keep("create", env, r)
	if f.err != nil {
		return nil, f.err
	}
	return &provider.SubMerchantResult{SubMerchantKey: "smk"}, nil
}

func (f *fakeMarketplaceService) UpdateSubMerchant(_ context.Context, env, _ string, r provider.SubMerchantRequest) (*provider.SubMerchantResult, error) {
	f.keep("update", env, r)
	if f.err != nil {
		return nil, f.err
	}
	return &provider.SubMerchantResult{SubMerchantKey: r.Key}, nil
}

func (f *fakeMarketplaceService) SubMerchantDetail(_ context.Context, env, _ string, r provider.SubMerchantDetailRequest) (*provider.SubMerchantDetail, error) {
	f.keep("detail", env, r)
	if f.err != nil {
		return nil, f.err
	}
	return &provider.SubMerchantDetail{SubMerchantKey: "smk", ExternalID: r.ExternalID}, nil
}

func (f *fakeMarketplaceService) ApproveItem(_ context.Context, env, _ string, r provider.ItemApprovalRequest) (*provider.ItemApprovalResult, error) {
	f.keep("approve", env, r)
	if f.err != nil {
		return nil, f.err
	}
	return &provider.ItemApprovalResult{PaymentTransactionID: r.PaymentTransactionID}, nil
}

func (f *fakeMarketplaceService) DisapproveItem(_ context.Context, env, _ string, r provider.ItemApprovalRequest) (*provider.ItemApprovalResult, error) {
	f.keep("disapprove", env, r)
	if f.err != nil {
		return nil, f.err
	}
	return &provider.ItemApprovalResult{PaymentTransactionID: r.PaymentTransactionID}, nil
}

const createSubMerchantJSON = `{"externalId":"fac-1","type":"PERSONAL","name":"Moda Spor","email":"info@moda.example",
  "gsmNumber":"+905551112233","address":"Moda Cd. 1","iban":"TR330006100519786457841326","contactName":"Ada",
  "contactSurname":"Kaya","identityNumber":"12345678901"}`

func TestMarketplaceRoutesMapTheRequests(t *testing.T) {
	svc := &fakeMarketplaceService{}
	h := checkoutRouter(&fakeCheckoutService{}, svc)

	code, out := call(t, h, "POST", "/payments/iyzico/submerchants?environment=sandbox", createSubMerchantJSON)
	got := svc.got.(provider.SubMerchantRequest)
	if code != http.StatusOK || !out.Success || got.SubMerchant.ExternalID != "fac-1" || got.SubMerchant.Type != "PERSONAL" ||
		got.SubMerchant.IBAN != "TR330006100519786457841326" || got.Key != "" {
		t.Errorf("create: %d %+v", code, got)
	}

	code, _ = call(t, h, "PUT", "/payments/iyzico/submerchants/smk-1?environment=sandbox",
		`{"email":"info@moda.example","gsmNumber":"+905551112233","address":"Moda Cd. 1","iban":"TR330006100519786457841326"}`)
	got = svc.got.(provider.SubMerchantRequest)
	if code != http.StatusOK || svc.op != "update" || got.Key != "smk-1" {
		t.Errorf("update: %d %s %+v", code, svc.op, got)
	}

	code, _ = call(t, h, "GET", "/payments/iyzico/submerchants/fac-1?environment=sandbox", "")
	if code != http.StatusOK || svc.op != "detail" || svc.got.(provider.SubMerchantDetailRequest).ExternalID != "fac-1" {
		t.Errorf("detail: %d %s %+v", code, svc.op, svc.got)
	}

	for _, op := range []string{"approve", "disapprove"} {
		code, _ = call(t, h, "POST", "/payments/iyzico/items/777/"+op+"?environment=sandbox", "")
		if code != http.StatusOK || svc.op != op || svc.got.(provider.ItemApprovalRequest).PaymentTransactionID != "777" {
			t.Errorf("%s with an empty body: %d %s %+v", op, code, svc.op, svc.got)
		}
	}
	code, _ = call(t, h, "POST", "/payments/iyzico/items/777/approve?environment=sandbox", `{"conversationId":"c"}`)
	if code != http.StatusOK || svc.got.(provider.ItemApprovalRequest).ConversationID != "c" {
		t.Errorf("approve with a body: %d %+v", code, svc.got)
	}
}

func TestMarketplaceRoutesRefuseBadRequests(t *testing.T) {
	h := checkoutRouter(&fakeCheckoutService{}, &fakeMarketplaceService{})
	cases := []struct{ method, path, body string }{
		{"POST", "/payments/iyzico/submerchants?environment=sandbox", `{"externalId":"f","type":"OTHER","email":"a@b.co","gsmNumber":"+905551112233","address":"x"}`},
		{"POST", "/payments/iyzico/submerchants?environment=sandbox", `{"type":"PERSONAL","email":"a@b.co","gsmNumber":"+905551112233","address":"x"}`},
		{"POST", "/payments/iyzico/submerchants?environment=sandbox", `{"externalId":"f","type":"PERSONAL","email":"not-mail","gsmNumber":"+905551112233","address":"x"}`},
		{"POST", "/payments/iyzico/submerchants?environment=sandbox", `{"externalId":"f","type":"PERSONAL","email":"a@b.co","gsmNumber":"+905551112233","address":"x","iban":"TR33 0006"}`},
		{"PUT", "/payments/iyzico/submerchants/k?environment=sandbox", `{"email":"a@b.co","gsmNumber":"+905551112233","address":"x","type":"PERSONAL"}`},
		{"POST", "/payments/iyzico/items/777/approve?environment=sandbox", `{"paymentTransactionId":"999"}`},
		{"POST", "/payments/iyzico/items/777/approve", ``},
		{"GET", "/payments/iyzico/submerchants/f?environment=staging", ``},
	}
	for _, tc := range cases {
		if code, _ := call(t, h, tc.method, tc.path, tc.body); code != http.StatusBadRequest {
			t.Errorf("%s %s %s: want 400, got %d", tc.method, tc.path, tc.body, code)
		}
	}
}

func TestMarketplaceRoutesMapProviderErrors(t *testing.T) {
	h := checkoutRouter(&fakeCheckoutService{}, &fakeMarketplaceService{err: &provider.ProviderRefusal{Code: "2002", Message: "Sub merchant does exist for ID"}})
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/payments/iyzico/submerchants?environment=sandbox", createSubMerchantJSON},
		{"PUT", "/payments/iyzico/submerchants/k?environment=sandbox", `{"email":"a@b.co","gsmNumber":"+905551112233","address":"x"}`},
		{"GET", "/payments/iyzico/submerchants/f?environment=sandbox", ``},
		{"POST", "/payments/iyzico/items/7/disapprove?environment=sandbox", ``},
	} {
		code, out := call(t, h, tc.method, tc.path, tc.body)
		if code != http.StatusUnprocessableEntity || out.Data.(map[string]any)["providerErrorCode"] != "2002" {
			t.Errorf("%s %s: got %d %+v", tc.method, tc.path, code, out)
		}
	}
}
