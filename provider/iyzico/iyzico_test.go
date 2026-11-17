package iyzico

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/mstgnz/gopay/provider"
)

func TestRegisteredAsIyzico(t *testing.T) {
	factory, err := provider.Get("iyzico")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := factory().(*IyzicoProvider); !ok {
		t.Fatal("the iyzico factory must build an *IyzicoProvider")
	}
}

func TestInitialize(t *testing.T) {
	cases := []struct {
		name    string
		conf    map[string]string
		wantErr bool
		wantURL string
		prod    bool
	}{
		{"sandbox", map[string]string{"apiKey": "a", "secretKey": "s", "environment": "sandbox"}, false, apiSandboxURL, false},
		{"production", map[string]string{"apiKey": "a", "secretKey": "s", "environment": "production"}, false, apiProductionURL, true},
		{"unknown environment is sandbox", map[string]string{"apiKey": "a", "secretKey": "s", "environment": "x"}, false, apiSandboxURL, false},
		{"missing apiKey", map[string]string{"secretKey": "s"}, true, "", false},
		{"missing secretKey", map[string]string{"apiKey": "a"}, true, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewProvider().(*IyzicoProvider)
			err := p.Initialize(tc.conf)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.client.baseURL != tc.wantURL || p.isProduction != tc.prod || p.client.apiKey != "a" || p.client.secretKey != "s" {
				t.Errorf("client = %+v, production = %v", p.client, p.isProduction)
			}
			if p.client.http != httpClient {
				t.Error("Initialize must use the TLS-verifying shared client")
			}
		})
	}
}

func TestCloneSharesTheClientOnly(t *testing.T) {
	p := NewProvider().(*IyzicoProvider)
	if err := p.Initialize(map[string]string{"apiKey": "a", "secretKey": "s"}); err != nil {
		t.Fatal(err)
	}
	c := p.Clone().(*IyzicoProvider)
	if c == p || c.client != p.client {
		t.Fatal("Clone must return a new provider sharing the read-only client")
	}
}

func TestValidateConfig(t *testing.T) {
	p := NewProvider()
	good := map[string]string{"apiKey": "sandbox-aaaaaaaaaaaaaaaaaaaa", "secretKey": "sandbox-bbbbbbbbbbbbbbbbbbbb", "environment": "sandbox"}
	if err := p.ValidateConfig(good); err != nil {
		t.Errorf("valid config refused: %v", err)
	}
	if err := p.ValidateConfig(map[string]string{"apiKey": "short", "environment": "sandbox"}); err == nil {
		t.Error("an incomplete config must be refused")
	}
}

func TestCardAndInstallmentCallsRefuse(t *testing.T) {
	p := &IyzicoProvider{}
	ctx := context.Background()
	if _, err := p.CreatePayment(ctx, provider.PaymentRequest{}); !errors.Is(err, ErrUseCheckout) {
		t.Errorf("CreatePayment: %v", err)
	}
	if _, err := p.Create3DPayment(ctx, provider.PaymentRequest{}); !errors.Is(err, ErrUseCheckout) {
		t.Errorf("Create3DPayment: %v", err)
	}
	if _, err := p.Complete3DPayment(ctx, &provider.CallbackState{}, nil); !errors.Is(err, ErrUseCheckout) {
		t.Errorf("Complete3DPayment: %v", err)
	}
	if _, err := p.GetInstallmentCount(ctx, provider.InstallmentInquireRequest{}); !errors.Is(err, ErrUseCheckout) {
		t.Errorf("GetInstallmentCount: %v", err)
	}
	if _, err := p.GetCommission(ctx, provider.CommissionRequest{}); !errors.Is(err, ErrUseCheckout) {
		t.Errorf("GetCommission: %v", err)
	}
	if ok, _, err := p.ValidateWebhook(ctx, nil, nil); ok || err == nil {
		t.Error("a webhook must never be accepted yet")
	}
}

func detailAnswer(t *testing.T, status string, fraud int) string {
	c := newClient(testAPIKey, testSecretKey, "http://unused")
	return mustJSON(t, map[string]any{"status": "success", "paymentStatus": status, "paymentId": "12345", "currency": "TRY",
		"basketId": "B-1", "conversationId": "c", "paidPrice": 450.5, "price": 450.5, "fraudStatus": fraud,
		"signature": c.responseSignature("12345", "TRY", "B-1", "c", "450.5", "450.5")})
}

