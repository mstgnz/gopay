package iyzico

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

func personal() provider.SubMerchant {
	return provider.SubMerchant{ExternalID: "fac-1", Type: provider.SubMerchantPersonal, Name: "Moda Spor",
		Email: "info@moda.example", GSMNumber: "+905551112233", Address: "Moda Cd. 1", IBAN: "TR330006100519786457841326",
		ContactName: "Ada", ContactSurname: "Kaya", IdentityNumber: "12345678901"}
}

func TestCreateSubMerchantSendsTheIyzicoBody(t *testing.T) {
	p, fake := newTestProvider(t, http.StatusOK, `{"status":"success","subMerchantKey":"smk-1","conversationId":"c"}`)
	got, err := p.CreateSubMerchant(context.Background(), provider.SubMerchantRequest{ConversationID: "c", SubMerchant: personal()})
	if err != nil {
		t.Fatal(err)
	}
	if got.SubMerchantKey != "smk-1" {
		t.Errorf("key = %q", got.SubMerchantKey)
	}
	sent := fake.last()
	want := map[string]any{"locale": "tr", "conversationId": "c", "subMerchantExternalId": "fac-1", "subMerchantType": "PERSONAL",
		"name": "Moda Spor", "email": "info@moda.example", "gsmNumber": "+905551112233", "address": "Moda Cd. 1",
		"iban": "TR330006100519786457841326", "currency": "TRY", "contactName": "Ada", "contactSurname": "Kaya",
		"identityNumber": "12345678901"}
	if sent.method != http.MethodPost || sent.path != pathSubMerchant || !reflect.DeepEqual(sent.body, want) {
		t.Errorf("sent %s %s %v", sent.method, sent.path, sent.body)
	}
}

func TestCreateSubMerchantChecksTheTypeFields(t *testing.T) {
	cases := map[string]struct {
		edit    func(*provider.SubMerchant)
		missing string
	}{
		"unknown type":           {func(s *provider.SubMerchant) { s.Type = "OTHER" }, "type must be"},
		"personal without tckn":  {func(s *provider.SubMerchant) { s.IdentityNumber = "" }, "identityNumber"},
		"company without office": {func(s *provider.SubMerchant) { s.Type = provider.SubMerchantPrivateCompany }, "taxOffice, legalCompanyTitle"},
		"limited without tax no.": {func(s *provider.SubMerchant) {
			s.Type, s.TaxOffice, s.LegalCompanyTitle = provider.SubMerchantLimitedOrJointStockCompany, "Kadikoy", "Moda A.S."
		}, "taxNumber"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, fake := newTestProvider(t, http.StatusOK, `{"status":"success","subMerchantKey":"k"}`)
			sm := personal()
			tc.edit(&sm)
			_, err := p.CreateSubMerchant(context.Background(), provider.SubMerchantRequest{SubMerchant: sm})
			if !errors.Is(err, provider.ErrInvalidRequest) || !strings.Contains(err.Error(), tc.missing) {
				t.Fatalf("want invalid naming %q, got %v", tc.missing, err)
			}
			if len(fake.requests) != 0 {
				t.Error("an invalid sub-merchant must not reach iyzico")
			}
		})
	}
}

func TestCreateSubMerchantWithoutKeyIsUnavailable(t *testing.T) {
	p, _ := newTestProvider(t, http.StatusOK, `{"status":"success"}`)
	if _, err := p.CreateSubMerchant(context.Background(), provider.SubMerchantRequest{SubMerchant: personal()}); !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Fatalf("want unavailable, got %v", err)
	}
}

func TestCreateSubMerchantDuplicateIsARefusal(t *testing.T) {
	p, _ := newTestProvider(t, http.StatusOK, `{"status":"failure","errorCode":"2002","errorMessage":"Sub merchant does exist for ID"}`)
	_, err := p.CreateSubMerchant(context.Background(), provider.SubMerchantRequest{SubMerchant: personal()})
	var refusal *provider.ProviderRefusal
	if !errors.As(err, &refusal) || refusal.Code != "2002" {
		t.Fatalf("want refusal 2002, got %v", err)
	}
}

