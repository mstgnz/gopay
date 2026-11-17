//go:build integration

package iyzico

// Sandbox tests against sandbox-api.iyzipay.com. They need a sandbox key pair:
//
//	IYZICO_TEST_API_KEY=... IYZICO_TEST_SECRET_KEY=... go test -tags integration -run TestIntegration ./provider/iyzico/
//
// IYZICO_TEST_SUBMERCHANT_KEY (a sub-merchant of a marketplace sandbox account) makes the basket a
// marketplace sale. Paying the page needs a browser: open the printed URL, pay with a test card
// (docs.iyzico.com "Test Cards"; 5528790000000008 succeeds, 4111111111111129 has no funds), then
// rerun TestIntegrationRetrieve with IYZICO_TEST_TOKEN set to read the verdict.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mstgnz/gopay/provider"
)

func sandboxProvider(t *testing.T) *IyzicoProvider {
	t.Helper()
	apiKey, secretKey := os.Getenv("IYZICO_TEST_API_KEY"), os.Getenv("IYZICO_TEST_SECRET_KEY")
	if apiKey == "" || secretKey == "" {
		t.Skip("IYZICO_TEST_API_KEY and IYZICO_TEST_SECRET_KEY are not set")
	}
	p := NewProvider().(*IyzicoProvider)
	if err := p.Initialize(map[string]string{"apiKey": apiKey, "secretKey": secretKey, "environment": "sandbox"}); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIntegrationCheckoutStartsAndIsOpen(t *testing.T) {
	p := sandboxProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	r := checkoutRequest()
	r.ConversationID = fmt.Sprintf("it-%d", time.Now().UnixNano())
	r.BasketID = r.ConversationID
	if key := os.Getenv("IYZICO_TEST_SUBMERCHANT_KEY"); key != "" {
		r.Items[0].SubMerchantKey = key
	} else {
		r.Items[0].SubMerchantKey, r.Items[0].SubMerchantPrice = "", ""
	}
	if err := provider.ValidateHostedCheckout(r, "sandbox"); err != nil {
		t.Fatal(err)
	}

	session, err := p.StartHostedCheckout(ctx, r)
	if err != nil {
		t.Fatalf("StartHostedCheckout: %v", err)
	}
	t.Logf("token=%s page=%s expires=%ds", session.Token, session.PaymentPageURL, session.TokenExpireSeconds)

	result, err := p.HostedCheckoutResult(ctx, provider.HostedCheckoutResultRequest{Token: session.Token})
	if err != nil {
		t.Fatalf("HostedCheckoutResult: %v", err)
	}
	if result.Outcome != provider.OutcomeNoVerdict {
		t.Errorf("an unpaid page must be no verdict, got %+v", result)
	}
}

func TestIntegrationRetrieve(t *testing.T) {
	p := sandboxProvider(t)
	token := os.Getenv("IYZICO_TEST_TOKEN")
	if token == "" {
		t.Skip("IYZICO_TEST_TOKEN is not set")
	}
	result, err := p.HostedCheckoutResult(context.Background(), provider.HostedCheckoutResultRequest{Token: token})
	if err != nil {
		t.Fatalf("HostedCheckoutResult: %v", err)
	}
	t.Logf("result: %+v", result)
}

func TestIntegrationUnknownSubMerchantIsARefusal(t *testing.T) {
	p := sandboxProvider(t)
	_, err := p.SubMerchantDetail(context.Background(), provider.SubMerchantDetailRequest{ExternalID: fmt.Sprintf("none-%d", time.Now().UnixNano())})
	var refusal *provider.ProviderRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("want a refusal, got %v", err)
	}
	t.Logf("refusal: %s %s", refusal.Code, refusal.Message)
}
