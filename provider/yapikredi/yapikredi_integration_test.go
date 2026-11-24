//go:build integration

package yapikredi

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

// Runs against the Posnet test system. It only works from an IP the bank whitelisted for the
// merchant, so run it on the GoPay server:
//
//	YAPIKREDI_MERCHANT_ID=... YAPIKREDI_TERMINAL_ID=... YAPIKREDI_POSNET_ID=... YAPIKREDI_ENC_KEY=... \
//	go test -tags integration -run TestSandbox -v ./provider/yapikredi/
func sandboxProvider(t *testing.T) *YapiKrediProvider {
	t.Helper()
	conf := map[string]string{
		"merchantId":  os.Getenv("YAPIKREDI_MERCHANT_ID"),
		"terminalId":  os.Getenv("YAPIKREDI_TERMINAL_ID"),
		"posnetId":    os.Getenv("YAPIKREDI_POSNET_ID"),
		"encKey":      os.Getenv("YAPIKREDI_ENC_KEY"),
		"environment": "sandbox",
	}
	for k, v := range conf {
		if v == "" {
			t.Skipf("%s is not set", k)
		}
	}
	p := NewProvider().(*YapiKrediProvider)
	if err := p.Initialize(conf); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	// No database here: callback state and log rows are not written.
	p.storeCallback = func(context.Context, string, provider.CallbackState) (string, error) {
		return "https://gopay.example/v1/callback/yapikredi?state=0", nil
	}
	p.recordLog = nil
	return p
}

// sandboxCard is test card 1 from the bank's public XML document (section 2.5).
func sandboxCard() provider.CardInfo {
	return provider.CardInfo{CardHolderName: "TEST KART", CardNumber: "4506349043174632", ExpireMonth: "02", ExpireYear: "29", CVV: "000"}
}

func TestSandboxSaleStatusCancel(t *testing.T) {
	p := sandboxProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	sale, err := p.CreatePayment(ctx, provider.PaymentRequest{TenantID: 1, Amount: 1.01, Currency: "TRY", CardInfo: sandboxCard()})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	t.Logf("sale: %+v", sale)
	if sale.Status != provider.StatusSuccessful {
		t.Fatalf("sale status %s %s %s", sale.Status, sale.ErrorCode, sale.Message)
	}

	status, err := p.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: sale.PaymentID})
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	t.Logf("status after sale: %s %+v", status.Status, status.ProviderResponse)

	cancelled, err := p.CancelPayment(ctx, provider.CancelRequest{PaymentID: sale.PaymentID})
	if err != nil {
		t.Fatalf("CancelPayment: %v", err)
	}
	t.Logf("cancel: %+v", cancelled)

	after, err := p.GetPaymentStatus(ctx, provider.GetPaymentStatusRequest{PaymentID: sale.PaymentID})
	if err != nil {
		t.Fatalf("GetPaymentStatus: %v", err)
	}
	// The agreement state names are not documented; this log line is how they are learned.
	t.Logf("status after cancel: %s %+v", after.Status, after.ProviderResponse)
}

func TestSandbox3DStart(t *testing.T) {
	p := sandboxProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	resp, err := p.Create3DPayment(ctx, provider.PaymentRequest{
		TenantID: 1, Amount: 1.01, Currency: "TRY", CardInfo: sandboxCard(), CallbackURL: "https://example.com/return",
	})
	if err != nil {
		t.Fatalf("Create3DPayment: %v", err)
	}
	if resp.Status != provider.StatusPending || !strings.Contains(resp.HTML, "posnetData") {
		t.Fatalf("3D start: %+v", resp)
	}
	t.Logf("3D start %s ok, form %d bytes", resp.PaymentID, len(resp.HTML))
}