func TestUpdateSubMerchantSendsKeyNotTypeOrExternalID(t *testing.T) {
	p, fake := newTestProvider(t, http.StatusOK, `{"status":"success","conversationId":"c"}`)
	got, err := p.UpdateSubMerchant(context.Background(), provider.SubMerchantRequest{Key: "smk-1", SubMerchant: personal()})
	if err != nil {
		t.Fatal(err)
	}
	if got.SubMerchantKey != "smk-1" {
		t.Errorf("key = %q", got.SubMerchantKey)
	}
	sent := fake.last()
	if sent.method != http.MethodPut || sent.body["subMerchantKey"] != "smk-1" {
		t.Errorf("sent %s %v", sent.method, sent.body)
	}
	for _, f := range []string{"subMerchantType", "subMerchantExternalId"} {
		if _, ok := sent.body[f]; ok {
			t.Errorf("update must not send %s", f)
		}
	}
}

func TestUpdateSubMerchantRequiresIBAN(t *testing.T) {
	p, fake := newTestProvider(t, http.StatusOK, `{"status":"success"}`)
	sm := personal()
	sm.IBAN = ""
	if _, err := p.UpdateSubMerchant(context.Background(), provider.SubMerchantRequest{Key: "k", SubMerchant: sm}); !errors.Is(err, provider.ErrInvalidRequest) {
		t.Fatalf("want invalid, got %v", err)
	}
	if len(fake.requests) != 0 {
		t.Error("an invalid update must not reach iyzico")
	}
}

func TestSubMerchantDetail(t *testing.T) {
	p, fake := newTestProvider(t, http.StatusOK, `{"status":"success","subMerchantKey":"smk-1","subMerchantExternalId":"fac-1",
		"subMerchantType":"PERSONAL","name":"Moda Spor","iban":"TR330006100519786457841326","identityNumber":"12345678901","currency":"TRY"}`)
	got, err := p.SubMerchantDetail(context.Background(), provider.SubMerchantDetailRequest{ExternalID: "fac-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.SubMerchantKey != "smk-1" || got.Name != "Moda Spor" || got.IBAN != "TR330006100519786457841326" || got.Type != "PERSONAL" {
		t.Errorf("detail = %+v", got)
	}
	if sent := fake.last(); sent.path != pathSubMerchantDetail || sent.body["subMerchantExternalId"] != "fac-1" {
		t.Errorf("sent %s %v", sent.path, sent.body)
	}

	p, _ = newTestProvider(t, http.StatusOK, `{"status":"success"}`)
	if _, err := p.SubMerchantDetail(context.Background(), provider.SubMerchantDetailRequest{ExternalID: "x"}); !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Fatalf("a detail without a key must be unavailable, got %v", err)
	}
}

func TestItemApprovalAndDisapproval(t *testing.T) {
	for name, tc := range map[string]struct {
		call func(*IyzicoProvider, context.Context, provider.ItemApprovalRequest) (*provider.ItemApprovalResult, error)
		path string
	}{
		"approve":    {(*IyzicoProvider).ApproveItem, pathItemApprove},
		"disapprove": {(*IyzicoProvider).DisapproveItem, pathItemDisapprove},
	} {
		t.Run(name, func(t *testing.T) {
			p, fake := newTestProvider(t, http.StatusOK, `{"status":"success","paymentTransactionId":777,"conversationId":"c"}`)
			got, err := tc.call(p, context.Background(), provider.ItemApprovalRequest{PaymentTransactionID: "777", ConversationID: "c"})
			if err != nil {
				t.Fatal(err)
			}
			if got.PaymentTransactionID != "777" || got.ConversationID != "c" {
				t.Errorf("result = %+v", got)
			}
			want := map[string]any{"locale": "tr", "conversationId": "c", "paymentTransactionId": "777"}
			if sent := fake.last(); sent.path != tc.path || !reflect.DeepEqual(sent.body, want) {
				t.Errorf("sent %s %v", sent.path, sent.body)
			}
		})
	}

	p, _ := newTestProvider(t, http.StatusOK, `{"status":"failure","errorCode":"5064","errorMessage":"Item approval is already given"}`)
	_, err := p.ApproveItem(context.Background(), provider.ItemApprovalRequest{PaymentTransactionID: "777"})
	var refusal *provider.ProviderRefusal
	if !errors.As(err, &refusal) || refusal.Code != "5064" {
		t.Fatalf("want refusal 5064, got %v", err)
	}
}
