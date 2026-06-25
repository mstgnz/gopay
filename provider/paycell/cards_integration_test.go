//go:build integration

package paycell

import (
	"context"
	"os"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

// Card-storage integration test against the Paycell sandbox. Opt-in only:
//
//	go test -tags integration ./provider/paycell/ -run TestCardStorageFlow -v
//
// Requires a reachable DB (request/response logging) and these env vars:
//
//	PAYCELL_USERNAME, PAYCELL_PASSWORD, PAYCELL_MERCHANT_ID, PAYCELL_SECURE_CODE,
//	PAYCELL_EULA_ID, PAYCELL_TEST_MSISDN, PAYCELL_TEST_OTP
//
// This is the source of truth for the real Paycell request/response field names: if any struct in
// cards.go is wrong, the parsing/assertions here fail and the shapes get corrected.
func TestCardStorageFlow(t *testing.T) {
	cfg := map[string]string{
		"username":    os.Getenv("PAYCELL_USERNAME"),
		"password":    os.Getenv("PAYCELL_PASSWORD"),
		"merchantId":  os.Getenv("PAYCELL_MERCHANT_ID"),
		"secureCode":  os.Getenv("PAYCELL_SECURE_CODE"),
		"eulaId":      os.Getenv("PAYCELL_EULA_ID"),
		"environment": "sandbox",
	}
	msisdn := os.Getenv("PAYCELL_TEST_MSISDN")
	otp := os.Getenv("PAYCELL_TEST_OTP")
	if cfg["username"] == "" || cfg["password"] == "" || msisdn == "" {
		t.Skip("paycell sandbox credentials not set; skipping card-storage integration test")
	}

	p := NewProvider().(*PaycellProvider)
	if err := p.Initialize(cfg); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	ctx := context.Background()
	card := testCards[0]

	// 1) Register a card and capture the provider cardId.
	reg, err := p.RegisterCard(ctx, provider.RegisterCardRequest{
		MSISDN: msisdn,
		Card: provider.CardInfo{
			CardNumber:  card.CardNumber,
			ExpireMonth: card.ExpireMonth,
			ExpireYear:  card.ExpireYear,
			CVV:         card.CVV,
		},
		Alias: "integration-test",
	})
	if err != nil {
		t.Fatalf("register card: %v", err)
	}
	if !reg.Success || reg.ProviderCardID == "" {
		t.Fatalf("register card unsuccessful: %+v", reg)
	}
	t.Logf("registered cardId=%s masked=%s brand=%s", reg.ProviderCardID, reg.MaskedCardNo, reg.CardBrand)

	// 2) OTP-validated wallet listing (PM_LIST flow).
	if otp != "" {
		send, err := p.SendCardOTP(ctx, provider.CardOTPSendRequest{MSISDN: msisdn})
		if err != nil {
			t.Fatalf("send otp: %v", err)
		}
		if _, err := p.ValidateCardOTP(ctx, provider.CardOTPValidateRequest{
			MSISDN:          msisdn,
			ReferenceNumber: send.ReferenceNumber,
			OTP:             otp,
			Token:           send.Token,
		}); err != nil {
			t.Fatalf("validate otp: %v", err)
		}
		list, err := p.ListProviderCards(ctx, provider.ListCardsRequest{MSISDN: msisdn, ReferenceNumber: send.ReferenceNumber})
		if err != nil {
			t.Fatalf("list cards: %v", err)
		}
		t.Logf("wallet has %d card(s)", len(list.Cards))
	}

	// 3) Charge the saved card without 3D (auto top-up path).
	pay, err := p.PayWithSavedCard(ctx, provider.SavedCardPaymentRequest{
		MSISDN:         msisdn,
		ProviderCardID: reg.ProviderCardID,
		Amount:         1.00,
		Currency:       "TRY",
	})
	if err != nil {
		t.Fatalf("pay with saved card: %v", err)
	}
	t.Logf("payment success=%v status=%s message=%s", pay.Success, pay.Status, pay.Message)

	// 4) Delete the saved card from the wallet.
	if _, err := p.DeleteProviderCard(ctx, provider.DeleteCardRequest{MSISDN: msisdn, ProviderCardID: reg.ProviderCardID}); err != nil {
		t.Fatalf("delete card: %v", err)
	}
}