func TestCommonGetPaymentStatus(t *testing.T) {
	cases := []struct {
		name    string
		answer  func(*testing.T) string
		status  provider.PaymentStatus
		success bool
	}{
		{"paid", func(t *testing.T) string { return detailAnswer(t, "SUCCESS", 1) }, provider.StatusSuccessful, true},
		{"fraud review", func(t *testing.T) string { return detailAnswer(t, "SUCCESS", 0) }, provider.StatusPending, false},
		{"fraud rejected", func(t *testing.T) string { return detailAnswer(t, "SUCCESS", -1) }, provider.StatusFailed, false},
		{"declined", func(t *testing.T) string { return detailAnswer(t, "FAILURE", 1) }, provider.StatusFailed, false},
		{"3DS open", func(t *testing.T) string { return detailAnswer(t, "INIT_THREEDS", 1) }, provider.StatusPending, false},
		{"error answer is pending", func(*testing.T) string { return `{"status":"failure","errorCode":"5086","errorMessage":"x"}` }, provider.StatusPending, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, fake := newTestProvider(t, http.StatusOK, tc.answer(t))
			got, err := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "12345"})
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tc.status || got.Success != tc.success {
				t.Errorf("status=%s success=%v", got.Status, got.Success)
			}
			if sent := fake.last(); sent.path != pathPaymentDetail || sent.body["paymentId"] != "12345" {
				t.Errorf("sent %s %v", sent.path, sent.body)
			}
		})
	}

	p, _ := newTestProvider(t, http.StatusOK, detailAnswer(t, "SUCCESS", 1))
	if got, _ := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "12345"}); got.Amount != 450.5 {
		t.Errorf("amount = %v", got.Amount)
	}

	p, _ = newTestProvider(t, http.StatusServiceUnavailable, `x`)
	if _, err := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "1"}); !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Errorf("a transport failure must be an error, got %v", err)
	}
	for name, answer := range map[string]string{
		"wrong signature":         `{"status":"success","paymentStatus":"SUCCESS","fraudStatus":1,"paymentId":"1","signature":"00"}`,
		"unsigned success":        `{"status":"success","paymentStatus":"SUCCESS","fraudStatus":1,"paymentId":"1"}`,
		"unsigned failure":        `{"status":"success","paymentStatus":"FAILURE","paymentId":"1"}`,
		"wrong signature pending": `{"status":"success","paymentStatus":"INIT_THREEDS","paymentId":"1","signature":"00"}`,
	} {
		p, _ = newTestProvider(t, http.StatusOK, answer)
		got, err := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "1"})
		if err != nil || got.Status != provider.StatusPending || got.Success || got.ErrorCode != codeSignatureMismatch {
			t.Errorf("%s: a verdict needs iyzico's signature, got %+v %v", name, got, err)
		}
	}
	p, _ = newTestProvider(t, http.StatusOK, `{"status":"success","paymentStatus":"INIT_THREEDS","paymentId":"1"}`)
	if got, err := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{PaymentID: "1"}); err != nil || got.ErrorCode != "" {
		t.Errorf("an unsigned answer without a verdict stays plain pending, got %+v %v", got, err)
	}
	if _, err := p.GetPaymentStatus(context.Background(), provider.GetPaymentStatusRequest{}); err == nil {
		t.Error("an empty paymentId must be refused")
	}
}

func TestCommonCancelPayment(t *testing.T) {
	p, fake := newTestProvider(t, http.StatusOK, `{"status":"success","paymentId":"12345","currency":"TRY"}`)
	got, err := p.CancelPayment(context.Background(), provider.CancelRequest{PaymentID: "12345"})
	if err != nil || !got.Success || got.Status != provider.StatusCancelled {
		t.Fatalf("got %+v %v", got, err)
	}
	if _, ok := fake.last().body["ip"]; ok {
		t.Error("the common cancel has no buyer IP and must not invent one")
	}

	p, _ = newTestProvider(t, http.StatusOK, `{"status":"failure","errorCode":"5086","errorMessage":"x"}`)
	got, err = p.CancelPayment(context.Background(), provider.CancelRequest{PaymentID: "12345"})
	if err != nil || got.Success || got.ErrorCode != "5086" {
		t.Fatalf("refusal: got %+v %v", got, err)
	}

	p, _ = newTestProvider(t, http.StatusBadGateway, `x`)
	if _, err := p.CancelPayment(context.Background(), provider.CancelRequest{PaymentID: "1"}); !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Errorf("want unavailable, got %v", err)
	}
	if _, err := p.CancelPayment(context.Background(), provider.CancelRequest{}); err == nil {
		t.Error("an empty paymentId must be refused")
	}
}

func TestCommonRefundPayment(t *testing.T) {
	p, fake := newTestProvider(t, http.StatusOK, refundAnswerJSON(t, true))
	got, err := p.RefundPayment(context.Background(), provider.RefundRequest{PaymentID: "777", RefundAmount: 100, ConversationID: "c-2"})
	if err != nil || !got.Success || got.RefundID != "777" {
		t.Fatalf("got %+v %v", got, err)
	}
	sent := fake.last().body
	if sent["paymentTransactionId"] != "777" || sent["price"] != "100.00" || sent["currency"] != "TRY" {
		t.Errorf("sent %v", sent)
	}

	p, _ = newTestProvider(t, http.StatusOK, `{"status":"failure","errorCode":"10223","errorMessage":"End of day"}`)
	got, err = p.RefundPayment(context.Background(), provider.RefundRequest{PaymentID: "777", RefundAmount: 1})
	if err != nil || got.Success || got.ErrorCode != "10223" {
		t.Fatalf("refusal: got %+v %v", got, err)
	}

	p, _ = newTestProvider(t, http.StatusOK, refundAnswerJSON(t, false))
	if _, err := p.RefundPayment(context.Background(), provider.RefundRequest{PaymentID: "777", RefundAmount: 100}); !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Errorf("bad signature: want unavailable, got %v", err)
	}
	p, _ = newTestProvider(t, http.StatusBadGateway, `x`)
	if _, err := p.RefundPayment(context.Background(), provider.RefundRequest{PaymentID: "777", RefundAmount: 1}); !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Errorf("transport: want unavailable, got %v", err)
	}
	if _, err := p.RefundPayment(context.Background(), provider.RefundRequest{PaymentID: "777"}); err == nil {
		t.Error("a zero amount must be refused")
	}
	if _, err := p.RefundPayment(context.Background(), provider.RefundRequest{RefundAmount: 1}); err == nil {
		t.Error("an empty paymentId must be refused")
	}
}
