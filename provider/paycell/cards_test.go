package paycell

import (
	"testing"

	"github.com/mstgnz/gopay/provider"
)

func TestNormalizeMsisdn(t *testing.T) {
	cases := map[string]string{
		"+905551234567": "5551234567",
		"905551234567":  "5551234567",
		"5551234567":    "5551234567",
		" 5551234567 ":  "5551234567",
	}
	for in, want := range cases {
		if got := normalizeMsisdn(in); got != want {
			t.Errorf("normalizeMsisdn(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestBuildCardIdProvisionRequest_NoToken is the load-bearing invariant for saved-card payments:
// the provision body must carry cardId and must NOT carry cardToken.
func TestBuildCardIdProvisionRequest_NoToken(t *testing.T) {
	p := &PaycellProvider{merchantID: "9998", username: "u", password: "pw"}

	req := p.buildCardIdProvisionRequest(provider.SavedCardPaymentRequest{
		Amount:           100.50,
		Currency:         "TRY",
		MSISDN:           "+905551234567",
		ProviderCardID:   "card-abc",
		InstallmentCount: 0,
	}, "")

	if _, ok := req["cardToken"]; ok {
		t.Fatal("cardToken must NOT be present when paying with a saved cardId")
	}
	if req["cardId"] != "card-abc" {
		t.Fatalf("cardId = %v, want card-abc", req["cardId"])
	}
	if req["amount"] != "10050" {
		t.Fatalf("amount (kuruş) = %v, want 10050", req["amount"])
	}
	if req["msisdn"] != "5551234567" {
		t.Fatalf("msisdn = %v, want normalized 5551234567", req["msisdn"])
	}
	if req["installmentCount"] != 1 {
		t.Fatalf("installmentCount = %v, want default 1", req["installmentCount"])
	}
	if req["paymentType"] != "SALE" || req["paymentMethodType"] != "CREDIT_CARD" {
		t.Fatalf("unexpected payment type fields: %v / %v", req["paymentType"], req["paymentMethodType"])
	}
	if _, ok := req["threeDSessionId"]; ok {
		t.Fatal("threeDSessionId must be omitted for non-3D saved-card payment")
	}
}

func TestBuildCardIdProvisionRequest_3D(t *testing.T) {
	p := &PaycellProvider{merchantID: "9998"}

	req := p.buildCardIdProvisionRequest(provider.SavedCardPaymentRequest{
		Amount:           1,
		Currency:         "TRY",
		MSISDN:           "5551234567",
		ProviderCardID:   "c",
		InstallmentCount: 3,
	}, "3dsess")

	if req["threeDSessionId"] != "3dsess" {
		t.Fatalf("threeDSessionId = %v, want 3dsess", req["threeDSessionId"])
	}
	if req["installmentCount"] != 3 {
		t.Fatalf("installmentCount = %v, want 3", req["installmentCount"])
	}
	if _, ok := req["cardToken"]; ok {
		t.Fatal("cardToken must NOT be present for 3D saved-card payment")
	}
}

// Compile-time guarantee that Paycell implements the optional capability interface.
func TestPaycellImplementsCardStorageProvider(t *testing.T) {
	var _ provider.CardStorageProvider = (*PaycellProvider)(nil)
}
